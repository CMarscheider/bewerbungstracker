package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"bewerbungsmanager/internal/httpapi"
	"bewerbungsmanager/internal/service"
	"bewerbungsmanager/internal/testdb"
)

const mailToken = "mail-token-0123456789abcdefghijklmnop"

func newMailTokenServer(t *testing.T, opts ...httpapi.RouterOption) *httptest.Server {
	t.Helper()
	testdb.Reset(t, testPool)
	opts = append([]httpapi.RouterOption{httpapi.WithAgentToken(agentToken), httpapi.WithAgentMailToken(mailToken)}, opts...)
	h, err := httpapi.NewRouter(service.New(testPool, time.Now), slog.New(slog.NewTextHandler(io.Discard, nil)), opts...)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestAgentMailTokenScope(t *testing.T) {
	srv := newMailTokenServer(t)
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	id := createApplication(t, srv, map[string]any{"type": "Vorgemerkt", "occurred_on": today()})

	allowed := []struct {
		method, path string
		body         any
		want         int
	}{
		{http.MethodGet, "/api/agent/cv", nil, http.StatusOK},
		{http.MethodGet, "/api/agent/applications/open", nil, http.StatusOK},
		{http.MethodPost, "/api/agent/applications/" + id + "/events", map[string]any{"type": "Beworben", "occurred_on": today()}, http.StatusCreated},
		{http.MethodPut, "/api/agent/applications/" + id + "/gmail-thread", map[string]any{"gmail_thread_id": "t1"}, http.StatusNoContent},
		{http.MethodPost, "/api/agent/suggestions", map[string]any{"suggested_type": "Absage", "occurred_on": today(), "reason": "x"}, http.StatusCreated},
		{http.MethodGet, "/api/agent/processed-mails/m1", nil, http.StatusNotFound},
		{http.MethodPost, "/api/agent/processed-mails", map[string]any{"gmail_message_id": "m1", "outcome": "ok"}, http.StatusCreated},
	}
	for _, c := range allowed {
		expectStatus(t, callWith(t, srv, c.method, c.path, mailToken, c.body), c.want)
	}

	forbidden := []struct{ method, path string }{
		{http.MethodGet, "/api/agent/applications?documents_state=angefordert"},
		{http.MethodPost, "/api/agent/applications"},
		{http.MethodPut, "/api/agent/applications/" + id + "/documents"},
		{http.MethodGet, "/api/agent/cv-reviews?state=angefordert"},
		{http.MethodPut, "/api/agent/cv-reviews/" + id},
		{http.MethodDelete, "/api/agent/processed-mails/m1"},
		{http.MethodGet, "/api/agent/x"},
	}
	for _, c := range forbidden {
		res := callWith(t, srv, c.method, c.path, mailToken, map[string]any{})
		if res.Status != http.StatusForbidden {
			t.Errorf("%s %s: Status %d; Body: %s", c.method, c.path, res.Status, res.Body)
			continue
		}
		expectProblem(t, res, http.StatusForbidden, "/problems/forbidden")
	}
	if status, _ := rawGet(t, srv.URL+"/api/agent/cv/../cv-reviews?state=angefordert", "Bearer "+mailToken); status != http.StatusForbidden {
		t.Errorf("Pfad mit ..: Status %d", status)
	}

	// Das Haupttoken behält vollen Zugriff.
	expectStatus(t, callWith(t, srv, http.MethodGet, "/api/agent/applications?documents_state=angefordert", agentToken, nil), http.StatusOK)
	expectStatus(t, callWith(t, srv, http.MethodGet, "/api/agent/applications/open", agentToken, nil), http.StatusOK)
}

func TestAgentMailTokenSharesRateLimit(t *testing.T) {
	srv := newMailTokenServer(t, httpapi.WithAgentRateLimit(rate.Every(time.Hour), 2))
	expectStatus(t, callWith(t, srv, http.MethodGet, "/api/agent/applications/open", mailToken, nil), http.StatusOK)
	expectStatus(t, callWith(t, srv, http.MethodGet, "/api/agent/applications/open", agentToken, nil), http.StatusOK)
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/applications/open", mailToken, nil),
		http.StatusTooManyRequests, "/problems/too-many-requests")
}

