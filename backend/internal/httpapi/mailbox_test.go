package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestAgentMailboxFlow(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := createApplication(t, srv, map[string]any{"type": "Vorgemerkt", "occurred_on": today()})

	// Pfadkonflikt: "open" darf nicht als {id} gelesen werden.
	open := callWith(t, srv, http.MethodGet, "/api/agent/applications/open", agentToken, nil)
	expectStatus(t, open, http.StatusOK)
	list := open.array(t)
	if len(list) != 1 {
		t.Fatalf("offene Liste: %s", open.Body)
	}
	entry := list[0].(map[string]any)
	if entry["id"] != id || entry["status"] != "Vorgemerkt" || entry["company_name"] != "Acme" {
		t.Fatalf("Eintrag: %v", entry)
	}
	allowed, _ := entry["allowed_events"].([]any)
	if !slices.Contains(allowed, any("Beworben")) {
		t.Fatalf("allowed_events = %v", entry["allowed_events"])
	}

	ev := callWith(t, srv, http.MethodPost, "/api/agent/applications/"+id+"/events", agentToken,
		map[string]any{"type": "Beworben", "occurred_on": today(), "note": "Bewerbung gesendet"})
	expectStatus(t, ev, http.StatusCreated)
	if note, _ := ev.object(t)["note"].(string); !strings.HasPrefix(note, "Agent: ") {
		t.Fatalf("Notiz = %q", note)
	}
	bad := callWith(t, srv, http.MethodPost, "/api/agent/applications/"+id+"/events", agentToken,
		map[string]any{"type": "Beworben", "occurred_on": today()}) // schon beworben
	expectProblem(t, bad, http.StatusUnprocessableEntity, "/problems/invalid-transition")

	th := callWith(t, srv, http.MethodPut, "/api/agent/applications/"+id+"/gmail-thread", agentToken,
		map[string]any{"gmail_thread_id": "18c0ffee_A-1"})
	expectStatus(t, th, http.StatusNoContent)
	app := call(t, srv, http.MethodGet, "/api/v1/applications/"+id, nil)
	expectStatus(t, app, http.StatusOK)
	if app.object(t)["gmail_thread_id"] != "18c0ffee_A-1" {
		t.Fatalf("gmail_thread_id fehlt: %s", app.Body)
	}

	sg := callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken, map[string]any{
		"application_id": id, "suggested_type": "Absage", "occurred_on": today(), "reason": "Klingt nach Absage",
		"mail_subject": "Ihre Bewerbung", "mail_from": "hr@acme.example", "mail_url": "https://mail.google.com/mail/u/0/#inbox/abc",
	})
	expectStatus(t, sg, http.StatusCreated)
	created := sg.object(t)
	sgID := created["id"].(string)
	if created["state"] != "offen" || created["company_name"] != "Acme" || created["position_title"] != "Go-Entwickler" {
		t.Fatalf("Vorschlag: %v", created)
	}

	sgs := call(t, srv, http.MethodGet, "/api/v1/suggestions", nil)
	expectStatus(t, sgs, http.StatusOK)
	if items := sgs.array(t); len(items) != 1 || items[0].(map[string]any)["id"] != sgID {
		t.Fatalf("Vorschläge: %s", sgs.Body)
	}
	acc := call(t, srv, http.MethodPost, "/api/v1/suggestions/"+sgID+"/accept", map[string]any{})
	expectStatus(t, acc, http.StatusOK)
	if acc.object(t)["status"] != "Absage" {
		t.Fatalf("nach Übernahme: %s", acc.Body)
	}
	again := call(t, srv, http.MethodPost, "/api/v1/suggestions/"+sgID+"/accept", map[string]any{})
	expectProblem(t, again, http.StatusConflict, "/problems/conflict")
	if items := call(t, srv, http.MethodGet, "/api/v1/suggestions", nil).array(t); len(items) != 0 {
		t.Fatalf("Liste nicht leer: %v", items)
	}

	// Nicht zugeordneter Vorschlag: verwerfen.
	sg2 := callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken, map[string]any{
		"suggested_type": "Interview", "occurred_on": today(), "reason": "Einladung?",
	})
	expectStatus(t, sg2, http.StatusCreated)
	sg2ID := sg2.object(t)["id"].(string)
	expectStatus(t, call(t, srv, http.MethodPost, "/api/v1/suggestions/"+sg2ID+"/dismiss", nil), http.StatusNoContent)
	if items := call(t, srv, http.MethodGet, "/api/v1/suggestions", nil).array(t); len(items) != 0 {
		t.Fatalf("Liste nicht leer: %v", items)
	}

	// Verarbeitete Mails: idempotent.
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/processed-mails/msg-1", agentToken, nil),
		http.StatusNotFound, "/problems/not-found")
	mail := map[string]any{"gmail_message_id": "msg-1", "application_id": id, "outcome": "Absage vorgeschlagen"}
	expectStatus(t, callWith(t, srv, http.MethodPost, "/api/agent/processed-mails", agentToken, mail), http.StatusCreated)
	mail["outcome"] = "anders"
	dup := callWith(t, srv, http.MethodPost, "/api/agent/processed-mails", agentToken, mail)
	expectStatus(t, dup, http.StatusOK)
	if dup.object(t)["outcome"] != "Absage vorgeschlagen" {
		t.Fatalf("vorhandener Eintrag geändert: %s", dup.Body)
	}
	got := callWith(t, srv, http.MethodGet, "/api/agent/processed-mails/msg-1", agentToken, nil)
	expectStatus(t, got, http.StatusOK)
	if o := got.object(t); o["gmail_message_id"] != "msg-1" || o["application_id"] != id || o["processed_at"] == nil {
		t.Fatalf("verarbeitete Mail: %v", o)
	}
}

