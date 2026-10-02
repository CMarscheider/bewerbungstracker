package service_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func newApp(companyID uuid.UUID, title string, first domain.NewEvent) service.NewApplication {
	return service.NewApplication{CompanyID: companyID, PositionTitle: title, FirstEvent: first}
}

func TestCreateApplicationWithFirstEvent(t *testing.T) {
	svc := newService(t)
	c := mustCompany(t, svc, "Acme")
	app, err := svc.CreateApplication(ctx, newApp(c.ID, "Go-Entwickler", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-3)}))
	if err != nil {
		t.Fatal(err)
	}
	if app.Status != domain.Beworben || app.Phase != domain.PhaseAktiv || app.CompanyName != "Acme" {
		t.Errorf("Bewerbung = %+v", app)
	}
	if len(app.Events) != 1 || !app.Events[0].OccurredOn.Equal(day(-3)) {
		t.Errorf("Ereignisse = %+v", app.Events)
	}
}

func TestCreateApplicationRejectsInvalidFirstEvent(t *testing.T) {
	svc := newService(t)
	c := mustCompany(t, svc, "Acme")
	_, err := svc.CreateApplication(ctx, newApp(c.ID, "Go-Entwickler", domain.NewEvent{Type: domain.Interview, OccurredOn: day(0)}))
	var te *domain.TransitionError
	if !errors.As(err, &te) {
		t.Fatalf("erwartet TransitionError, bekommen %v", err)
	}
	list, _ := svc.ListApplications(ctx, service.ApplicationFilter{})
	if len(list) != 0 {
		t.Errorf("es darf nichts angelegt worden sein: %+v", list)
	}
}

func TestCreateApplicationUnknownCompany(t *testing.T) {
	svc := newService(t)
	_, err := svc.CreateApplication(ctx, newApp(uuid.New(), "Go-Entwickler", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0)}))
	var re *domain.RuleError
	if !errors.As(err, &re) || re.Code != service.CodeUnknownCompany {
		t.Fatalf("erwartet RuleError %s, bekommen %v", service.CodeUnknownCompany, err)
	}
}

func TestCreateApplicationRequiresTitle(t *testing.T) {
	svc := newService(t)
	c := mustCompany(t, svc, "Acme")
	_, err := svc.CreateApplication(ctx, newApp(c.ID, " ", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0)}))
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "position_title" {
		t.Fatalf("erwartet ValidationError(position_title), bekommen %v", err)
	}
}

func TestListApplicationsFilters(t *testing.T) {
	svc := newService(t)
	alpha := mustCompany(t, svc, "Alpha")
	beta := mustCompany(t, svc, "Beta")
	planned, err := svc.CreateApplication(ctx, newApp(alpha.ID, "Backend-Entwickler",
		domain.NewEvent{Type: domain.Vorgemerkt, OccurredOn: day(0), DueOn: ptr(day(10))}))
	if err != nil {
		t.Fatal(err)
	}
	applied, err := svc.CreateApplication(ctx, newApp(beta.ID, "Go Developer",
		domain.NewEvent{Type: domain.Beworben, OccurredOn: day(-1)}))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		filter service.ApplicationFilter
		want   []uuid.UUID
	}{
		{"Phase Vorbereitung", service.ApplicationFilter{Phase: ptr(domain.PhaseVorbereitung)}, []uuid.UUID{planned.ID}},
		{"Status Beworben", service.ApplicationFilter{Status: ptr(domain.Beworben)}, []uuid.UUID{applied.ID}},
		{"Suche Stelle", service.ApplicationFilter{Query: ptr("go dev")}, []uuid.UUID{applied.ID}},
		{"Suche Firma", service.ApplicationFilter{Query: ptr("alpha")}, []uuid.UUID{planned.ID}},
		{"Phase und Status widersprüchlich", service.ApplicationFilter{Phase: ptr(domain.PhaseAktiv), Status: ptr(domain.Vorgemerkt)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, err := svc.ListApplications(ctx, tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			var got []uuid.UUID
			for _, a := range list {
				got = append(got, a.ID)
			}
			if len(got) != len(tt.want) || (len(got) > 0 && got[0] != tt.want[0]) {
				t.Errorf("IDs = %v, erwartet %v", got, tt.want)
			}
		})
	}

	all, err := svc.ListApplications(ctx, service.ApplicationFilter{Phase: ptr(domain.PhaseVorbereitung)})
	if err != nil {
		t.Fatal(err)
	}
	if all[0].OpenDueOn == nil || !all[0].OpenDueOn.Equal(day(10)) || !all[0].LastEventOn.Equal(day(0)) {
		t.Errorf("Frist/letztes Ereignis falsch: %+v", all[0])
	}
}

func TestUpdateApplicationPatch(t *testing.T) {
	svc := newService(t)
	c := mustCompany(t, svc, "Acme")
	app, err := svc.CreateApplication(ctx, service.NewApplication{
		CompanyID: c.ID, PositionTitle: "Alt", Location: ptr("Berlin"),
		FirstEvent: domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0)},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateApplication(ctx, app.ID, service.ApplicationPatch{
		PositionTitle: ptr("Neu"), Location: ptr(""), Source: ptr("LinkedIn"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.PositionTitle != "Neu" || updated.Location != nil || updated.Source == nil || *updated.Source != "LinkedIn" {
		t.Errorf("UpdateApplication = %+v", updated)
	}
	if updated.Status != domain.Beworben {
		t.Errorf("Status darf sich durch Patch nicht ändern: %s", updated.Status)
	}
}

func TestDeleteApplicationAndCompanyConflict(t *testing.T) {
	svc := newService(t)
	c := mustCompany(t, svc, "Acme")
	app, err := svc.CreateApplication(ctx, newApp(c.ID, "Go", domain.NewEvent{Type: domain.Beworben, OccurredOn: day(0)}))
	if err != nil {
		t.Fatal(err)
	}

	var ce *service.ConflictError
	if err := svc.DeleteCompany(ctx, c.ID); !errors.As(err, &ce) {
		t.Fatalf("Firma mit Bewerbung löschen: erwartet ConflictError, bekommen %v", err)
	}

	if err := svc.DeleteApplication(ctx, app.ID); err != nil {
		t.Fatal(err)
	}
	var nf *service.NotFoundError
	if _, err := svc.GetApplication(ctx, app.ID); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
	if err := svc.DeleteCompany(ctx, c.ID); err != nil {
		t.Fatalf("Firma ohne Bewerbungen löschen: %v", err)
	}
}
