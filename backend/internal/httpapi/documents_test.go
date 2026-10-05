package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"bewerbungsmanager/internal/httpapi"
	"bewerbungsmanager/internal/service"
	"bewerbungsmanager/internal/testdb"
)

// fakeDrafter merkt sich die abgelegten Entwürfe, statt sie per IMAP abzulegen.
type fakeDrafter struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (d *fakeDrafter) Save(_ context.Context, msg []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.msgs = append(d.msgs, msg)
	return nil
}

func (d *fakeDrafter) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.msgs)
}

// newDocumentsTestServer startet einen Server mit Agent-Token, Fake-PDF-Dienst und Fake-Drafter.
func newDocumentsTestServer(t *testing.T) (*httptest.Server, *fakeDrafter) {
	t.Helper()
	testdb.Reset(t, testPool)
	drafter := &fakeDrafter{}
	svc := service.New(testPool, time.Now,
		service.WithPDFConverter(fakeConverter{}), service.WithDrafter(drafter, "erika@example.com"))
	h, err := httpapi.NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), httpapi.WithAgentToken(agentToken))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, drafter
}

func documentsBody(version int) map[string]any {
	return map[string]any{
		"version": version, "language": "de",
		"cover_letter": "Sehr geehrte Damen und Herren,\n\nich bewerbe mich …",
		"profile_line": "Junior Frontend-Entwickler mit Angular-Erfahrung.",
		"highlights":   []string{"TypeScript", "Angular"},
		"mail_subject": "Bewerbung als Junior Frontend-Entwickler",
		"mail_body":    "Guten Tag,\n\nanbei meine Unterlagen.",
	}
}

func TestDocumentsFlow(t *testing.T) {
	srv, drafter := newDocumentsTestServer(t)
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	created := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(agentJobBody))
	expectStatus(t, created, http.StatusCreated)
	app := created.object(t)
	id := app["id"].(string)
	if app["documents_state"] != "keine" {
		t.Fatalf("documents_state = %v", app["documents_state"])
	}

	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/applications/"+id+"/documents", nil),
		http.StatusNotFound, "/problems/not-found")
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/applications/"+id+"/documents/pdf", nil),
		http.StatusNotFound, "/problems/not-found")

	req := call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/documents/request", nil)
	expectStatus(t, req, http.StatusOK)
	if s := req.object(t)["documents_state"]; s != "angefordert" {
		t.Fatalf("nach Anforderung: %v", s)
	}

	list := callWith(t, srv, http.MethodGet, "/api/agent/applications?documents_state=angefordert", agentToken, nil)
	expectStatus(t, list, http.StatusOK)
	var items []struct {
		ID               string `json:"id"`
		CompanyName      string `json:"company_name"`
		PostingText      string `json:"posting_text"`
		ContactEmail     string `json:"contact_email"`
		DocumentsState   string `json:"documents_state"`
		DocumentsVersion *int   `json:"documents_version"`
	}
	if err := json.Unmarshal(list.Body, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id || items[0].PostingText != "Wir suchen …" || items[0].CompanyName != "Acme GmbH" ||
		items[0].DocumentsState != "angefordert" || items[0].DocumentsVersion == nil || *items[0].DocumentsVersion != 0 {
		t.Fatalf("Agent-Liste: %s", list.Body)
	}
	expectStatus(t, callWith(t, srv, http.MethodGet, "/api/agent/applications?documents_state=unbekannt", agentToken, nil),
		http.StatusBadRequest)

	put := callWith(t, srv, http.MethodPut, "/api/agent/applications/"+id+"/documents", agentToken, documentsBody(1))
	expectStatus(t, put, http.StatusOK)
	docs := put.object(t)
	if docs["version"] != float64(1) || docs["language"] != "de" || !strings.HasPrefix(docs["file_name"].(string), "Bewerbung_") {
		t.Fatalf("Unterlagen: %v", docs)
	}
	if drafter.count() != 1 {
		t.Fatalf("Entwürfe: %d", drafter.count())
	}

	got := call(t, srv, http.MethodGet, "/api/v1/applications/"+id, nil).object(t)
	if got["documents_state"] != "entwurf_angelegt" || got["gmail_draft_at"] == nil {
		t.Fatalf("Bewerbung: %v", got)
	}
	expectStatus(t, call(t, srv, http.MethodGet, "/api/v1/applications/"+id+"/documents", nil), http.StatusOK)

	pdf := call(t, srv, http.MethodGet, "/api/v1/applications/"+id+"/documents/pdf", nil)
	expectStatus(t, pdf, http.StatusOK)
	if pdf.ContentType != "application/pdf" || pdf.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Content-Type %q, Cache-Control %q", pdf.ContentType, pdf.Header.Get("Cache-Control"))
	}
	if cd := pdf.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, `inline; filename="Bewerbung_`) {
		t.Errorf("Content-Disposition = %q", cd)
	}

	// Gleiche Lieferung erneut: No-op.
	expectStatus(t, callWith(t, srv, http.MethodPut, "/api/agent/applications/"+id+"/documents", agentToken, documentsBody(1)),
		http.StatusOK)
	if drafter.count() != 1 {
		t.Fatalf("Entwürfe nach Wiederholung: %d", drafter.count())
	}

	// Oberfläche bearbeitet: neue Version, kein neuer Entwurf.
	edit := documentsBody(0)
	delete(edit, "version")
	edit["cover_letter"] = "Neu formuliert."
	upd := call(t, srv, http.MethodPut, "/api/v1/applications/"+id+"/documents", edit)
	expectStatus(t, upd, http.StatusOK)
	if v := upd.object(t)["version"]; v != float64(2) || drafter.count() != 1 {
		t.Fatalf("Version %v, Entwürfe %d", v, drafter.count())
	}

	draft := call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/documents/draft", nil)
	expectStatus(t, draft, http.StatusOK)
	if draft.object(t)["documents_state"] != "entwurf_angelegt" || drafter.count() != 2 {
		t.Fatalf("Entwurf neu: %s, Entwürfe %d", draft.Body, drafter.count())
	}
}

