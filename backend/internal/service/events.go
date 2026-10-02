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
		if _, err := q.LockApplication(ctx, appID); err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		rows, err := q.ListEvents(ctx, appID)
		if err != nil {
			return err
		}
		history := toHistory(rows)
		if err := domain.CanApply(history, next, s.today()); err != nil {
			return err
		}
		var round *int32
		if next.Type == domain.Interview {
			r := int32(domain.InterviewRound(history))
			round = &r
		}
		if created, err = q.InsertEvent(ctx, insertParams(appID, next, round)); err != nil {
			return err
		}
		return q.SetApplicationStatus(ctx, store.SetApplicationStatusParams{ID: appID, CurrentStatus: string(next.Type)})
	})
	if err != nil {
		return Event{}, err
	}
	return toEvent(created), nil
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
	q := s.queries()
	if _, err := q.GetApplication(ctx, appID); err != nil {
		return nil, notFoundIfNoRows(err, "Bewerbung")
	}
	rows, err := q.ListEvents(ctx, appID)
	if err != nil {
		return nil, err
	}
	return domain.AllowedNext(domain.Types(toHistory(rows))), nil
}

func toHistory(rows []store.ApplicationEvent) []domain.Event {
	out := make([]domain.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Event{
			Type:       domain.EventType(r.Type),
			OccurredOn: r.OccurredOn,
			RecordedOn: domain.DateOf(r.CreatedAt.In(time.Local)),
		})
	}
	return out
}
