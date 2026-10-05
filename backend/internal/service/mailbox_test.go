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

func TestListOpenAgentApplicationsIncludesKeineRueckmeldung(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.KeineRueckmeldung, OccurredOn: day(-1)})

	list, err := svc.ListOpenAgentApplications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != app.ID || list[0].Status != domain.KeineRueckmeldung {
		t.Fatalf("Liste = %+v", list)
	}
	want, err := svc.AllowedEvents(ctx, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(list[0].AllowedEvents, want) || !slices.Contains(want, domain.Absage) {
		t.Errorf("AllowedEvents = %v, erwartet %v", list[0].AllowedEvents, want)
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
	_, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(0)})
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
	assigned, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(&app.ID))
	if err != nil {
		t.Fatal(err)
	}
	if assigned.State != service.SuggestionOpen || assigned.CompanyName == nil || *assigned.CompanyName != "Acme GmbH" {
		t.Errorf("Vorschlag = %+v", assigned)
	}
	unassigned, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
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
	if _, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(ptr(uuid.New()))); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
	empty := sampleSuggestion(nil)
	empty.Reason = "  "
	var ve *domain.ValidationError
	if _, _, err := svc.CreateSuggestion(ctx, empty); !errors.As(err, &ve) || ve.Field != "reason" {
		t.Errorf("erwartet ValidationError reason, bekommen %v", err)
	}
	badType := sampleSuggestion(nil)
	badType.SuggestedType = "Quatsch"
	if _, _, err := svc.CreateSuggestion(ctx, badType); !errors.As(err, &ve) || ve.Field != "suggested_type" {
		t.Errorf("erwartet ValidationError suggested_type, bekommen %v", err)
	}
}

func TestAcceptAssignedSuggestion(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	s, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(&app.ID))
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
	s, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
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
	s, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(&app.ID))
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
	s, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
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

// suggestionAppID liest die gespeicherte Zuordnung eines Vorschlags direkt aus der Datenbank.
func suggestionAppID(t *testing.T, id uuid.UUID) (*uuid.UUID, string) {
	t.Helper()
	var (
		appID *uuid.UUID
		state string
	)
	err := testPool.QueryRow(ctx, "SELECT application_id, state FROM status_suggestions WHERE id = $1", id).Scan(&appID, &state)
	if err != nil {
		t.Fatal(err)
	}
	return appID, state
}

func TestAcceptSuggestionOverridesApplication(t *testing.T) {
	svc := newService(t)
	assignedTo := agentApp(t, svc)
	other := appliedApp(t, svc)
	s, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(&assignedTo.ID))
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.AcceptSuggestion(ctx, s.ID, &other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != other.ID || got.Status != domain.Absage {
		t.Errorf("Bewerbung = %+v", got)
	}
	if first, _ := svc.GetApplication(ctx, assignedTo.ID); first.Status != domain.Vorgemerkt {
		t.Errorf("ursprüngliche Bewerbung verändert: %s", first.Status)
	}
	if appID, state := suggestionAppID(t, s.ID); appID == nil || *appID != other.ID || state != service.SuggestionAccepted {
		t.Errorf("Vorschlag: application_id=%v state=%s", appID, state)
	}
}

func TestAcceptSuggestionUnknownApplicationKeepsOpen(t *testing.T) {
	svc := newService(t)
	s, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
	if err != nil {
		t.Fatal(err)
	}
	var nf *service.NotFoundError
	if _, err := svc.AcceptSuggestion(ctx, s.ID, ptr(uuid.New())); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
	if appID, state := suggestionAppID(t, s.ID); appID != nil || state != service.SuggestionOpen {
		t.Errorf("Vorschlag: application_id=%v state=%s", appID, state)
	}
}

