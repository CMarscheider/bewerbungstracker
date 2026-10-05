package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/documents"
	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/mail"
	"bewerbungsmanager/internal/service"
)

func TestRequestDocumentsSetsRequested(t *testing.T) {
	svc := newService(t)
	a, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	if a.DocumentsState != service.DocsNone {
		t.Fatalf("Start: %s", a.DocumentsState)
	}
	got, err := svc.RequestDocuments(ctx, a.ID)
	if err != nil || got.DocumentsState != service.DocsRequested {
		t.Fatalf("angefordert: %v %s", err, got.DocumentsState)
	}
	if _, err := svc.RequestDocuments(ctx, a.ID); err != nil { // idempotent
		t.Fatal(err)
	}
	list, err := svc.ListAgentApplications(ctx, service.DocsRequested)
	if err != nil || len(list) != 1 || list[0].ID != a.ID || list[0].PostingText == nil {
		t.Fatalf("Agent-Liste: %v %+v", err, list)
	}
	if list[0].CompanyName != "Acme GmbH" || list[0].DocumentsVersion != 0 || list[0].DocumentsState != service.DocsRequested {
		t.Fatalf("Agent-Felder: %+v", list[0])
	}
	none, err := svc.ListAgentApplications(ctx, service.DocsNone)
	if err != nil || len(none) != 0 {
		t.Fatalf("keine: %v %+v", err, none)
	}
}

func TestRequestDocumentsUnknownApplication(t *testing.T) {
	svc := newService(t)
	var nf *service.NotFoundError
	if _, err := svc.RequestDocuments(ctx, uuid.New()); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFound, bekommen %v", err)
	}
}

func TestListAgentApplicationsUnknownState(t *testing.T) {
	svc := newService(t)
	var ve *domain.ValidationError
	if _, err := svc.ListAgentApplications(ctx, "quatsch"); !errors.As(err, &ve) || ve.Field != "documents_state" {
		t.Fatalf("erwartet ValidationError documents_state, bekommen %v", err)
	}
}

type fakeDrafter struct {
	calls int
	last  []byte
	err   error
}

func (f *fakeDrafter) Save(_ context.Context, msg []byte) error {
	f.calls++
	f.last = msg
	return f.err
}

func sampleDocs() service.DocumentsInput {
	return service.DocumentsInput{
		Version: 1, Language: "de",
		CoverLetter: "Sehr geehrte Damen und Herren,\n\nich bewerbe mich.",
		MailSubject: "Bewerbung als Junior Frontend-Entwickler",
		MailBody:    "Guten Tag,\n\nanbei meine Unterlagen.",
		Highlights:  []string{" TypeScript ", ""},
	}
}