func TestAgentMailboxErrors(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	other := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(agentJobBody))
	expectStatus(t, other, http.StatusCreated)
	otherID := other.object(t)["id"].(string)
	unknown := "00000000-0000-4000-8000-000000000001"

	// Thread-Konflikte: schon vergeben bzw. andere ID für dieselbe Bewerbung.
	expectStatus(t, callWith(t, srv, http.MethodPut, "/api/agent/applications/"+id+"/gmail-thread", agentToken,
		map[string]any{"gmail_thread_id": "t1"}), http.StatusNoContent)
	expectProblem(t, callWith(t, srv, http.MethodPut, "/api/agent/applications/"+otherID+"/gmail-thread", agentToken,
		map[string]any{"gmail_thread_id": "t1"}), http.StatusConflict, "/problems/conflict")
	expectProblem(t, callWith(t, srv, http.MethodPut, "/api/agent/applications/"+id+"/gmail-thread", agentToken,
		map[string]any{"gmail_thread_id": "t2"}), http.StatusConflict, "/problems/conflict")

	// Unbekannte Bewerbung.
	expectProblem(t, callWith(t, srv, http.MethodPost, "/api/agent/applications/"+unknown+"/events", agentToken,
		map[string]any{"type": "Absage", "occurred_on": today()}), http.StatusNotFound, "/problems/not-found")
	expectProblem(t, callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken,
		map[string]any{"application_id": unknown, "suggested_type": "Absage", "occurred_on": today(), "reason": "x"}),
		http.StatusNotFound, "/problems/not-found")
	expectProblem(t, callWith(t, srv, http.MethodPost, "/api/agent/processed-mails", agentToken,
		map[string]any{"gmail_message_id": "m1", "application_id": unknown, "outcome": "x"}), http.StatusNotFound, "/problems/not-found")

	validation := []struct {
		name, method, path, field string
		body                      map[string]any
	}{
		{"due_on bei Absage (Ereignis)", http.MethodPost, "/api/agent/applications/" + id + "/events", "due_on",
			map[string]any{"type": "Absage", "occurred_on": today(), "due_on": today()}},
		{"due_on bei Absage (Vorschlag)", http.MethodPost, "/api/agent/suggestions", "due_on",
			map[string]any{"suggested_type": "Absage", "occurred_on": today(), "due_on": today(), "reason": "x"}},
		{"Whitelist", http.MethodPost, "/api/agent/applications/" + id + "/events", "type",
			map[string]any{"type": "Zurueckgezogen", "occurred_on": today()}},
		{"unsichtbares Zeichen im Grund", http.MethodPost, "/api/agent/suggestions", "reason",
			map[string]any{"suggested_type": "Absage", "occurred_on": today(), "reason": "a\u202eb"}},
		{"unsichtbares Zeichen in Notiz", http.MethodPost, "/api/agent/applications/" + id + "/events", "note",
			map[string]any{"type": "Absage", "occurred_on": today(), "note": "a\u200bb"}},
		{"Steuerzeichen im Ergebnis", http.MethodPost, "/api/agent/processed-mails", "outcome",
			map[string]any{"gmail_message_id": "m9", "outcome": "a\u2028b"}},
	}
	for _, c := range validation {
		res := callWith(t, srv, c.method, c.path, agentToken, c.body)
		if res.Status != http.StatusBadRequest {
			t.Errorf("%s: Status %d; Body: %s", c.name, res.Status, res.Body)
			continue
		}
		if p := res.object(t); p["field"] != c.field {
			t.Errorf("%s: field = %v, erwartet %s", c.name, p["field"], c.field)
		}
	}
	for name, body := range map[string]map[string]any{
		"mail_from mit CR/LF": {"suggested_type": "Absage", "occurred_on": today(), "reason": "x", "mail_from": "a@b.de\r\nBcc: x"},
		"mail_from mit LF":    {"suggested_type": "Absage", "occurred_on": today(), "reason": "x", "mail_from": "a@b.de\nx"},
	} {
		if res := callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken, body); res.Status != http.StatusBadRequest {
			t.Errorf("%s: Status %d", name, res.Status)
		}
	}
	if res := callWith(t, srv, http.MethodPost, "/api/agent/processed-mails", agentToken,
		map[string]any{"gmail_message_id": "m2", "outcome": strings.Repeat("x", 201)}); res.Status != http.StatusBadRequest {
		t.Errorf("outcome zu lang: Status %d", res.Status)
	}
}