func TestAcceptUnassignedSuggestionStoresApplication(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	s, _, err := svc.CreateSuggestion(ctx, sampleSuggestion(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AcceptSuggestion(ctx, s.ID, &app.ID); err != nil {
		t.Fatal(err)
	}
	if appID, state := suggestionAppID(t, s.ID); appID == nil || *appID != app.ID || state != service.SuggestionAccepted {
		t.Errorf("Vorschlag: application_id=%v state=%s", appID, state)
	}
}

func TestAcceptSuggestionDoesNotDoublePrefix(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	in := sampleSuggestion(&app.ID)
	in.Reason = "Agent: Absage erkannt"
	s, _, err := svc.CreateSuggestion(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.AcceptSuggestion(ctx, s.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if note := got.Events[len(got.Events)-1].Note; note == nil || *note != "Agent: Absage erkannt" {
		t.Errorf("Notiz = %v", note)
	}
}

func TestAgentAddEventDefaultNote(t *testing.T) {
	for name, note := range map[string]*string{"nil": nil, "leer": ptr("  ")} {
		t.Run(name, func(t *testing.T) {
			svc := newService(t)
			app := agentApp(t, svc)
			e, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0), Note: note})
			if err != nil {
				t.Fatal(err)
			}
			if e.Note == nil || *e.Note != "Agent: automatisch erfasst" {
				t.Errorf("Notiz = %v", e.Note)
			}
		})
	}
}

func TestSetGmailThreadRejectsDifferentID(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	if err := svc.SetGmailThread(ctx, app.ID, "thread-a"); err != nil {
		t.Fatal(err)
	}
	var ce *service.ConflictError
	if err := svc.SetGmailThread(ctx, app.ID, "thread-b"); !errors.As(err, &ce) {
		t.Fatalf("erwartet ConflictError, bekommen %v", err)
	}
	got, _ := svc.GetApplication(ctx, app.ID)
	if got.GmailThreadID == nil || *got.GmailThreadID != "thread-a" {
		t.Errorf("GmailThreadID = %v", got.GmailThreadID)
	}
	var ve *domain.ValidationError
	if err := svc.SetGmailThread(ctx, app.ID, strings.Repeat("a", 101)); !errors.As(err, &ve) || ve.Field != "gmail_thread_id" {
		t.Errorf("erwartet ValidationError gmail_thread_id, bekommen %v", err)
	}
}

func TestCreateSuggestionRejectsDueOnAndLongReason(t *testing.T) {
	svc := newService(t)
	var ve *domain.ValidationError
	withDue := sampleSuggestion(nil) // Absage hat keine Frist
	withDue.DueOn = ptr(day(5))
	if _, _, err := svc.CreateSuggestion(ctx, withDue); !errors.As(err, &ve) || ve.Field != "due_on" {
		t.Errorf("erwartet ValidationError due_on, bekommen %v", err)
	}
	long := sampleSuggestion(nil)
	long.Reason = strings.Repeat("ä", 1001)
	if _, _, err := svc.CreateSuggestion(ctx, long); !errors.As(err, &ve) || ve.Field != "reason" {
		t.Errorf("erwartet ValidationError reason, bekommen %v", err)
	}
	ok := sampleSuggestion(nil)
	ok.Reason = strings.Repeat("ä", 1000)
	if _, _, err := svc.CreateSuggestion(ctx, ok); err != nil {
		t.Errorf("1000 Zeichen abgelehnt: %v", err)
	}
}

func TestListOpenAgentApplicationsMailSubjectAndOrder(t *testing.T) {
	svc := newService(t)
	vorgemerkt := domain.NewEvent{Type: domain.Vorgemerkt, OccurredOn: day(0)}
	beta, err := svc.CreateApplication(ctx, newApp(mustCompany(t, svc, "beta").ID, "Frontend", vorgemerkt))
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := svc.CreateApplication(ctx, newApp(mustCompany(t, svc, "Alpha").ID, "Frontend", vorgemerkt))
	if err != nil {
		t.Fatal(err)
	}
	_, err = testPool.Exec(ctx, `INSERT INTO application_documents (application_id, version, language, cover_letter, mail_subject, mail_body)
		VALUES ($1, 1, 'de', 'Anschreiben', 'Bewerbung als Frontend-Entwickler', 'Hallo')`, beta.ID)
	if err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListOpenAgentApplications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != alpha.ID || list[1].ID != beta.ID {
		t.Fatalf("Reihenfolge = %+v", list)
	}
	if list[0].MailSubject != nil {
		t.Errorf("Alpha ohne Unterlagen hat Betreff %v", *list[0].MailSubject)
	}
	if list[1].MailSubject == nil || *list[1].MailSubject != "Bewerbung als Frontend-Entwickler" {
		t.Errorf("MailSubject = %v", list[1].MailSubject)
	}
}

func TestAgentAddEventWhitelist(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	for _, typ := range []domain.EventType{domain.Zurueckgezogen, domain.Vorgemerkt, domain.KeineRueckmeldung,
		domain.AngebotAngenommen, domain.AngebotAbgelehnt, domain.ChallengeAbgegeben} {
		_, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: typ, OccurredOn: day(0)})
		var ve *domain.ValidationError
		if !errors.As(err, &ve) || ve.Field != "type" || !strings.Contains(ve.Detail, "Vorschlag") {
			t.Errorf("%s: erwartet ValidationError auf type mit Hinweis, bekommen %v", typ, err)
		}
	}
	got, _ := svc.GetApplication(ctx, app.ID)
	if got.Status != domain.Vorgemerkt || len(got.Events) != 1 {
		t.Fatalf("Ereignis trotz Ablehnung angelegt: %+v", got.Events)
	}
	if _, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0)}); err != nil {
		t.Fatalf("Beworben muss erlaubt sein: %v", err)
	}
	// Vorschläge dürfen weiterhin jeden Typ tragen.
	sg := sampleSuggestion(&app.ID)
	sg.SuggestedType = domain.Zurueckgezogen
	if _, _, err := svc.CreateSuggestion(ctx, sg); err != nil {
		t.Fatalf("Vorschlag Zurueckgezogen: %v", err)
	}
}

