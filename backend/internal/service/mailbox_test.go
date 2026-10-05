package service_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

// agentApp legt eine vorgemerkte Stelle per Agent-Job an.
func agentApp(t *testing.T, svc *service.Service) service.Application {
	t.Helper()
	app, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func sampleSuggestion(appID *uuid.UUID) service.NewSuggestion {
	return service.NewSuggestion{
		ApplicationID: appID,
		SuggestedType: domain.Absage,
		OccurredOn:    day(0),
		Reason:        "Mail klingt nach Absage, aber nicht eindeutig.",
		MailSubject:   ptr("Ihre Bewerbung"),
		MailFrom:      ptr("jobs@acme.example"),
		MailURL:       ptr("https://mail.google.com/mail/u/0/#inbox/abc"),
	}
}

func TestListOpenAgentApplications(t *testing.T) {
	svc := newService(t)
	open := agentApp(t, svc)
	closed := appliedApp(t, svc)
	mustAdd(t, svc, closed.ID, domain.NewEvent{Type: domain.Absage, OccurredOn: day(0)})

	list, err := svc.ListOpenAgentApplications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != open.ID {
		t.Fatalf("Liste = %+v", list)
	}
	got := list[0]
	if got.CompanyName != "Acme GmbH" || got.Status != domain.Vorgemerkt || got.ContactEmail == nil || got.GmailThreadID != nil {
		t.Errorf("Eintrag = %+v", got)
	}
	if !slices.Contains(got.AllowedEvents, domain.Beworben) {
		t.Errorf("AllowedEvents = %v, erwartet Beworben", got.AllowedEvents)
	}
}

func TestAgentAddEventPrefixesNote(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	e, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0), Note: ptr("Gesendet am 06.10.")})
	if err != nil {
		t.Fatal(err)
	}
	if e.Note == nil || *e.Note != "Agent: Gesendet am 06.10." {
		t.Errorf("Notiz = %v", e.Note)
	}
	e, err = svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.ScreeningGespraech, OccurredOn: day(0), Note: ptr("Agent: Termin")})
	if err != nil {
		t.Fatal(err)
	}
	if e.Note == nil || *e.Note != "Agent: Termin" {
		t.Errorf("Notiz verdoppelt: %v", e.Note)
	}
	got, _ := svc.GetApplication(ctx, app.ID)
	if got.Status != domain.ScreeningGespraech {
		t.Errorf("Status = %s", got.Status)
	}
}

func TestAgentAddEventRejectsInvalidTransition(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	_, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.AngebotAngenommen, OccurredOn: day(0)})
	var te *domain.TransitionError
	if !errors.As(err, &te) {
		t.Fatalf("erwartet TransitionError, bekommen %v", err)
	}
}

func TestSetGmailThread(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	other := appliedApp(t, svc)
	if err := svc.SetGmailThread(ctx, app.ID, "18c0ffee"); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetApplication(ctx, app.ID)
	if got.GmailThreadID == nil || *got.GmailThreadID != "18c0ffee" {
		t.Errorf("GmailThreadID = %v", got.GmailThreadID)
	}
	if err := svc.SetGmailThread(ctx, app.ID, "18c0ffee"); err != nil {
		t.Errorf("erneut setzen: %v", err)
	}
	var ce *service.ConflictError
	if err := svc.SetGmailThread(ctx, other.ID, "18c0ffee"); !errors.As(err, &ce) {
		t.Errorf("erwartet ConflictError, bekommen %v", err)
	}
	var nf *service.NotFoundError
	if err := svc.SetGmailThread(ctx, uuid.New(), "abc"); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
	var ve *domain.ValidationError
	if err := svc.SetGmailThread(ctx, app.ID, " "); !errors.As(err, &ve) {
		t.Errorf("erwartet ValidationError, bekommen %v", err)
	}
}

func TestCreateAndListSuggestions(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	assigned, err := svc.CreateSuggestion(ctx, sampleSuggestion(&app.ID))
	if err != nil {
		t.Fatal(err)
	}
	if assigned.State != service.SuggestionOpen || assigned.CompanyName == nil || *assigned.CompanyName != "Acme GmbH" {
		t.Errorf("Vorschlag = %+v", assigned)
	}
	unassigned, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
	if err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListOpenSuggestions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != assigned.ID || list[1].ID != unassigned.ID {
		t.Fatalf("Liste = %+v", list)
	}
	if list[0].PositionTitle == nil || *list[0].PositionTitle != app.PositionTitle || list[0].MailURL == nil {
		t.Errorf("zugeordnet = %+v", list[0])
	}
	if list[1].ApplicationID != nil || list[1].CompanyName != nil || list[1].PositionTitle != nil {
		t.Errorf("nicht zugeordnet = %+v", list[1])
	}
}

func TestCreateSuggestionValidation(t *testing.T) {
	svc := newService(t)
	var nf *service.NotFoundError
	if _, err := svc.CreateSuggestion(ctx, sampleSuggestion(ptr(uuid.New()))); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
	empty := sampleSuggestion(nil)
	empty.Reason = "  "
	var ve *domain.ValidationError
	if _, err := svc.CreateSuggestion(ctx, empty); !errors.As(err, &ve) || ve.Field != "reason" {
		t.Errorf("erwartet ValidationError reason, bekommen %v", err)
	}
	badType := sampleSuggestion(nil)
	badType.SuggestedType = "Quatsch"
	if _, err := svc.CreateSuggestion(ctx, badType); !errors.As(err, &ve) || ve.Field != "suggested_type" {
		t.Errorf("erwartet ValidationError suggested_type, bekommen %v", err)
	}
}

