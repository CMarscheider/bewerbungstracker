package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

const agentJobBody = `{"company_name":"Acme GmbH","position_title":"Junior Frontend-Entwickler",
"job_url":"https://jobs.example.com/1","location":"Remote","source":"Arbeitnow",
"contact_email":"jobs@acme.example","posting_text":"Wir suchen …","fit_score":82,"fit_reason":"Junior, Angular passt."}`

func TestAgentCreateApplication(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	res := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(agentJobBody))
	expectStatus(t, res, http.StatusCreated)
	var app struct {
		ID             string `json:"id"`
		Status         string `json:"status"`
		FitScore       int    `json:"fit_score"`
		FitReason      string `json:"fit_reason"`
		PostingText    string `json:"posting_text"`
		CreatedByAgent bool   `json:"created_by_agent"`
		ContactEmail   string `json:"contact_email"`
	}
	if err := json.Unmarshal(res.Body, &app); err != nil {
		t.Fatal(err)
	}
	if app.Status != "Vorgemerkt" || app.FitScore != 82 || !app.CreatedByAgent || app.ContactEmail != "jobs@acme.example" ||
		app.FitReason != "Junior, Angular passt." || app.PostingText != "Wir suchen …" {
		t.Fatalf("unerwartet: %+v", app)
	}

	dup := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(agentJobBody))
	p := expectProblem(t, dup, http.StatusConflict, "/problems/duplicate")
	if p["existing_id"] != app.ID {
		t.Fatalf("Problem: %v", p)
	}
}

func TestAgentCreateApplicationValidatesAndNeedsToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	for name, body := range map[string]string{
		"score":                   `{"company_name":"A","position_title":"B","fit_score":101,"fit_reason":"x"}`,
		"url":                     `{"company_name":"A","position_title":"B","job_url":"javascript:alert(1)","fit_score":5,"fit_reason":"x"}`,
		"email":                   `{"company_name":"A","position_title":"B","contact_email":"kein-mail","fit_score":5,"fit_reason":"x"}`,
		"leer":                    `{"company_name":" ","position_title":"B","fit_score":5,"fit_reason":"x"}`,
		"grund":                   `{"company_name":"A","position_title":"B","fit_score":5}`,
		"url mit Leerzeichen":     `{"company_name":"A","position_title":"B","job_url":"https://jobs.example.com/a b","fit_score":5,"fit_reason":"x"}`,
		"website mit Leerzeichen": `{"company_name":"A","company_website":"https://a.example x","position_title":"B","fit_score":5,"fit_reason":"x"}`,
		"email mit Query":         `{"company_name":"A","position_title":"B","contact_email":"a@b.de?subject=x","fit_score":5,"fit_reason":"x"}`,
	} {
		if res := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(body)); res.Status != http.StatusBadRequest {
			t.Errorf("%s: status %d; Body: %s", name, res.Status, res.Body)
		}
	}
	for _, bad := range invalidContactEmails[:5] {
		body := map[string]any{"company_name": "A", "position_title": "B", "contact_email": bad, "fit_score": 5, "fit_reason": "x"}
		if res := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, body); res.Status != http.StatusBadRequest {
			t.Errorf("E-Mail %q: status %d", bad, res.Status)
		}
	}
	res := callWith(t, srv, http.MethodPost, "/api/agent/applications", "", json.RawMessage(agentJobBody))
	expectStatus(t, res, http.StatusUnauthorized)
}

func TestListApplicationsFromAgentAndSort(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	low := `{"company_name":"Beta AG","position_title":"Senior Frontend","job_url":"https://jobs.example.com/2","fit_score":40,"fit_reason":"Senior"}`
	for _, b := range []string{low, agentJobBody} {
		expectStatus(t, callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(b)), http.StatusCreated)
	}
	// Eine manuell angelegte Bewerbung darf im Agenten-Filter nicht auftauchen.
	manual := call(t, srv, http.MethodPost, "/api/v1/applications", map[string]any{
		"company_id": createCompany(t, srv, "Gamma KG"), "position_title": "Manuell",
		"contact_email": "hr@gamma.example",
		"first_event":   map[string]any{"type": "Vorgemerkt", "occurred_on": today()},
	})
	expectStatus(t, manual, http.StatusCreated)
	if m := manual.object(t); m["contact_email"] != "hr@gamma.example" || m["created_by_agent"] != false {
		t.Fatalf("manuell: %v", m)
	}

	res := call(t, srv, http.MethodGet, "/api/v1/applications?from_agent=true&sort=score", nil)
	expectStatus(t, res, http.StatusOK)
	var list []struct {
		FitScore       int  `json:"fit_score"`
		CreatedByAgent bool `json:"created_by_agent"`
	}
	if err := json.Unmarshal(res.Body, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].FitScore != 82 || list[1].FitScore != 40 || !list[0].CreatedByAgent {
		t.Fatalf("Liste: %+v", list)
	}

	if n := len(call(t, srv, http.MethodGet, "/api/v1/applications?from_agent=false", nil).array(t)); n != 1 {
		t.Fatalf("manuelle: %d", n)
	}
	expectStatus(t, call(t, srv, http.MethodGet, "/api/v1/applications?sort=foo", nil), http.StatusBadRequest)
}

// Ungültige Bewerbungs-Adressen, vor allem solche, die im mailto-Link Parameter einschleusen könnten.
// Die ersten fünf prüft auch der Agenten-Test (mehr sprengt dort die Drosselung, Burst 20).
var invalidContactEmails = []string{
	"a@b.de?cc=x", "a@b.de%3Fcc", "a b@c.de", "a@b.de#x", "a@b",
	"a@b.de?subject=x", "a@b.de&cc=c@d.de", `"a"@b.de`, "<a@b.de>", "a@b.de=x", "a,b@c.de", "a;b@c.de", "a/b@c.de", "a:b@c.de",
}

func TestContactEmailRejectsMailtoParameters(t *testing.T) {
	srv := newTestServer(t)
	company := createCompany(t, srv, "Acme")
	res := call(t, srv, http.MethodPost, "/api/v1/applications", map[string]any{
		"company_id": company, "position_title": "Frontend", "contact_email": "a@b.de?subject=x",
		"first_event": map[string]any{"type": "Vorgemerkt", "occurred_on": today()},
	})
	expectStatus(t, res, http.StatusBadRequest)

	ok := call(t, srv, http.MethodPost, "/api/v1/applications", map[string]any{
		"company_id": company, "position_title": "Frontend", "contact_email": "jobs@acme.example",
		"first_event": map[string]any{"type": "Vorgemerkt", "occurred_on": today()},
	})
	expectStatus(t, ok, http.StatusCreated)
	id := ok.object(t)["id"].(string)
	for _, bad := range invalidContactEmails {
		res := call(t, srv, http.MethodPatch, "/api/v1/applications/"+id, map[string]any{"contact_email": bad})
		if res.Status != http.StatusBadRequest {
			t.Errorf("PATCH %q: status %d", bad, res.Status)
		}
	}
	expectStatus(t, call(t, srv, http.MethodPatch, "/api/v1/applications/"+id, map[string]any{"contact_email": ""}), http.StatusOK)
}
