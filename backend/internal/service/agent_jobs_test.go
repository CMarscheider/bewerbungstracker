package service_test

import (
	"errors"
	"testing"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func sampleJob() service.AgentJob {
	return service.AgentJob{
		CompanyName:   "Acme GmbH",
		PositionTitle: "Junior Frontend-Entwickler (m/w/d)",
		JobURL:        ptr("https://jobs.example.com/acme/123/"),
		Location:      ptr("Remote"),
		Source:        ptr("Arbeitnow"),
		ContactEmail:  ptr("jobs@acme.example"),
		PostingText:   ptr("Wir suchen …"),
		FitScore:      82,
		FitReason:     "Junior-Stelle, Angular und TypeScript passen.",
	}
}

func TestCreateAgentJobCreatesCompanyAndApplication(t *testing.T) {
	svc := newService(t)
	a, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	if a.CompanyName != "Acme GmbH" || a.Status != domain.Vorgemerkt || !a.CreatedByAgent {
		t.Fatalf("unerwartet: %+v", a)
	}
	if a.FitScore == nil || *a.FitScore != 82 || a.FitReason == nil || a.ContactEmail == nil || a.PostingText == nil {
		t.Fatalf("Agent-Felder fehlen: %+v", a)
	}
	if len(a.Events) != 1 || a.Events[0].Type != domain.Vorgemerkt {
		t.Fatalf("erstes Ereignis fehlt: %+v", a.Events)
	}
}

func TestCreateAgentJobReusesCompanyCaseInsensitive(t *testing.T) {
	svc := newService(t)
	first, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	other := sampleJob()
	other.CompanyName = "ACME GmbH"
	other.PositionTitle = "KI-Entwickler"
	other.JobURL = ptr("https://jobs.example.com/acme/456")
	second, err := svc.CreateAgentJob(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if second.CompanyID != first.CompanyID {
		t.Fatal("Firma wurde doppelt angelegt")
	}
}

func TestCreateAgentJobDetectsDuplicates(t *testing.T) {
	svc := newService(t)
	first, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	sameURL := sampleJob()
	sameURL.PositionTitle = "Anderer Titel"
	sameURL.JobURL = ptr("HTTPS://jobs.example.com/acme/123") // ohne Schrägstrich, andere Schreibung
	sameTitle := sampleJob()
	sameTitle.CompanyName = "acme gmbh"
	sameTitle.PositionTitle = "junior frontend-entwickler (m/w/d)"
	sameTitle.JobURL = nil
	for name, job := range map[string]service.AgentJob{"url": sameURL, "titel": sameTitle} {
		_, err := svc.CreateAgentJob(ctx, job)
		var dup *service.DuplicateError
		if !errors.As(err, &dup) || dup.ExistingID != first.ID {
			t.Fatalf("%s: erwartet DuplicateError mit %s, bekommen %v", name, first.ID, err)
		}
	}
}

func TestCreateAgentJobValidates(t *testing.T) {
	svc := newService(t)
	bad := sampleJob()
	bad.FitScore = 101
	var v *domain.ValidationError
	if _, err := svc.CreateAgentJob(ctx, bad); !errors.As(err, &v) || v.Field != "fit_score" {
		t.Fatalf("fit_score: %v", err)
	}
	bad = sampleJob()
	bad.CompanyName = "  "
	if _, err := svc.CreateAgentJob(ctx, bad); !errors.As(err, &v) || v.Field != "company_name" {
		t.Fatalf("company_name: %v", err)
	}
}

func TestManualApplicationWithSameURLConflicts(t *testing.T) {
	svc := newService(t)
	a, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateApplication(ctx, service.NewApplication{
		CompanyID: a.CompanyID, PositionTitle: "Etwas anderes", JobURL: ptr("https://jobs.example.com/acme/123"),
		FirstEvent: domain.NewEvent{Type: domain.Vorgemerkt, OccurredOn: a.Events[0].OccurredOn},
	})
	var c *service.ConflictError
	if !errors.As(err, &c) {
		t.Fatalf("erwartet ConflictError, bekommen %v", err)
	}
}

func TestListApplicationsFiltersAgentAndSortsByScore(t *testing.T) {
	svc := newService(t)
	low := sampleJob()
	low.PositionTitle, low.JobURL, low.FitScore = "Senior Frontend", ptr("https://jobs.example.com/2"), 40
	if _, err := svc.CreateAgentJob(ctx, low); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAgentJob(ctx, sampleJob()); err != nil { // Score 82
		t.Fatal(err)
	}
	fromAgent := true
	list, err := svc.ListApplications(ctx, service.ApplicationFilter{FromAgent: &fromAgent, SortByScore: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || *list[0].FitScore != 82 || *list[1].FitScore != 40 || !list[0].CreatedByAgent {
		t.Fatalf("Reihenfolge/Felder falsch: %+v", list)
	}
	manual := false
	list, err = svc.ListApplications(ctx, service.ApplicationFilter{FromAgent: &manual})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("erwartet keine manuellen, bekommen %d", len(list))
	}
}