// docsSetup legt Lebenslauf und eine angeforderte Stelle an.
func docsSetup(t *testing.T, job service.AgentJob, opts ...service.Option) (*service.Service, uuid.UUID) {
	t.Helper()
	svc := newServiceWith(t, opts...)
	saveSampleCV(t, svc)
	a, err := svc.CreateAgentJob(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestDocuments(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	return svc, a.ID
}

func mustState(t *testing.T, svc *service.Service, id uuid.UUID, want string) service.Application {
	t.Helper()
	a, err := svc.GetApplication(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if a.DocumentsState != want {
		t.Fatalf("Zustand = %s (Fehler %v), erwartet %s", a.DocumentsState, a.DocumentsError, want)
	}
	return a
}

func TestSaveAgentDocumentsCreatesDraft(t *testing.T) {
	conv, dr := &fakeConverter{}, &fakeDrafter{}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(conv), service.WithDrafter(dr, "erika@example.com"))

	d, err := svc.SaveAgentDocuments(ctx, id, sampleDocs())
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != 1 || d.FileName == nil || *d.FileName != "Bewerbung_Mustermann_Acme_GmbH.pdf" || d.RenderedAt == nil {
		t.Fatalf("Unterlagen: %+v", d)
	}
	if len(d.Highlights) != 1 || d.Highlights[0] != "TypeScript" {
		t.Errorf("Highlights = %q", d.Highlights)
	}
	if !strings.Contains(string(conv.html), "ich bewerbe mich.") {
		t.Error("Anschreiben fehlt im HTML")
	}
	a := mustState(t, svc, id, service.DocsDrafted)
	if a.GmailDraftAt == nil || a.DocumentsError != nil {
		t.Errorf("Entwurf: %v %v", a.GmailDraftAt, a.DocumentsError)
	}
	msg := string(dr.last)
	if dr.calls != 1 || !strings.Contains(msg, "To: <jobs@acme.example>") ||
		!strings.Contains(msg, `filename="Bewerbung_Mustermann_Acme_GmbH.pdf"`) ||
		!strings.Contains(msg, `From: "Erika Mustermann" <erika@example.com>`) {
		t.Fatalf("Drafter %d×, Nachricht:\n%s", dr.calls, msg)
	}
	got, err := svc.GetDocuments(ctx, id)
	if err != nil || got.Version != 1 || got.MailSubject != sampleDocs().MailSubject {
		t.Fatalf("GetDocuments: %v %+v", err, got)
	}
	pdf, name, err := svc.DocumentsPDF(ctx, id)
	if err != nil || string(pdf) != "%PDF-fake" || name != "Bewerbung_Mustermann_Acme_GmbH.pdf" {
		t.Fatalf("DocumentsPDF: %v %q %q", err, pdf, name)
	}

	// Idempotent: gleiche Lieferung erneut.
	again, err := svc.SaveAgentDocuments(ctx, id, sampleDocs())
	if err != nil || again.Version != 1 || dr.calls != 1 {
		t.Fatalf("idempotent: %v %+v, Drafter %d×", err, again, dr.calls)
	}
	mustState(t, svc, id, service.DocsDrafted)

	// Gleiche Version, anderer Inhalt.
	changed := sampleDocs()
	changed.CoverLetter = "Anders."
	var ce *service.ConflictError
	if _, err := svc.SaveAgentDocuments(ctx, id, changed); !errors.As(err, &ce) {
		t.Fatalf("erwartet Conflict, bekommen %v", err)
	}
}

func TestSaveAgentDocumentsWithoutRequestIsConflict(t *testing.T) {
	svc := newServiceWith(t, service.WithPDFConverter(&fakeConverter{}))
	saveSampleCV(t, svc)
	a, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	var ce *service.ConflictError
	if _, err := svc.SaveAgentDocuments(ctx, a.ID, sampleDocs()); !errors.As(err, &ce) {
		t.Fatalf("erwartet Conflict, bekommen %v", err)
	}
	var nf *service.NotFoundError
	if _, err := svc.SaveAgentDocuments(ctx, uuid.New(), sampleDocs()); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFound, bekommen %v", err)
	}
}

func TestSaveAgentDocumentsStaleVersion(t *testing.T) {
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestDocuments(ctx, id); err != nil {
		t.Fatal(err)
	}
	changed := sampleDocs()
	changed.CoverLetter = "Neu."
	var ce *service.ConflictError
	if _, err := svc.SaveAgentDocuments(ctx, id, changed); !errors.As(err, &ce) {
		t.Fatalf("Version 1 erneut: erwartet Conflict, bekommen %v", err)
	}
	changed.Version = 2
	if d, err := svc.SaveAgentDocuments(ctx, id, changed); err != nil || d.Version != 2 {
		t.Fatalf("Version 2: %v %+v", err, d)
	}
}

func TestSaveAgentDocumentsWithoutAddressIsPortal(t *testing.T) {
	dr := &fakeDrafter{}
	job := sampleJob()
	job.ContactEmail = nil
	svc, id := docsSetup(t, job, service.WithPDFConverter(&fakeConverter{}), service.WithDrafter(dr, "erika@example.com"))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	mustState(t, svc, id, service.DocsPortal)
	if dr.calls != 0 {
		t.Errorf("Drafter %d× aufgerufen", dr.calls)
	}
}

func TestSaveAgentDocumentsWithoutDrafterIsCreated(t *testing.T) {
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	mustState(t, svc, id, service.DocsCreated)
}

func TestSaveAgentDocumentsPDFUnavailable(t *testing.T) {
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{err: documents.ErrUnavailable}))
	var ue *service.UnavailableError
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); !errors.As(err, &ue) {
		t.Fatalf("erwartet Unavailable, bekommen %v", err)
	}
	mustState(t, svc, id, service.DocsRequested)
	var nf *service.NotFoundError
	if _, err := svc.GetDocuments(ctx, id); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFound, bekommen %v", err)
	}

	svc, id = docsSetup(t, sampleJob()) // ganz ohne Konverter
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); !errors.As(err, &ue) {
		t.Fatalf("ohne Konverter: erwartet Unavailable, bekommen %v", err)
	}
	mustState(t, svc, id, service.DocsRequested)
}

