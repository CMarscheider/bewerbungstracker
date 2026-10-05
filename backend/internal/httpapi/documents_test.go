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

	"bewerbungsmanager/internal/documents"
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
	drafter := &fakeDrafter{}
	return newDocumentsTestServerWith(t,
		service.WithPDFConverter(fakeConverter{}), service.WithDrafter(drafter, "erika@example.com")), drafter
}

// newDocumentsTestServerWith startet einen Server mit Agent-Token und den angegebenen Diensten.
func newDocumentsTestServerWith(t *testing.T, opts ...service.Option) *httptest.Server {
	t.Helper()
	testdb.Reset(t, testPool)
	svc := service.New(testPool, time.Now, opts...)
	h, err := httpapi.NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), httpapi.WithAgentToken(agentToken))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// unavailableConverter simuliert einen nicht erreichbaren PDF-Dienst.
type unavailableConverter struct{}

func (unavailableConverter) Convert(context.Context, []byte, map[string][]byte) ([]byte, error) {
	return nil, documents.ErrUnavailable
}

// requestedApplication legt Lebenslauf und eine Stelle an (job: JSON wie agentJobBody) und fordert
// Unterlagen an; liefert den Pfad der Agent-Unterlagen und die ID.
func requestedApplication(t *testing.T, srv *httptest.Server, job string) (string, string) {
	t.Helper()
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	created := callWith(t, srv, http.MethodPost, "/api/agent/applications", agentToken, json.RawMessage(job))
	expectStatus(t, created, http.StatusCreated)
	id := created.object(t)["id"].(string)
	expectStatus(t, call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/documents/request", nil), http.StatusOK)
	return "/api/agent/applications/" + id + "/documents", id
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
	fileName, ok := docs["file_name"].(string)
	if docs["version"] != float64(1) || docs["language"] != "de" || !ok || !strings.HasPrefix(fileName, "Bewerbung_") {
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

	const (
		byValidator = "/problems/bad-request"      // OpenAPI-Validator
		byService   = "/problems/validation-error" // erst der Service erkennt es
	)
	cases := map[string]struct {
		mutate  func(map[string]any)
		problem string
	}{
		"Betreff mit Zeilenumbruch": {func(b map[string]any) { b["mail_subject"] = "Bewerbung\nBcc: x@y.de" }, byValidator},
		"Anschreiben zu lang":       {func(b map[string]any) { b["cover_letter"] = strings.Repeat("a", 3001) }, byValidator},
		"Anschreiben leer":          {func(b map[string]any) { b["cover_letter"] = "   " }, byValidator},
		"Sprache":                   {func(b map[string]any) { b["language"] = "fr" }, byValidator},
		"ohne Version":              {func(b map[string]any) { delete(b, "version") }, byValidator},
		"Version 0":                 {func(b map[string]any) { b["version"] = 0 }, byValidator},
		"zu viele Schwerpunkte":     {func(b map[string]any) { b["highlights"] = strings.Split("a,b,c,d,e,f,g,h,i", ",") }, byValidator},
		"ohne Mailtext":             {func(b map[string]any) { delete(b, "mail_body") }, byValidator},
		"Profil mit Zeilenumbruch":  {func(b map[string]any) { b["profile_line"] = "Zeile 1\nZeile 2" }, byValidator},
		"NUL im Anschreiben":        {func(b map[string]any) { b["cover_letter"] = "Hallo\x00Welt" }, byService},
	}
	for name, c := range cases {
		b := documentsBody(1)
		c.mutate(b)
		res := callWith(t, srv, http.MethodPut, path, agentToken, b)
		if res.Status != http.StatusBadRequest {
			t.Errorf("%s: Status %d; Body: %s", name, res.Status, res.Body)
			continue
		}
		if typ := res.object(t)["type"]; typ != c.problem {
			t.Errorf("%s: Problem-Typ %v, erwartet %s", name, typ, c.problem)
		}
	}

	expectStatus(t, callWith(t, srv, http.MethodPut, path, "", documentsBody(1)), http.StatusUnauthorized)
	expectStatus(t, callWith(t, srv, http.MethodGet, "/api/agent/applications?documents_state=angefordert", "", nil),
		http.StatusUnauthorized)

	// Nicht angefordert → 409.
	expectProblem(t, callWith(t, srv, http.MethodPut, path, agentToken, documentsBody(1)), http.StatusConflict, "/problems/conflict")
}

func TestAgentPutDocumentsPDFUnavailable(t *testing.T) {
	srv := newDocumentsTestServerWith(t, service.WithPDFConverter(unavailableConverter{}))
	path, id := requestedApplication(t, srv, agentJobBody)
	expectProblem(t, callWith(t, srv, http.MethodPut, path, agentToken, documentsBody(1)),
		http.StatusServiceUnavailable, "/problems/service-unavailable")
	if s := call(t, srv, http.MethodGet, "/api/v1/applications/"+id, nil).object(t)["documents_state"]; s != "angefordert" {
		t.Errorf("documents_state = %v, erwartet angefordert", s)
	}
}

func TestAgentPutDocumentsStaleVersion(t *testing.T) {
	srv, _ := newDocumentsTestServer(t)
	path, id := requestedApplication(t, srv, agentJobBody)
	expectStatus(t, callWith(t, srv, http.MethodPut, path, agentToken, documentsBody(1)), http.StatusOK)
	expectStatus(t, call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/documents/request", nil), http.StatusOK)
	changed := documentsBody(1)
	changed["cover_letter"] = "Anders formuliert."
	p := expectProblem(t, callWith(t, srv, http.MethodPut, path, agentToken, changed), http.StatusConflict, "/problems/conflict")
	if p["detail"] != "Version ist veraltet" {
		t.Errorf("detail = %v", p["detail"])
	}
}

func TestCreateDraftWithoutAddress(t *testing.T) {
	srv, drafter := newDocumentsTestServer(t)
	job := strings.Replace(agentJobBody, `"contact_email":"jobs@acme.example",`, "", 1)
	path, id := requestedApplication(t, srv, job)
	expectStatus(t, callWith(t, srv, http.MethodPut, path, agentToken, documentsBody(1)), http.StatusOK)
	expectProblem(t, call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/documents/draft", nil),
		http.StatusBadRequest, "/problems/validation-error")
	if drafter.count() != 0 {
		t.Errorf("Entwürfe: %d", drafter.count())
	}
}

func TestCreateDraftWithoutDrafter(t *testing.T) {
	srv := newDocumentsTestServerWith(t, service.WithPDFConverter(fakeConverter{}))
	path, id := requestedApplication(t, srv, agentJobBody)
	expectStatus(t, callWith(t, srv, http.MethodPut, path, agentToken, documentsBody(1)), http.StatusOK)
	expectProblem(t, call(t, srv, http.MethodPost, "/api/v1/applications/"+id+"/documents/draft", nil),
		http.StatusServiceUnavailable, "/problems/service-unavailable")
}
