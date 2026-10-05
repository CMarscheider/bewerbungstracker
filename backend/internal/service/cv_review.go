package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"bewerbungsmanager/internal/store"
)

// Zustände einer Lebenslauf-Optimierung.
const (
	CVReviewRequested = "angefordert"
	CVReviewReady     = "fertig"
	CVReviewClosed    = "abgeschlossen"
)

// CVReview ist ein Optimierungslauf; Proposal ist nil, solange er angefordert ist.
type CVReview struct {
	ID               uuid.UUID
	State            string
	BasedOnUpdatedAt time.Time
	Proposal         json.RawMessage
	Notes            []string
	RequestedAt      time.Time
	CompletedAt      *time.Time
}

func toCVReview(r store.CvReview) (CVReview, error) {
	notes := []string{}
	if len(r.Notes) > 0 {
		if err := json.Unmarshal(r.Notes, &notes); err != nil {
			return CVReview{}, fmt.Errorf("hinweise lesen: %w", err)
		}
	}
	return CVReview{
		ID: r.ID, State: r.State, BasedOnUpdatedAt: r.BasedOnUpdatedAt, Proposal: r.Proposal,
		Notes: notes, RequestedAt: r.RequestedAt, CompletedAt: r.CompletedAt,
	}, nil
}

// RequestCVReview fordert eine Optimierung des gespeicherten Lebenslaufs an.
func (s *Service) RequestCVReview(ctx context.Context) (CVReview, error) {
	cv, err := s.GetCV(ctx)
	if err != nil {
		return CVReview{}, err
	}
	if cv.Data == nil {
		return CVReview{}, &ConflictError{Detail: "Bitte zuerst den Lebenslauf speichern"}
	}
	r, err := s.queries().CreateCVReview(ctx, cv.UpdatedAt)
	if isUniqueViolation(err) {
		return CVReview{}, &ConflictError{Detail: "Es läuft bereits eine Optimierung"}
	}
	if err != nil {
		return CVReview{}, err
	}
	return toCVReview(r)
}

// OpenCVReview liefert den offenen Lauf (angefordert oder fertig).
func (s *Service) OpenCVReview(ctx context.Context) (CVReview, error) {
	r, err := s.queries().GetOpenCVReview(ctx)
	if err != nil {
		return CVReview{}, notFoundIfNoRows(err, "Optimierung")
	}
	return toCVReview(r)
}

// CloseCVReview schließt den offenen Lauf ab oder zieht die Anfrage zurück; ohne offenen Lauf passiert nichts.
func (s *Service) CloseCVReview(ctx context.Context) error {
	return s.queries().CloseOpenCVReviews(ctx)
}

// ListCVReviews liefert alle Läufe in einem Zustand (für den Agenten).
func (s *Service) ListCVReviews(ctx context.Context, state string) ([]CVReview, error) {
	rows, err := s.queries().ListCVReviewsByState(ctx, state)
	if err != nil {
		return nil, err
	}
	out := make([]CVReview, 0, len(rows))
	for _, r := range rows {
		rv, err := toCVReview(r)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, nil
}

// CompleteCVReview speichert den Vorschlag des Agenten; nur für angeforderte Läufe.
func (s *Service) CompleteCVReview(ctx context.Context, id uuid.UUID, proposal json.RawMessage, notes []string) (CVReview, error) {
	if err := validateCVData(proposal); err != nil {
		return CVReview{}, err
	}
	if notes == nil {
		notes = []string{}
	}
	notesJSON, err := json.Marshal(notes)
	if err != nil {
		return CVReview{}, err
	}
	r, err := s.queries().CompleteCVReview(ctx, store.CompleteCVReviewParams{ID: id, Proposal: proposal, Notes: notesJSON})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, gerr := s.queries().GetCVReview(ctx, id); gerr != nil {
			return CVReview{}, notFoundIfNoRows(gerr, "Optimierung")
		}
		return CVReview{}, &ConflictError{Detail: "Die Optimierung ist nicht (mehr) angefordert"}
	}
	if err != nil {
		return CVReview{}, err
	}
	return toCVReview(r)
}
