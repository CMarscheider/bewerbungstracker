package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/stats"
	"bewerbungsmanager/internal/store"
)

// Deadline ist eine offene Frist.
type Deadline struct {
	ApplicationID uuid.UUID
	CompanyName   string
	PositionTitle string
	EventType     domain.EventType
	DueOn         time.Time
	Overdue       bool
}

// Appointment ist ein anstehender Termin.
type Appointment struct {
	ApplicationID  uuid.UUID
	CompanyName    string
	PositionTitle  string
	EventType      domain.EventType
	Date           time.Time
	InterviewRound *int
}

func validateDays(days int) error {
	if days < 0 {
		return &domain.ValidationError{Field: "within_days", Detail: "darf nicht negativ sein"}
	}
	if days > 365 {
		return &domain.ValidationError{Field: "within_days", Detail: "darf höchstens 365 sein"}
	}
	return nil
}

// Deadlines liefert offene Fristen bis heute + withinDays, inklusive überfälliger.
func (s *Service) Deadlines(ctx context.Context, withinDays int) ([]Deadline, error) {
	if err := validateDays(withinDays); err != nil {
		return nil, err
	}
	today := s.today()
	rows, err := s.queries().ListOpenDeadlines(ctx, today.AddDate(0, 0, withinDays))
	if err != nil {
		return nil, err
	}
	out := make([]Deadline, 0, len(rows))
	for _, r := range rows {
		if r.DueOn == nil {
			continue
		}
		out = append(out, Deadline{
			ApplicationID: r.ApplicationID, CompanyName: r.CompanyName, PositionTitle: r.PositionTitle,
			EventType: domain.EventType(r.EventType), DueOn: *r.DueOn, Overdue: r.DueOn.Before(today),
		})
	}
	return out, nil
}

// Appointments liefert geplante Termine von heute bis heute + withinDays.
func (s *Service) Appointments(ctx context.Context, withinDays int) ([]Appointment, error) {
	if err := validateDays(withinDays); err != nil {
		return nil, err
	}
	today := s.today()
	rows, err := s.queries().ListUpcomingAppointments(ctx, store.ListUpcomingAppointmentsParams{
		FromDate: today, UntilDate: today.AddDate(0, 0, withinDays),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Appointment, 0, len(rows))
	for _, r := range rows {
		var round *int
		if r.InterviewRound != nil {
			n := int(*r.InterviewRound)
			round = &n
		}
		out = append(out, Appointment{
			ApplicationID: r.ApplicationID, CompanyName: r.CompanyName, PositionTitle: r.PositionTitle,
			EventType: domain.EventType(r.EventType), Date: r.OccurredOn, InterviewRound: round,
		})
	}
	return out, nil
}

func (s *Service) Funnel(ctx context.Context) ([]stats.FunnelStep, error) {
	apps, err := s.statsInput(ctx)
	if err != nil {
		return nil, err
	}
	return stats.Funnel(apps), nil
}

func (s *Service) Summary(ctx context.Context) (stats.Summary, error) {
	apps, err := s.statsInput(ctx)
	if err != nil {
		return stats.Summary{}, err
	}
	return stats.Summarize(apps), nil
}

// statsInput lädt alle Verläufe; die Zeilen kommen nach Bewerbung gruppiert.
func (s *Service) statsInput(ctx context.Context) ([]stats.Application, error) {
	rows, err := s.queries().ListAllEventsWithSource(ctx)
	if err != nil {
		return nil, err
	}
	var apps []stats.Application
	var current uuid.UUID
	for _, r := range rows {
		if len(apps) == 0 || r.ApplicationID != current {
			apps = append(apps, stats.Application{Source: r.Source})
			current = r.ApplicationID
		}
		last := &apps[len(apps)-1]
		last.Events = append(last.Events, toDomainEvent(r.Type, r.OccurredOn, r.CreatedAt))
	}
	return apps, nil
}