func TestSaveAgentDocumentsRenderFailure(t *testing.T) {
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{err: errors.New("kaputt")}))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err == nil {
		t.Fatal("erwartet Fehler")
	}
	a := mustState(t, svc, id, service.DocsFailed)
	if a.DocumentsError == nil || !strings.Contains(*a.DocumentsError, "kaputt") {
		t.Errorf("DocumentsError = %v", a.DocumentsError)
	}

	// Erneut anfordern setzt Zustand und Fehler zurück.
	re, err := svc.RequestDocuments(ctx, id)
	if err != nil || re.DocumentsState != service.DocsRequested || re.DocumentsError != nil {
		t.Fatalf("erneut angefordert: %v %s %v", err, re.DocumentsState, re.DocumentsError)
	}
}

func TestRequestDocumentsKeepsUpdatedAtWhenAlreadyRequested(t *testing.T) {
	svc, id := docsSetup(t, sampleJob())
	before := mustState(t, svc, id, service.DocsRequested)
	after, err := svc.RequestDocuments(ctx, id)
	if err != nil || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("updated_at geändert: %v %v → %v", err, before.UpdatedAt, after.UpdatedAt)
	}
}

func TestSaveAgentDocumentsDraftFailureKeepsDocuments(t *testing.T) {
	dr := &fakeDrafter{err: errors.New("imap kaputt")}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}), service.WithDrafter(dr, "erika@example.com"))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatalf("Entwurfsfehler darf nicht an den Aufrufer: %v", err)
	}
	a := mustState(t, svc, id, service.DocsCreated)
	if a.DocumentsError == nil || !strings.HasPrefix(*a.DocumentsError, "Gmail-Entwurf konnte nicht angelegt werden: ") {
		t.Errorf("DocumentsError = %v", a.DocumentsError)
	}
	if _, err := svc.GetDocuments(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateDocuments(t *testing.T) {
	conv, dr := &fakeConverter{}, &fakeDrafter{}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(conv), service.WithDrafter(dr, "erika@example.com"))
	in := sampleDocs()
	in.Version = 0
	var nf *service.NotFoundError
	if _, err := svc.UpdateDocuments(ctx, id, in); !errors.As(err, &nf) {
		t.Fatalf("ohne Unterlagen: erwartet NotFound, bekommen %v", err)
	}
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	in.CoverLetter = "Überarbeitet von Hand."
	d, err := svc.UpdateDocuments(ctx, id, in)
	if err != nil || d.Version != 2 || d.CoverLetter != "Überarbeitet von Hand." {
		t.Fatalf("Update: %v %+v", err, d)
	}
	if !strings.Contains(string(conv.html), "Überarbeitet von Hand.") {
		t.Error("nicht neu gerendert")
	}
	mustState(t, svc, id, service.DocsDrafted)
	if dr.calls != 1 {
		t.Errorf("Update darf keinen Entwurf anlegen, Drafter %d×", dr.calls)
	}
}

