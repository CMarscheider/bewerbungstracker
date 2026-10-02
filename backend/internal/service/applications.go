package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/store"
)

// CodeUnknownCompany: referenzierte Firma existiert nicht.
const CodeUnknownCompany = "unknown-company"

var errUnknownCompany = &domain.RuleError{Code: CodeUnknownCompany, Detail: "Die angegebene Firma existiert nicht"}

// Event ist ein gespeichertes Ereignis.
type Event struct {
	ID             uuid.UUID
	Type           domain.EventType
	OccurredOn     time.Time
	DueOn          *time.Time
	InterviewRound *int
	Note           *string
	CreatedAt      time.Time
}

// Application ist eine Bewerbung mit vollständigem Verlauf.
type Application struct {
	ID            uuid.UUID
	CompanyID     uuid.UUID
	CompanyName   string
	PositionTitle string
	JobURL        *string
	Location      *string
	Source        *string
	Notes         *string
	Status        domain.EventType
	Phase         domain.Phase
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Events        []Event
}

// ApplicationSummary ist eine Zeile der Bewerbungsliste.
type ApplicationSummary struct {
	ID            uuid.UUID
	CompanyID     uuid.UUID
	CompanyName   string
	PositionTitle string
	Status        domain.EventType
	Phase         domain.Phase
	UpdatedAt     time.Time
	LastEventOn   time.Time
	OpenDueOn     *time.Time // Frist des letzten Ereignisses, falls vorhanden
}

// NewApplication sind die Daten zum Anlegen inklusive erstem Ereignis.
type NewApplication struct {
	CompanyID     uuid.UUID
	PositionTitle string
	JobURL        *string
	Location      *string
	Source        *string
	Notes         *string
	FirstEvent    domain.NewEvent
}

// ApplicationPatch ändert nur Stammdaten; nil = unverändert, "" = löschen.
type ApplicationPatch struct {
	CompanyID     *uuid.UUID
	PositionTitle *string
	JobURL        *string
	Location      *string
	Source        *string
	Notes         *string
}

// ApplicationFilter filtert die Liste; alle Felder optional.
type ApplicationFilter struct {
	Phase  *domain.Phase
	Status *domain.EventType
	Query  *string
}

func (s *Service) CreateApplication(ctx context.Context, in NewApplication) (Application, error) {
	title, err := requireText("position_title", in.PositionTitle)
	if err != nil {
		return Application{}, err
	}
	if err := domain.CanApply(nil, in.FirstEvent, s.today()); err != nil {
		return Application{}, err
	}
	var id uuid.UUID
	err = s.inTx(ctx, func(q *store.Queries) error {
		app, err := q.CreateApplication(ctx, store.CreateApplicationParams{
			CompanyID:     in.CompanyID,
			PositionTitle: title,
			JobUrl:        cleanOptional(in.JobURL),
			Location:      cleanOptional(in.Location),
			Source:        cleanOptional(in.Source),
			Notes:         cleanOptional(in.Notes),
			CurrentStatus: string(in.FirstEvent.Type),
		})
		if isForeignKeyViolation(err) {
			return errUnknownCompany
		}
		if err != nil {
			return err
		}
		id = app.ID
		_, err = q.InsertEvent(ctx, insertParams(app.ID, in.FirstEvent, nil))
		return err
	})
	if err != nil {
		return Application{}, err
	}
	return s.GetApplication(ctx, id)
}

func (s *Service) GetApplication(ctx context.Context, id uuid.UUID) (Application, error) {
	q := s.queries()
	r, err := q.GetApplication(ctx, id)
	if err != nil {
		return Application{}, notFoundIfNoRows(err, "Bewerbung")
	}
	rows, err := q.ListEvents(ctx, id)
	if err != nil {
		return Application{}, err
	}
	events := make([]Event, 0, len(rows))
	for _, e := range rows {
		events = append(events, toEvent(e))
	}
	status := domain.EventType(r.CurrentStatus)
	return Application{
		ID: r.ID, CompanyID: r.CompanyID, CompanyName: r.CompanyName, PositionTitle: r.PositionTitle,
		JobURL: r.JobUrl, Location: r.Location, Source: r.Source, Notes: r.Notes,
		Status: status, Phase: status.Phase(),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Events: events,
	}, nil
}