func TestClearGmailThread(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	if err := svc.SetGmailThread(ctx, app.ID, "thread-a"); err != nil {
		t.Fatal(err)
	}
	for range 2 { // idempotent
		if err := svc.ClearGmailThread(ctx, app.ID); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := svc.GetApplication(ctx, app.ID)
	if got.GmailThreadID != nil {
		t.Fatalf("GmailThreadID = %v", *got.GmailThreadID)
	}
	if err := svc.SetGmailThread(ctx, app.ID, "thread-b"); err != nil {
		t.Fatalf("nach dem Lösen neu setzen: %v", err)
	}
	var nf *service.NotFoundError
	if err := svc.ClearGmailThread(ctx, uuid.New()); !errors.As(err, &nf) {
		t.Errorf("erwartet NotFoundError, bekommen %v", err)
	}
}

func TestListAndDeleteProcessedMails(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	for _, id := range []string{"m1", "m2", "m3"} {
		var appID *uuid.UUID
		if id == "m2" {
			appID = &app.ID
		}
		if _, _, err := svc.MarkMailProcessed(ctx, id, appID, "erledigt "+id); err != nil {
			t.Fatal(err)
		}
	}
	list, err := svc.ListProcessedMails(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].GmailMessageID != "m3" || list[1].GmailMessageID != "m2" {
		t.Fatalf("Liste = %+v", list)
	}
	if list[1].CompanyName == nil || *list[1].CompanyName != "Acme GmbH" || list[1].PositionTitle == nil || list[0].CompanyName != nil {
		t.Errorf("Zuordnung = %+v", list)
	}
	for range 2 { // idempotent
		if err := svc.DeleteProcessedMail(ctx, "m2"); err != nil {
			t.Fatal(err)
		}
	}
	var nf *service.NotFoundError
	if _, err := svc.GetProcessedMail(ctx, "m2"); !errors.As(err, &nf) {
		t.Errorf("nach dem Löschen: %v", err)
	}
	if _, created, err := svc.MarkMailProcessed(ctx, "m2", nil, "neu bewertet"); err != nil || !created {
		t.Errorf("erneut merken: created=%v, err=%v", created, err)
	}
}

func TestMarkMailProcessedCap(t *testing.T) {
	svc := newServiceWith(t, service.WithProcessedMailCap(2))
	for _, id := range []string{"a", "b"} {
		if _, _, err := svc.MarkMailProcessed(ctx, id, nil, "ok"); err != nil {
			t.Fatal(err)
		}
	}
	var ce *service.ConflictError
	if _, _, err := svc.MarkMailProcessed(ctx, "c", nil, "ok"); !errors.As(err, &ce) {
		t.Fatalf("erwartet ConflictError, bekommen %v", err)
	}
	if _, created, err := svc.MarkMailProcessed(ctx, "a", nil, "ok"); err != nil || created {
		t.Errorf("vorhandene Mail trotz Obergrenze: created=%v, err=%v", created, err)
	}
	if service.DefaultProcessedMailCap != 200 {
		t.Errorf("Obergrenze = %d", service.DefaultProcessedMailCap)
	}
}

func TestMailboxRejectsInvisibleCharacters(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	for _, bad := range []string{"a\u200bb", "a\u202eb", "a\u2028b", "a\u2029b", "a\x07b", "a\nb", "a\tb", "a\u00adb"} {
		for field, mutate := range map[string]func(*service.NewSuggestion){
			"reason":       func(s *service.NewSuggestion) { s.Reason = bad },
			"mail_subject": func(s *service.NewSuggestion) { s.MailSubject = ptr(bad) },
			"mail_from":    func(s *service.NewSuggestion) { s.MailFrom = ptr(bad) },
			"mail_url":     func(s *service.NewSuggestion) { s.MailURL = ptr("https://mail.google.com/" + bad) },
		} {
			sg := sampleSuggestion(&app.ID)
			mutate(&sg)
			_, _, err := svc.CreateSuggestion(ctx, sg)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != field {
				t.Errorf("%s %q: erwartet ValidationError, bekommen %v", field, bad, err)
			}
		}
		_, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0), Note: ptr(bad)})
		var ve *domain.ValidationError
		if !errors.As(err, &ve) || ve.Field != "note" {
			t.Errorf("note %q: erwartet ValidationError, bekommen %v", bad, err)
		}
		_, _, err = svc.MarkMailProcessed(ctx, "m-1", nil, bad)
		if !errors.As(err, &ve) || ve.Field != "outcome" {
			t.Errorf("outcome %q: erwartet ValidationError, bekommen %v", bad, err)
		}
	}
	if _, err := svc.AgentAddEvent(ctx, app.ID, domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0), Note: ptr("Grüße – „ok“ 👍")}); err != nil {
		t.Errorf("normale Unicode-Zeichen: %v", err)
	}
}