func TestAgentPutDocumentsValidatesAndNeedsToken(t *testing.T) {
	srv, _ := newDocumentsTestServer(t)
	created := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(agentJobBody))
	expectStatus(t, created, http.StatusCreated)
	path := "/api/agent/applications/" + created.object(t)["id"].(string) + "/documents"

	cases := map[string]func(map[string]any){
		"Betreff mit Zeilenumbruch": func(b map[string]any) { b["mail_subject"] = "Bewerbung\nBcc: x@y.de" },
		"Anschreiben zu lang":       func(b map[string]any) { b["cover_letter"] = strings.Repeat("a", 3001) },
		"Anschreiben leer":          func(b map[string]any) { b["cover_letter"] = "   " },
		"Sprache":                   func(b map[string]any) { b["language"] = "fr" },
		"ohne Version":              func(b map[string]any) { delete(b, "version") },
		"Version 0":                 func(b map[string]any) { b["version"] = 0 },
		"zu viele Schwerpunkte":     func(b map[string]any) { b["highlights"] = strings.Split("a,b,c,d,e,f,g,h,i", ",") },
		"ohne Mailtext":             func(b map[string]any) { delete(b, "mail_body") },
	}
	for name, mutate := range cases {
		b := documentsBody(1)
		mutate(b)
		if res := callWith(t, srv, http.MethodPut, path, agentToken, b); res.Status != http.StatusBadRequest {
			t.Errorf("%s: Status %d; Body: %s", name, res.Status, res.Body)
		}
	}

	expectStatus(t, callWith(t, srv, http.MethodPut, path, "", documentsBody(1)), http.StatusUnauthorized)
	expectStatus(t, callWith(t, srv, http.MethodGet, "/api/agent/applications?documents_state=angefordert", "", nil),
		http.StatusUnauthorized)

	// Nicht angefordert → 409.
	expectProblem(t, callWith(t, srv, http.MethodPut, path, agentToken, documentsBody(1)), http.StatusConflict, "/problems/conflict")
}