func (s *Service) ListApplications(ctx context.Context, f ApplicationFilter) ([]ApplicationSummary, error) {
	statuses, err := statusFilter(f)
	if err != nil {
		return nil, err
	}
	if statuses != nil && len(statuses) == 0 {
		return []ApplicationSummary{}, nil
	}
	rows, err := s.queries().ListApplications(ctx, store.ListApplicationsParams{
		Statuses: statuses,
		Query:    cleanOptional(f.Query),
	})
	if err != nil {
		return nil, err
	}
	out := make([]ApplicationSummary, 0, len(rows))
	for _, r := range rows {
		status := domain.EventType(r.CurrentStatus)
		out = append(out, ApplicationSummary{
			ID: r.ID, CompanyID: r.CompanyID, CompanyName: r.CompanyName, PositionTitle: r.PositionTitle,
			Status: status, Phase: status.Phase(), UpdatedAt: r.UpdatedAt,
			LastEventOn: r.LastEventOn, OpenDueOn: r.OpenDueOn,
		})
	}
	return out, nil
}

// statusFilter übersetzt Phase/Status in eine Statusliste; nil = kein Filter, leer = nichts passt.
func statusFilter(f ApplicationFilter) ([]string, error) {
	if f.Status != nil && !f.Status.Valid() {
		return nil, &domain.ValidationError{Field: "status", Detail: "unbekannter Status"}
	}
	switch {
	case f.Phase != nil:
		out := []string{}
		for _, t := range domain.StatusesInPhase(*f.Phase) {
			if f.Status == nil || *f.Status == t {
				out = append(out, string(t))
			}
		}
		return out, nil
	case f.Status != nil:
		return []string{string(*f.Status)}, nil
	}
	return nil, nil
}

func (s *Service) UpdateApplication(ctx context.Context, id uuid.UUID, p ApplicationPatch) (Application, error) {
	err := s.inTx(ctx, func(q *store.Queries) error {
		cur, err := q.LockApplication(ctx, id)
		if err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		params := store.UpdateApplicationParams{
			ID:            id,
			CompanyID:     cur.CompanyID,
			PositionTitle: cur.PositionTitle,
			JobUrl:        applyOptional(cur.JobUrl, p.JobURL),
			Location:      applyOptional(cur.Location, p.Location),
			Source:        applyOptional(cur.Source, p.Source),
			Notes:         applyOptional(cur.Notes, p.Notes),
		}
		if p.CompanyID != nil {
			params.CompanyID = *p.CompanyID
		}
		if p.PositionTitle != nil {
			if params.PositionTitle, err = requireText("position_title", *p.PositionTitle); err != nil {
				return err
			}
		}
		_, err = q.UpdateApplication(ctx, params)
		if isForeignKeyViolation(err) {
			return errUnknownCompany
		}
		return err
	})
	if err != nil {
		return Application{}, err
	}
	return s.GetApplication(ctx, id)
}

func (s *Service) DeleteApplication(ctx context.Context, id uuid.UUID) error {
	n, err := s.queries().DeleteApplication(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return &NotFoundError{Resource: "Bewerbung"}
	}
	return nil
}

func insertParams(appID uuid.UUID, e domain.NewEvent, round *int32) store.InsertEventParams {
	var due *time.Time
	if e.DueOn != nil {
		d := domain.DateOf(*e.DueOn)
		due = &d
	}
	return store.InsertEventParams{
		ApplicationID:  appID,
		Type:           string(e.Type),
		OccurredOn:     domain.DateOf(e.OccurredOn),
		DueOn:          due,
		InterviewRound: round,
		Note:           cleanOptional(e.Note),
	}
}

func toEvent(e store.ApplicationEvent) Event {
	var round *int
	if e.InterviewRound != nil {
		r := int(*e.InterviewRound)
		round = &r
	}
	return Event{
		ID: e.ID, Type: domain.EventType(e.Type), OccurredOn: e.OccurredOn, DueOn: e.DueOn,
		InterviewRound: round, Note: e.Note, CreatedAt: e.CreatedAt,
	}
}