func TestClearGmailThreadEndpoint(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	expectStatus(t, callWith(t, srv, http.MethodPut, "/api/agent/applications/"+id+"/gmail-thread", agentToken,
		map[string]any{"gmail_thread_id": "t1"}), http.StatusNoContent)
	for range 2 { // idempotent
		expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/applications/"+id+"/gmail-thread", nil), http.StatusNoContent)
	}
	if _, ok := call(t, srv, http.MethodGet, "/api/v1/applications/"+id, nil).object(t)["gmail_thread_id"]; ok {
		t.Error("gmail_thread_id noch gesetzt")
	}
	expectStatus(t, callWith(t, srv, http.MethodPut, "/api/agent/applications/"+id+"/gmail-thread", agentToken,
		map[string]any{"gmail_thread_id": "t2"}), http.StatusNoContent)
	expectProblem(t, call(t, srv, http.MethodDelete, "/api/v1/applications/00000000-0000-4000-8000-000000000001/gmail-thread", nil),
		http.StatusNotFound, "/problems/not-found")
}

func TestProcessedMailsUIEndpoints(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	for _, m := range []map[string]any{
		{"gmail_message_id": "m1", "outcome": "Werbung"},
		{"gmail_message_id": "m2", "application_id": id, "outcome": "Absage gebucht"},
		{"gmail_message_id": "m3", "outcome": "Newsletter"},
	} {
		expectStatus(t, callWith(t, srv, http.MethodPost, "/api/agent/processed-mails", agentToken, m), http.StatusCreated)
	}
	res := call(t, srv, http.MethodGet, "/api/v1/processed-mails?limit=2", nil)
	expectStatus(t, res, http.StatusOK)
	list := res.array(t)
	if len(list) != 2 {
		t.Fatalf("Liste: %s", res.Body)
	}
	first, second := list[0].(map[string]any), list[1].(map[string]any)
	if first["gmail_message_id"] != "m3" || second["gmail_message_id"] != "m2" ||
		second["company_name"] != "Acme" || second["position_title"] != "Go-Entwickler" {
		t.Fatalf("Liste: %s", res.Body)
	}
	if all := call(t, srv, http.MethodGet, "/api/v1/processed-mails", nil); len(all.array(t)) != 3 {
		t.Errorf("ohne limit: %s", all.Body)
	}
	for _, q := range []string{"limit=0", "limit=201"} {
		if r := call(t, srv, http.MethodGet, "/api/v1/processed-mails?"+q, nil); r.Status != http.StatusBadRequest {
			t.Errorf("%s: Status %d", q, r.Status)
		}
	}
	for range 2 { // idempotent
		expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/processed-mails/m2", nil), http.StatusNoContent)
	}
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/processed-mails/m2", agentToken, nil), http.StatusNotFound, "/problems/not-found")
	expectStatus(t, callWith(t, srv, http.MethodPost, "/api/agent/processed-mails", agentToken,
		map[string]any{"gmail_message_id": "m2", "outcome": "neu"}), http.StatusCreated)
}

func TestAgentCreateSuggestionDeduplicates(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	body := map[string]any{"suggested_type": "Absage", "occurred_on": today(), "reason": "Absage?", "gmail_message_id": "msg-1"}
	first := callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken, body)
	expectStatus(t, first, http.StatusCreated)
	created := first.object(t)
	if created["gmail_message_id"] != "msg-1" {
		t.Fatalf("Vorschlag: %v", created)
	}
	body["reason"] = "anders"
	again := callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken, body)
	expectStatus(t, again, http.StatusOK)
	if o := again.object(t); o["id"] != created["id"] || o["reason"] != "Absage?" {
		t.Fatalf("Dublette: %v", o)
	}
	if items := call(t, srv, http.MethodGet, "/api/v1/suggestions", nil).array(t); len(items) != 1 {
		t.Fatalf("Vorschläge: %v", items)
	}
	body["gmail_message_id"] = "a/b"
	if res := callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken, body); res.Status != http.StatusBadRequest {
		t.Errorf("ungültige Message-ID: Status %d", res.Status)
	}
}
