package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDeadlinesAppointmentsAndStats(t *testing.T) {
	srv := newTestServer(t)
	inTwoDays := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	createApplication(t, srv, map[string]any{"type": "Vorgemerkt", "occurred_on": today(), "due_on": inTwoDays})

	deadlines := call(t, srv, http.MethodGet, "/api/v1/deadlines", nil)
	expectStatus(t, deadlines, http.StatusOK)
	list := deadlines.array(t)
	if len(list) != 1 {
		t.Fatalf("Fristen = %s", deadlines.Body)
	}
	if d := list[0].(map[string]any); d["due_on"] != inTwoDays || d["overdue"] != false || d["event_type"] != "Vorgemerkt" {
		t.Errorf("Frist = %v", d)
	}

	appointments := call(t, srv, http.MethodGet, "/api/v1/appointments?within_days=30", nil)
	expectStatus(t, appointments, http.StatusOK)
	if n := len(appointments.array(t)); n != 0 {
		t.Errorf("Termine = %d, erwartet 0", n)
	}

	funnel := call(t, srv, http.MethodGet, "/api/v1/stats/funnel", nil)
	expectStatus(t, funnel, http.StatusOK)
	if n := len(funnel.array(t)); n != 7 {
		t.Errorf("Funnel-Stufen = %d, erwartet 7", n)
	}

	summary := call(t, srv, http.MethodGet, "/api/v1/stats/summary", nil)
	expectStatus(t, summary, http.StatusOK)
	if body := summary.object(t); body["applied"] != float64(0) {
		t.Errorf("Summary = %v", body)
	}
}

func TestWithinDaysOutOfRangeIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/deadlines?within_days=-1", nil), http.StatusBadRequest, "")
}

func TestDocsEndpoints(t *testing.T) {
	srv := newTestServer(t)
	spec := call(t, srv, http.MethodGet, "/api/openapi.json", nil)
	expectStatus(t, spec, http.StatusOK)
	if !strings.Contains(string(spec.Body), `"openapi"`) {
		t.Errorf("Spec enthält kein openapi-Feld")
	}
	docs := call(t, srv, http.MethodGet, "/api/docs", nil)
	expectStatus(t, docs, http.StatusOK)
	if !strings.HasPrefix(docs.ContentType, "text/html") {
		t.Errorf("Content-Type = %q", docs.ContentType)
	}
}

func TestWithinDaysAboveMaximumIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/deadlines?within_days=366", nil), http.StatusBadRequest, "")
}