func TestUpdateDocumentsRenderFailureKeepsStateAndPDF(t *testing.T) {
	conv := &fakeConverter{}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(conv), service.WithDrafter(&fakeDrafter{}, "erika@example.com"))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	conv.err = errors.New("kaputt")
	in := sampleDocs()
	in.Version, in.CoverLetter = 0, "Neu."
	if _, err := svc.UpdateDocuments(ctx, id, in); err == nil || !strings.Contains(err.Error(), "kaputt") {
		t.Fatalf("erwartet Renderfehler, bekommen %v", err)
	}
	if a := mustState(t, svc, id, service.DocsDrafted); a.DocumentsError != nil {
		t.Errorf("DocumentsError = %v", *a.DocumentsError)
	}
	d, err := svc.GetDocuments(ctx, id)
	if err != nil || d.Version != 1 || d.CoverLetter != sampleDocs().CoverLetter {
		t.Fatalf("alte Fassung verloren: %v %+v", err, d)
	}
	if pdf, _, err := svc.DocumentsPDF(ctx, id); err != nil || string(pdf) != "%PDF-fake" {
		t.Fatalf("altes PDF verloren: %v %q", err, pdf)
	}

	// Auch eine neue Anforderung bleibt bei einem gescheiterten Bearbeiten bestehen.
	if _, err := svc.RequestDocuments(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateDocuments(ctx, id, in); err == nil {
		t.Fatal("erwartet Fehler")
	}
	mustState(t, svc, id, service.DocsRequested)
}

// failVersion2 legt Version 1 an, fordert neu an und lässt Version 2 beim Rendern scheitern.
func failVersion2(t *testing.T, job service.AgentJob) (*service.Service, uuid.UUID, *fakeConverter) {
	t.Helper()
	conv := &fakeConverter{}
	svc, id := docsSetup(t, job, service.WithPDFConverter(conv))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestDocuments(ctx, id); err != nil {
		t.Fatal(err)
	}
	conv.err = errors.New("kaputt")
	v2 := sampleDocs()
	v2.Version, v2.CoverLetter = 2, "Version 2."
	if _, err := svc.SaveAgentDocuments(ctx, id, v2); err == nil {
		t.Fatal("erwartet Fehler")
	}
	mustState(t, svc, id, service.DocsFailed)
	conv.err = nil
	return svc, id, conv
}

func TestUpdateDocumentsAfterFailureIsCreated(t *testing.T) {
	svc, id, _ := failVersion2(t, sampleJob())
	in := sampleDocs()
	in.Version = 0
	if _, err := svc.UpdateDocuments(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if a := mustState(t, svc, id, service.DocsCreated); a.DocumentsError != nil {
		t.Errorf("DocumentsError = %v", *a.DocumentsError)
	}
}

func TestUpdateDocumentsAfterFailureWithoutAddressIsPortal(t *testing.T) {
	job := sampleJob()
	job.ContactEmail = nil
	svc, id, _ := failVersion2(t, job)
	in := sampleDocs()
	in.Version = 0
	if _, err := svc.UpdateDocuments(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if a := mustState(t, svc, id, service.DocsPortal); a.DocumentsError != nil {
		t.Errorf("DocumentsError = %v", *a.DocumentsError)
	}
}

// hookConverter ruft beim ersten Convert einmal hook auf (z. B. eine parallele Lieferung).
// Der beim ersten Aufruf gelieferte Fehler ist err; alle weiteren Aufrufe gelingen.
type hookConverter struct {
	hook func()
	err  error
	done bool
}

func (h *hookConverter) Convert(_ context.Context, _ []byte, _ map[string][]byte) ([]byte, error) {
	if !h.done {
		h.done = true
		h.hook()
		if h.err != nil {
			return nil, h.err
		}
	}
	return []byte("%PDF-fake"), nil
}

func TestSaveAgentDocumentsConcurrentIdenticalDelivery(t *testing.T) {
	conv, dr := &hookConverter{}, &fakeDrafter{}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(conv), service.WithDrafter(dr, "erika@example.com"))
	conv.hook = func() {
		if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
			t.Errorf("parallele Lieferung: %v", err)
		}
	}
	d, err := svc.SaveAgentDocuments(ctx, id, sampleDocs())
	if err != nil || d.Version != 1 {
		t.Fatalf("gleiche Lieferung während des Renderns: %v %+v", err, d)
	}
	if dr.calls != 1 {
		t.Errorf("Drafter %d×, erwartet 1", dr.calls)
	}
	mustState(t, svc, id, service.DocsDrafted)
}

