package service

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/store"
)

// Zustände der Bewerbungsunterlagen (Spalte applications.documents_state).
const (
	DocsNone      = "keine"
	DocsRequested = "angefordert"
	DocsCreated   = "erstellt"
	DocsDrafted   = "entwurf_angelegt"
	DocsPortal    = "portal"
	DocsFailed    = "fehler"
)

var docsStates = []string{DocsNone, DocsRequested, DocsCreated, DocsDrafted, DocsPortal, DocsFailed}

// AgentApplication ist eine Stelle, wie sie der Agent für die Unterlagen braucht.
type AgentApplication struct {
	ID               uuid.UUID
	CompanyName      string
	CompanyWebsite   *string
	PositionTitle    string
	JobURL           *string
	Location         *string
	ContactEmail     *string
	PostingText      *string
	FitReason        *string
	DocumentsState   string
	DocumentsVersion int // 0 = noch keine Unterlagen; der Agent liefert Version DocumentsVersion+1
}

// RequestDocuments gibt eine Stelle für die Unterlagen frei (auch erneut, z. B. nach einem Fehler).
func (s *Service) RequestDocuments(ctx context.Context, id uuid.UUID) (Application, error) {
	err := s.inTx(ctx, func(q *store.Queries) error {
		if _, err := q.LockApplication(ctx, id); err != nil {
			return notFoundIfNoRows(err, "Bewerbung")
		}
		return q.SetDocumentsState(ctx, store.SetDocumentsStateParams{ID: id, DocumentsState: DocsRequested})
	})
	if err != nil {
		return Application{}, err
	}
	return s.GetApplication(ctx, id)
}

// ListAgentApplications liefert Stellen in einem Unterlagen-Zustand, älteste Änderung zuerst.
func (s *Service) ListAgentApplications(ctx context.Context, state string) ([]AgentApplication, error) {
	if !slices.Contains(docsStates, state) {
		return nil, &domain.ValidationError{Field: "documents_state", Detail: "unbekannter Zustand"}
	}
	rows, err := s.queries().ListAgentApplicationsByDocumentsState(ctx, state)
	if err != nil {
		return nil, err
	}
	out := make([]AgentApplication, 0, len(rows))
	for _, r := range rows {
		out = append(out, AgentApplication{
			ID: r.ID, CompanyName: r.CompanyName, CompanyWebsite: r.CompanyWebsite, PositionTitle: r.PositionTitle,
			JobURL: r.JobUrl, Location: r.Location, ContactEmail: r.ContactEmail, PostingText: r.PostingText,
			FitReason: r.FitReason, DocumentsState: r.DocumentsState, DocumentsVersion: int(r.DocumentsVersion),
		})
	}
	return out, nil
}