func TestAcceptUnassignedSuggestionNeedsApplication(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	sg := callWith(t, srv, http.MethodPost, "/api/agent/suggestions", agentToken, map[string]any{
		"suggested_type": "Absage", "occurred_on": today(), "reason": "Absage ohne Bezug",
	})
	expectStatus(t, sg, http.StatusCreated)
	sgID := sg.object(t)["id"].(string)

	expectProblem(t, call(t, srv, http.MethodPost, "/api/v1/suggestions/"+sgID+"/accept", map[string]any{}),
		http.StatusBadRequest, "/problems/validation-error")
	acc := call(t, srv, http.MethodPost, "/api/v1/suggestions/"+sgID+"/accept", map[string]any{"application_id": id})
	expectStatus(t, acc, http.StatusOK)
	if o := acc.object(t); o["id"] != id || o["status"] != "Absage" {
		t.Fatalf("nach Übernahme: %v", o)
	}
}

func TestAgentMailboxValidation(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := createApplication(t, srv, map[string]any{"type": "Beworben", "occurred_on": today()})
	cases := []struct {
		name, method, path string
		body               map[string]any
	}{
		{"mail_url javascript", http.MethodPost, "/api/agent/suggestions", map[string]any{
			"suggested_type": "Absage", "occurred_on": today(), "reason": "x", "mail_url": "javascript:alert(1)"}},
		{"mail_subject mit Umbruch", http.MethodPost, "/api/agent/suggestions", map[string]any{
			"suggested_type": "Absage", "occurred_on": today(), "reason": "x", "mail_subject": "a\r\nBcc: x"}},
		{"reason leer", http.MethodPost, "/api/agent/suggestions", map[string]any{
			"suggested_type": "Absage", "occurred_on": today(), "reason": "   "}},
		{"thread mit Slash", http.MethodPut, "/api/agent/applications/" + id + "/gmail-thread", map[string]any{
			"gmail_thread_id": "abc/def"}},
		{"message-id leer", http.MethodPost, "/api/agent/processed-mails", map[string]any{
			"gmail_message_id": "", "outcome": "x"}},
		{"notiz zu lang", http.MethodPost, "/api/agent/applications/" + id + "/events", map[string]any{
			"type": "Absage", "occurred_on": today(), "note": strings.Repeat("x", 1001)}},
	}
	for _, c := range cases {
		if res := callWith(t, srv, c.method, c.path, agentToken, c.body); res.Status != http.StatusBadRequest {
			t.Errorf("%s: Status %d; Body: %s", c.name, res.Status, res.Body)
		}
	}
	// Message-ID mit Slash ist im Pfad kein gültiger Wert.
	if res := callWith(t, srv, http.MethodGet, "/api/agent/processed-mails/a%2Fb", agentToken, nil); res.Status != http.StatusBadRequest {
		t.Errorf("Message-ID mit Slash: Status %d; Body: %s", res.Status, res.Body)
	}
}

func TestAgentMailboxNeedsToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := "00000000-0000-0000-0000-000000000001"
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/agent/applications/open"},
		{http.MethodPost, "/api/agent/applications/" + id + "/events"},
		{http.MethodPut, "/api/agent/applications/" + id + "/gmail-thread"},
		{http.MethodPost, "/api/agent/suggestions"},
		{http.MethodGet, "/api/agent/processed-mails/msg-1"},
		{http.MethodPost, "/api/agent/processed-mails"},
	} {
		if res := callWith(t, srv, c.method, c.path, "", map[string]any{}); res.Status != http.StatusUnauthorized {
			t.Errorf("%s %s: Status %d", c.method, c.path, res.Status)
		}
	}
}
