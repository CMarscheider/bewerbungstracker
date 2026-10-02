package service_test

import (
	"errors"
	"testing"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func createApp(t *testing.T, svc *service.Service, company, title string, first domain.NewEvent) service.Application {
	t.Helper()
	c := mustCompany(t, svc, company)
	app, err := svc.CreateApplication(ctx, newApp(c.ID, title, first))
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestDeadlines(t *testing.T) {
	svc := newService(t)
	overdue := createApp(t, svc, "A", "Überfällig", domain.NewEvent{Type: domain.Vorgemerkt, OccurredOn: day(-5), DueOn: ptr(day(-1))})
	challenge := createApp(t, svc, "B", "Challenge", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-5)})
	mustAdd(t, svc, challenge.ID, domain.NewEvent{Type: domain.ChallengeErhalten, OccurredOn: day(-1), DueOn: ptr(day(3))})
	done := createApp(t, svc, "C", "Erledigt", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-5)})
	mustAdd(t, svc, done.ID, domain.NewEvent{Type: domain.ChallengeErhalten, OccurredOn: day(-4), DueOn: ptr(day(3))})
	mustAdd(t, svc, done.ID, domain.NewEvent{Type: domain.ChallengeAbgegeben, OccurredOn: day(-1)})
	later := createApp(t, svc, "D", "Später", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-5)})
	mustAdd(t, svc, later.ID, domain.NewEvent{Type: domain.AngebotErhalten, OccurredOn: day(0), DueOn: ptr(day(20))})

	got, err := svc.Deadlines(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("erwartet 2 Fristen, bekommen %+v", got)
	}
	if got[0].ApplicationID != overdue.ID || !got[0].Overdue || got[0].EventType != domain.Vorgemerkt {
		t.Errorf("erste Frist = %+v", got[0])
	}
	if got[1].ApplicationID != challenge.ID || got[1].Overdue || !got[1].DueOn.Equal(day(3)) {
		t.Errorf("zweite Frist = %+v", got[1])
	}

	var ve *domain.ValidationError
	if _, err := svc.Deadlines(ctx, -1); !errors.As(err, &ve) {
		t.Fatalf("negative Tage: erwartet ValidationError, bekommen %v", err)
	}
}

func TestAppointments(t *testing.T) {
	svc := newService(t)
	upcoming := createApp(t, svc, "A", "Bald", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-10)})
	mustAdd(t, svc, upcoming.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(2)})
	past := createApp(t, svc, "B", "Vorbei", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-10)})
	mustAdd(t, svc, past.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(-3)})
	cancelled := createApp(t, svc, "C", "Abgesagt", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-10)})
	mustAdd(t, svc, cancelled.ID, domain.NewEvent{Type: domain.Interview, OccurredOn: day(2)})
	mustAdd(t, svc, cancelled.ID, domain.NewEvent{Type: domain.Absage, OccurredOn: day(0)})

	got, err := svc.Appointments(ctx, 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ApplicationID != upcoming.ID {
		t.Fatalf("Termine = %+v", got)
	}
	if got[0].InterviewRound == nil || *got[0].InterviewRound != 1 || !got[0].Date.Equal(day(2)) {
		t.Errorf("Termin = %+v", got[0])
	}
}

func TestStatsUseStoredEvents(t *testing.T) {
	svc := newService(t)
	rejected := createApp(t, svc, "A", "Abgelehnt", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-5)})
	mustAdd(t, svc, rejected.ID, domain.NewEvent{Type: domain.Absage, OccurredOn: day(0)})
	createApp(t, svc, "B", "Nur gemerkt", domain.NewEvent{Type: domain.Vorgemerkt, OccurredOn: day(0)})

	funnel, err := svc.Funnel(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(funnel) != 7 || funnel[0].Reached != 1 {
		t.Errorf("Funnel = %+v", funnel)
	}
	summary, err := svc.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Applied != 1 || summary.Responded != 1 || summary.MedianDaysToResponse == nil || *summary.MedianDaysToResponse != 5 {
		t.Errorf("Summary = %+v", summary)
	}
}