func TestCreateSuggestionDeduplicatesByMessageID(t *testing.T) {
	svc := newService(t)
	app := agentApp(t, svc)
	in := sampleSuggestion(&app.ID)
	in.GmailMessageID = ptr("msg-1")
	first, created, err := svc.CreateSuggestion(ctx, in)
	if err != nil || !created {
		t.Fatalf("erster Vorschlag: created=%v, err=%v", created, err)
	}
	if first.GmailMessageID == nil || *first.GmailMessageID != "msg-1" {
		t.Errorf("GmailMessageID = %v", first.GmailMessageID)
	}
	in.Reason = "anderer Grund"
	again, created, err := svc.CreateSuggestion(ctx, in)
	if err != nil || created || again.ID != first.ID || again.Reason != first.Reason {
		t.Fatalf("Dublette: created=%v, err=%v, %+v", created, err, again)
	}
	// Auch nach dem Verwerfen entsteht kein neuer Vorschlag für dieselbe Mail.
	if err := svc.DismissSuggestion(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	again, created, err = svc.CreateSuggestion(ctx, in)
	if err != nil || created || again.ID != first.ID || again.State != service.SuggestionDismissed {
		t.Fatalf("nach Verwerfen: created=%v, err=%v, %+v", created, err, again)
	}
	// Ohne Message-ID gibt es keine Dublettenprüfung.
	for range 2 {
		if _, created, err := svc.CreateSuggestion(ctx, sampleSuggestion(&app.ID)); err != nil || !created {
			t.Fatalf("ohne Message-ID: created=%v, err=%v", created, err)
		}
	}
	list, _ := svc.ListOpenSuggestions(ctx)
	if len(list) != 2 {
		t.Errorf("offene Vorschläge = %d, erwartet 2", len(list))
	}
	in.GmailMessageID = ptr("a/b")
	var ve *domain.ValidationError
	if _, _, err := svc.CreateSuggestion(ctx, in); !errors.As(err, &ve) || ve.Field != "gmail_message_id" {
		t.Errorf("ungültige Message-ID: %v", err)
	}
}

func TestCreateSuggestionReportsMailFieldsInOrder(t *testing.T) {
	svc := newService(t)
	for range 20 {
		in := sampleSuggestion(nil)
		in.MailSubject = ptr("a\x07b")
		in.MailURL = ptr("https://mail.google.com/\x07")
		var ve *domain.ValidationError
		if _, _, err := svc.CreateSuggestion(ctx, in); !errors.As(err, &ve) || ve.Field != "mail_subject" {
			t.Fatalf("erwartet mail_subject, bekommen %v", err)
		}
		in.MailSubject = nil
		in.MailFrom = ptr("a\x07b")
		if _, _, err := svc.CreateSuggestion(ctx, in); !errors.As(err, &ve) || ve.Field != "mail_from" {
			t.Fatalf("erwartet mail_from, bekommen %v", err)
		}
	}
}