func TestRenderFailureDoesNotOverwriteNewerState(t *testing.T) {
	conv := &hookConverter{err: errors.New("kaputt")}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(conv))
	// Während des gescheiterten Renderns liefert eine parallele Lieferung erfolgreich.
	conv.hook = func() {
		if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
			t.Errorf("parallele Lieferung: %v", err)
		}
	}
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err == nil {
		t.Fatal("erwartet Renderfehler")
	}
	if a := mustState(t, svc, id, service.DocsCreated); a.DocumentsError != nil {
		t.Errorf("DocumentsError = %v", *a.DocumentsError)
	}
}

// hookDrafter ruft vor dem Scheitern hook auf.
type hookDrafter struct{ hook func() }

func (h *hookDrafter) Save(context.Context, []byte) error {
	h.hook()
	return errors.New("imap kaputt")
}

func TestDraftFailureDoesNotOverwriteNewerState(t *testing.T) {
	dr := &hookDrafter{}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}), service.WithDrafter(dr, "erika@example.com"))
	dr.hook = func() {
		if _, err := svc.RequestDocuments(ctx, id); err != nil {
			t.Error(err)
		}
	}
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	if a := mustState(t, svc, id, service.DocsRequested); a.DocumentsError != nil {
		t.Errorf("DocumentsError = %v", *a.DocumentsError)
	}
}

func TestSaveAgentDocumentsRejectsLineBreakInSubject(t *testing.T) {
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}))
	for _, subj := range []string{"Bewerbung\nBcc: x@y.z", "Bewerbung\r\nBcc: x@y.z", "Bewerbung\rX"} {
		in := sampleDocs()
		in.MailSubject = subj
		var ve *domain.ValidationError
		if _, err := svc.SaveAgentDocuments(ctx, id, in); !errors.As(err, &ve) || ve.Field != "mail_subject" {
			t.Errorf("%q: erwartet ValidationError mail_subject, bekommen %v", subj, err)
		}
	}
}

func TestCreateDraftWhileRequestedIsConflict(t *testing.T) {
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}), service.WithDrafter(&fakeDrafter{}, "erika@example.com"))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestDocuments(ctx, id); err != nil {
		t.Fatal(err)
	}
	var ce *service.ConflictError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &ce) || !strings.Contains(ce.Detail, "gerade erstellt") {
		t.Fatalf("erwartet Conflict, bekommen %v", err)
	}
}

func TestCreateDraftErrorMapping(t *testing.T) {
	dr := &fakeDrafter{}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}), service.WithDrafter(dr, "erika@example.com"))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	dr.err = fmt.Errorf("x: %w", mail.ErrNoDrafts)
	var ce *service.ConflictError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &ce) || !strings.Contains(ce.Detail, "Entwurfsordner") {
		t.Fatalf("ErrNoDrafts: erwartet Conflict, bekommen %v", err)
	}
	dr.err = fmt.Errorf("x: %w", context.DeadlineExceeded)
	var ue *service.UnavailableError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &ue) {
		t.Fatalf("DeadlineExceeded: erwartet Unavailable, bekommen %v", err)
	}
}