func TestAcceptAssignedSuggestion(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	s, err := svc.CreateSuggestion(ctx, sampleSuggestion(&app.ID))
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.AcceptSuggestion(ctx, s.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	last := got.Events[len(got.Events)-1]
	if got.Status != domain.Absage || last.Note == nil || *last.Note != "Agent: "+s.Reason {
		t.Errorf("Status=%s Notiz=%v", got.Status, last.Note)
	}
	if list, _ := svc.ListOpenSuggestions(ctx); len(list) != 0 {
		t.Errorf("Vorschlag noch offen: %+v", list)
	}
	var ce *service.ConflictError
	if _, err := svc.AcceptSuggestion(ctx, s.ID, nil); !errors.As(err, &ce) {
		t.Errorf("erwartet ConflictError, bekommen %v", err)
	}
}

func TestAcceptUnassignedSuggestion(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	s, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
	if err != nil {
		t.Fatal(err)
	}
	var ve *domain.ValidationError
	if _, err := svc.AcceptSuggestion(ctx, s.ID, nil); !errors.As(err, &ve) || ve.Field != "application_id" {
		t.Fatalf("erwartet ValidationError application_id, bekommen %v", err)
	}
	got, err := svc.AcceptSuggestion(ctx, s.ID, &app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != app.ID || got.Status != domain.Absage {
		t.Errorf("Bewerbung = %+v", got)
	}
}

func TestAcceptSuggestionInvalidTransitionKeepsOpen(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc) // Vorgemerkt: Absage ist nicht erlaubt
	s, err := svc.CreateSuggestion(ctx, sampleSuggestion(&app.ID))
	if err != nil {
		t.Fatal(err)
	}
	var te *domain.TransitionError
	if _, err := svc.AcceptSuggestion(ctx, s.ID, nil); !errors.As(err, &te) {
		t.Fatalf("erwartet TransitionError, bekommen %v", err)
	}
	list, _ := svc.ListOpenSuggestions(ctx)
	if len(list) != 1 || list[0].State != service.SuggestionOpen {
		t.Errorf("Vorschlag nicht mehr offen: %+v", list)
	}
}

func TestAcceptSuggestionUnknown(t *testing.T) {
	svc := newService(t)
	var nf *service.NotFoundError
	if _, err := svc.AcceptSuggestion(ctx, uuid.New(), nil); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
}

func TestDismissSuggestion(t *testing.T) {
	svc := newService(t)
	s, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DismissSuggestion(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := svc.ListOpenSuggestions(ctx); len(list) != 0 {
		t.Errorf("Vorschlag noch offen: %+v", list)
	}
	var ce *service.ConflictError
	if err := svc.DismissSuggestion(ctx, s.ID); !errors.As(err, &ce) {
		t.Errorf("erwartet ConflictError, bekommen %v", err)
	}
	var nf *service.NotFoundError
	if err := svc.DismissSuggestion(ctx, uuid.New()); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
}

func TestMarkMailProcessed(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	m, created, err := svc.MarkMailProcessed(ctx, "msg-1", &app.ID, "beworben")
	if err != nil || !created {
		t.Fatalf("erstes Merken: created=%v err=%v", created, err)
	}
	if m.GmailMessageID != "msg-1" || m.ApplicationID == nil || *m.ApplicationID != app.ID || m.Outcome != "beworben" {
		t.Errorf("Eintrag = %+v", m)
	}
	again, created, err := svc.MarkMailProcessed(ctx, "msg-1", nil, "anderes")
	if err != nil || created {
		t.Fatalf("erneutes Merken: created=%v err=%v", created, err)
	}
	if again.Outcome != "beworben" {
		t.Errorf("vorhandener Eintrag überschrieben: %+v", again)
	}

	if got, err := svc.GetProcessedMail(ctx, "msg-1"); err != nil || got.Outcome != "beworben" {
		t.Errorf("GetProcessedMail = %+v, %v", got, err)
	}
	var nf *service.NotFoundError
	if _, err := svc.GetProcessedMail(ctx, "msg-2"); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
}

func TestMarkMailProcessedValidation(t *testing.T) {
	svc := newService(t)
	var ve *domain.ValidationError
	if _, _, err := svc.MarkMailProcessed(ctx, " ", nil, "ignoriert"); !errors.As(err, &ve) || ve.Field != "gmail_message_id" {
		t.Errorf("erwartet ValidationError gmail_message_id, bekommen %v", err)
	}
	if _, _, err := svc.MarkMailProcessed(ctx, "msg-1", nil, strings.Repeat("x", 201)); !errors.As(err, &ve) || ve.Field != "outcome" {
		t.Errorf("erwartet ValidationError outcome, bekommen %v", err)
	}
	if _, _, err := svc.MarkMailProcessed(ctx, "msg-1", nil, ""); !errors.As(err, &ve) || ve.Field != "outcome" {
		t.Errorf("erwartet ValidationError outcome (leer), bekommen %v", err)
	}
	var nf *service.NotFoundError
	if _, _, err := svc.MarkMailProcessed(ctx, "msg-1", ptr(uuid.New()), "x"); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
}
