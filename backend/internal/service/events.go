package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/store"
)

// AddEvent hängt ein Ereignis an, sofern der Zustandsautomat es erlaubt.
func (s *Service) AddEvent(ctx context.Context, appID uuid.UUID, next domain.NewEvent) (Event, error) {
	var created store.ApplicationEvent
	err := s.inTx(ctx, func(q *store.Queries) error {
		var err error
		created, err = s.addEvent(ctx, q, appID, next)
		return err
	})
	if err != nil {
		return Event{}, err
	}
	return toEvent(created), nil
}

// addEvent prüft und speichert ein Ereignis innerhalb einer laufenden Transaktion.
func (s *Service) addEvent(ctx context.Context, q *store.Queries, appID uuid.UUID, next domain.NewEvent) (store.ApplicationEvent, error) {
	if _, err := q.LockApplication(ctx, appID); err != nil {
		return store.ApplicationEvent{}, notFoundIfNoRows(err, "Bewerbung")
	}
	rows, err := q.ListEvents(ctx, appID)
	if err != nil {
		return store.ApplicationEvent{}, err
	}
	history := toHistory(rows)
	if err := domain.CanApply(history, next, s.today()); err != nil {
		return store.ApplicationEvent{}, err
	}
	var round *int32
	if next.Type == domain.Interview {
		r := int32(domain.InterviewRound(history))
		round = &r
	}
	created, err := q.InsertEvent(ctx, insertParams(appID, next, round))
	if err != nil {
		return store.ApplicationEvent{}, err
	}
	err = q.SetApplicationStatus(ctx, store.SetApplicationStatusParams{ID: appID, CurrentStatus: string(next.Type)})
	return created, err
}

// UndoLastEvent löscht das letzte Ereignis und setzt den Status zurück.
func (s *Service) UndoLastEvent(ctx context.Context, appID uuid.UUID) (Application, error) {
	err := s.inTx(ctx, func(q *store.Queries) error {
		if _, err := q.LockApplication(ctx, appID); err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		rows, err := q.ListEvents(ctx, appID)
		if err != nil {
			return err
		}
		if err := domain.CanUndo(toHistory(rows)); err != nil {
			return err
		}
		if err := q.DeleteEvent(ctx, rows[len(rows)-1].ID); err != nil {
			return err
		}
		return q.SetApplicationStatus(ctx, store.SetApplicationStatusParams{ID: appID, CurrentStatus: rows[len(rows)-2].Type})
	})
	if err != nil {
		return Application{}, err
	}
	return s.GetApplication(ctx, appID)
}

// AllowedEvents liefert die Ereignistypen, die als Nächstes erlaubt sind.
func (s *Service) AllowedEvents(ctx context.Context, appID uuid.UUID) ([]domain.EventType, error) {
	var rows []store.ApplicationEvent
	err := s.inReadTx(ctx, func(q *store.Queries) error {
		if _, err := q.GetApplication(ctx, appID); err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		var err error
		rows, err = q.ListEvents(ctx, appID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return domain.AllowedNext(domain.Types(toHistory(rows))), nil
}

func toHistory(rows []store.ApplicationEvent) []domain.Event {
	out := make([]domain.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDomainEvent(r.Type, r.OccurredOn, r.CreatedAt))
	}
	return out
}

// toDomainEvent übersetzt eine gespeicherte Zeile; RecordedOn ist das lokale Anlegedatum.
func toDomainEvent(eventType string, occurredOn, createdAt time.Time) domain.Event {
	return domain.Event{
		Type:       domain.EventType(eventType),
		OccurredOn: occurredOn,
		RecordedOn: domain.DateOf(createdAt.In(time.Local)),
	}
}
