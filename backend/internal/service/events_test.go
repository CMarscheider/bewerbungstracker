package service_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

// appliedApp legt eine Bewerbung mit "Beworben" vor 10 Tagen an.
func appliedApp(t *testing.T, svc *service.Service) service.Application {
	t.Helper()
	c := mustCompany(t, svc, "Acme")
	app, err := svc.CreateApplication(ctx, newApp(c.ID, "Go", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-10)}))
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func mustAdd(t *testing.T, svc *service.Service, id uuid.UUID, next domain.NewEvent) service.Event {
	t.Helper()
	e, err := svc.AddEvent(ctx, id, next)
	if err != nil {
		t.Fatalf("AddEvent(%s): %v", next.Type, err)
	}
	return e
}

func TestAddEventUpdatesStatus(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	e := mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.ChallengeErhalten, OccurredOn: day(-5), DueOn: ptr(day(3)), Note: ptr("Kata")})
	if e.DueOn == nil || !e.DueOn.Equal(day(3)) || e.Note == nil {
		t.Errorf("Ereignis = %+v", e)
	}
	got, err := svc.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.ChallengeErhalten || len(got.Events) != 2 {
		t.Errorf("Status=%s Ereignisse=%d", got.Status, len(got.Events))
	}
}

func TestAddEventAssignsInterviewRounds(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	first := mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(-5)})
	second := mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(2)})
	if first.InterviewRound == nil || *first.InterviewRound != 1 || second.InterviewRound == nil || *second.InterviewRound != 2 {
		t.Errorf("Runden = %v, %v", first.InterviewRound, second.InterviewRound)
	}
}

func TestAddEventRejectsInvalidTransition(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.Absage, OccurredOn: day(-1)})
	_, err := svc.AddEvent(ctx, app.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(0)})
	var te *domain.TransitionError
	if !errors.As(err, &te) || te.From != domain.Absage {
		t.Fatalf("erwartet TransitionError von Absage, bekommen %v", err)
	}
	got, _ := svc.GetApplication(ctx, app.ID)
	if got.Status != domain.Absage || len(got.Events) != 2 {
		t.Errorf("Zustand verändert: Status=%s Ereignisse=%d", got.Status, len(got.Events))
	}
}

func TestAddEventUnknownApplication(t *testing.T) {
	svc := newService(t)
	_, err := svc.AddEvent(ctx, uuid.New(), domain.NewEvent{Type: domain.Absage, OccurredOn: day(0)})
	var nf *service.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
}

func TestKeineRueckmeldungCanBeReopened(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.KeineRueckmeldung, OccurredOn: day(-2)})
	mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(1)})
	got, _ := svc.GetApplication(ctx, app.ID)
	if got.Status != domain.Interview {
		t.Errorf("Status = %s, erwartet Interview", got.Status)
	}
}

func TestUndoLastEvent(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.Absage, OccurredOn: day(-1)})

	got, err := svc.UndoLastEvent(ctx, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.Beworben || len(got.Events) != 1 {
		t.Errorf("nach Rückgängig: Status=%s Ereignisse=%d", got.Status, len(got.Events))
	}

	_, err = svc.UndoLastEvent(ctx, app.ID)
	var re *domain.RuleError
	if !errors.As(err, &re) || re.Code != domain.CodeCannotUndoFirst {
		t.Fatalf("erwartet RuleError %s, bekommen %v", domain.CodeCannotUndoFirst, err)
	}
}

func TestAllowedEvents(t *testing.T) {
	svc := newService(t)
	app := appliedApp(t, svc)
	mustAdd(t, svc, app.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(-1)})
	got, err := svc.AllowedEvents(ctx, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.AllowedNext([]domain.EventType{domain.Beworben, domain.Interview})
	if !slices.Equal(got, want) {
		t.Errorf("AllowedEvents = %v, erwartet %v", got, want)
	}

	var nf *service.NotFoundError
	if _, err := svc.AllowedEvents(ctx, uuid.New()); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
}
