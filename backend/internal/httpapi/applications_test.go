package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func createApplication(t *testing.T, srv *httptest.Server, firstEvent map[string]any) string {
	t.Helper()
	companyID := createCompany(t, srv, "Acme")
	r := call(t, srv, http.MethodPost, "/api/v1/applications", map[string]any{
		"company_id": companyID, "position_title": "Go-Entwickler", "source": "LinkedIn",
		"first_event": firstEvent,
	})
	expectStatus(t, r, http.StatusCreated)
	return r.object(t)["id"].(string)
}

func TestApplicationLifecycle(t *testing.T) {
	srv := newTestServer(t)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	base := "/api/v1/applications/" + id

	got := call(t, srv, http.MethodGet, base, nil)
	expectStatus(t, got, http.StatusOK)
	if body := got.object(t); body["status"] != "Beworben" || body["phase"] != "Aktiv" || body["company_name"] != "Acme" {
		t.Errorf("GET = %v", body)
	}

	allowed := call(t, srv, http.MethodGet, base+"/allowed-events", nil)
	expectStatus(t, allowed, http.StatusOK)
	if !slices.Contains(allowed.array(t), any("Interview")) {
		t.Errorf("Interview sollte erlaubt sein: %s", allowed.Body)
	}

	interview := call(t, srv, http.MethodPost, base+"/events", map[string]any{"type": "Interview", "occurred_on": today()})
	expectStatus(t, interview, http.StatusCreated)
	if round := interview.object(t)["interview_round"]; round != float64(1) {
		t.Errorf("interview_round = %v", round)
	}

	expectStatus(t, call(t, srv, http.MethodPost, base+"/events", map[string]any{"type": "Absage", "occurred_on": today()}), http.StatusCreated)

	invalid := call(t, srv, http.MethodPost, base+"/events", map[string]any{"type": "Interview", "occurred_on": today()})
	p := expectProblem(t, invalid, http.StatusUnprocessableEntity, "/problems/invalid-transition")
	if p["from"] != "Absage" || p["attempted"] != "Interview" {
		t.Errorf("Problem = %v", p)
	}

	undone := call(t, srv, http.MethodDelete, base+"/events/latest", nil)
	expectStatus(t, undone, http.StatusOK)
	if status := undone.object(t)["status"]; status != "Interview" {
		t.Errorf("Status nach Rückgängig = %v", status)
	}

	active := call(t, srv, http.MethodGet, "/api/v1/applications?phase=Aktiv", nil)
	expectStatus(t, active, http.StatusOK)
	if n := len(active.array(t)); n != 1 {
		t.Errorf("aktive Bewerbungen = %d, erwartet 1", n)
	}
	closed := call(t, srv, http.MethodGet, "/api/v1/applications?phase=Abgeschlossen", nil)
	if n := len(closed.array(t)); n != 0 {
		t.Errorf("abgeschlossene Bewerbungen = %d, erwartet 0", n)
	}

	patched := call(t, srv, http.MethodPatch, base, map[string]any{"position_title": "Senior Go-Entwickler"})
	expectStatus(t, patched, http.StatusOK)
	if patched.object(t)["position_title"] != "Senior Go-Entwickler" {
		t.Errorf("PATCH = %s", patched.Body)
	}

	expectStatus(t, call(t, srv, http.MethodDelete, base, nil), http.StatusNoContent)
	expectProblem(t, call(t, srv, http.MethodGet, base, nil), http.StatusNotFound, "/problems/not-found")
}

func TestUnknownEventTypeIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	r := call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/events", map[string]any{"type": "Quatsch", "occurred_on": today()})
	expectProblem(t, r, http.StatusBadRequest, "")
}

func TestDueOnOnWrongTypeIsValidationError(t *testing.T) {
	srv := newTestServer(t)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	r := call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/events", map[string]any{
		"type": "Absage", "occurred_on": today(), "due_on": today(),
	})
	p := expectProblem(t, r, http.StatusBadRequest, "/problems/validation-error")
	if p["field"] != "due_on" {
		t.Errorf("field = %v", p["field"])
	}
}

func TestUndoFirstEventIsRuleViolation(t *testing.T) {
	srv := newTestServer(t)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	r := call(t, srv, http.MethodDelete, "/api/v1/applications/"+id+"/events/latest", nil)
	expectProblem(t, r, http.StatusUnprocessableEntity, "/problems/cannot-undo-first-event")
}

func TestListAllowedEventsAfterAbsageIsEmptyArray(t *testing.T) {
	srv := newTestServer(t)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	base := "/api/v1/applications/" + id
	expectStatus(t, call(t, srv, http.MethodPost, base+"/events", map[string]any{"type": "Absage", "occurred_on": today()}), http.StatusCreated)

	r := call(t, srv, http.MethodGet, base+"/allowed-events", nil)
	expectStatus(t, r, http.StatusOK)
	if got := strings.TrimSpace(string(r.Body)); got != "[]" {
		t.Errorf("Body = %q, erwartet []", got)
	}
}

func TestMalformedCompanyIDInBodyIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	r := call(t, srv, http.MethodPost, "/api/v1/applications", map[string]any{
		"company_id": "abc", "position_title": "Go-Entwickler",
		"first_event": map[string]any{"type": "Beworben", "occurred_on": today()},
	})
	expectProblem(t, r, http.StatusBadRequest, "")
}