func TestCreateDraft(t *testing.T) {
	dr := &fakeDrafter{}
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}), service.WithDrafter(dr, "erika@example.com"))
	var nf *service.NotFoundError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &nf) {
		t.Fatalf("ohne Unterlagen: erwartet NotFound, bekommen %v", err)
	}
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	a, err := svc.CreateDraft(ctx, id)
	if err != nil || a.DocumentsState != service.DocsDrafted || dr.calls != 2 {
		t.Fatalf("CreateDraft: %v %s, Drafter %d×", err, a.DocumentsState, dr.calls)
	}

	dr.err = fmt.Errorf("x: %w", mail.ErrAuth)
	var ce *service.ConflictError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &ce) || !strings.Contains(ce.Detail, "App-Passwort") {
		t.Fatalf("Anmeldefehler: erwartet Conflict, bekommen %v", err)
	}
	dr.err = fmt.Errorf("x: %w", mail.ErrUnavailable)
	var ue *service.UnavailableError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &ue) {
		t.Fatalf("IMAP weg: erwartet Unavailable, bekommen %v", err)
	}
	mustState(t, svc, id, service.DocsDrafted)
}

func TestCreateDraftWithoutAddressOrDrafter(t *testing.T) {
	job := sampleJob()
	job.ContactEmail = nil
	svc, id := docsSetup(t, job, service.WithPDFConverter(&fakeConverter{}), service.WithDrafter(&fakeDrafter{}, "erika@example.com"))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	var ve *domain.ValidationError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &ve) || ve.Field != "contact_email" {
		t.Fatalf("ohne Adresse: erwartet ValidationError contact_email, bekommen %v", err)
	}

	svc, id = docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}))
	if _, err := svc.SaveAgentDocuments(ctx, id, sampleDocs()); err != nil {
		t.Fatal(err)
	}
	var ue *service.UnavailableError
	if _, err := svc.CreateDraft(ctx, id); !errors.As(err, &ue) {
		t.Fatalf("ohne Drafter: erwartet Unavailable, bekommen %v", err)
	}
}

func TestSaveAgentDocumentsValidation(t *testing.T) {
	svc, id := docsSetup(t, sampleJob(), service.WithPDFConverter(&fakeConverter{}))
	cases := map[string]func(*service.DocumentsInput){
		"language":     func(in *service.DocumentsInput) { in.Language = "fr" },
		"cover_letter": func(in *service.DocumentsInput) { in.CoverLetter = "  " },
		"mail_subject": func(in *service.DocumentsInput) { in.MailSubject = "" },
		"mail_body":    func(in *service.DocumentsInput) { in.MailBody = "\n" },
		"version":      func(in *service.DocumentsInput) { in.Version = 0 },
		"highlights":   func(in *service.DocumentsInput) { in.Highlights = strings.Fields("a b c d e f g h i") },
	}
	for field, mutate := range cases {
		in := sampleDocs()
		mutate(&in)
		var ve *domain.ValidationError
		if _, err := svc.SaveAgentDocuments(ctx, id, in); !errors.As(err, &ve) || ve.Field != field {
			t.Errorf("%s: erwartet ValidationError, bekommen %v", field, err)
		}
	}
	in := sampleDocs()
	in.Version, in.Language = 0, "fr"
	var ve *domain.ValidationError
	if _, err := svc.UpdateDocuments(ctx, id, in); !errors.As(err, &ve) || ve.Field != "language" {
		t.Errorf("Update: erwartet ValidationError language, bekommen %v", err)
	}
}

func TestSaveAgentDocumentsWithoutCV(t *testing.T) {
	svc := newServiceWith(t, service.WithPDFConverter(&fakeConverter{}))
	a, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestDocuments(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	var ce *service.ConflictError
	if _, err := svc.SaveAgentDocuments(ctx, a.ID, sampleDocs()); !errors.As(err, &ce) || !strings.Contains(ce.Detail, "Lebenslauf") {
		t.Fatalf("erwartet Conflict, bekommen %v", err)
	}
	mustState(t, svc, a.ID, service.DocsRequested)
}
