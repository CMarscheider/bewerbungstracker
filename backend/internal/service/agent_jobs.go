package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/store"
)

// AgentJob ist eine vom Agenten gefundene Stelle.
type AgentJob struct {
	CompanyName    string
	CompanyWebsite *string
	PositionTitle  string
	JobURL         *string
	Location       *string
	Source         *string
	ContactEmail   *string
	PostingText    *string
	FitScore       int
	FitReason      string
}

// CreateAgentJob legt eine gefundene Stelle als Vorgemerkt an. Die Firma wird gesucht (ohne
// Groß-/Kleinschreibung) oder angelegt; gibt es die Stelle schon, kommt DuplicateError.
func (s *Service) CreateAgentJob(ctx context.Context, in AgentJob) (Application, error) {
	company, err := requireText("company_name", in.CompanyName)
	if err != nil {
		return Application{}, err
	}
	title, err := requireText("position_title", in.PositionTitle)
	if err != nil {
		return Application{}, err
	}
	reason, err := requireText("fit_reason", in.FitReason)
	if err != nil {
		return Application{}, err
	}
	if in.FitScore < 0 || in.FitScore > 100 {
		return Application{}, &domain.ValidationError{Field: "fit_score", Detail: "muss zwischen 0 und 100 liegen"}
	}
	first := domain.NewEvent{Type: domain.Vorgemerkt, OccurredOn: s.today()}
	if err := domain.CanApply(nil, first, s.today()); err != nil {
		return Application{}, err
	}
	jobURL := cleanOptional(in.JobURL)
	score := int32(in.FitScore)

	var id uuid.UUID
	err = s.inTx(ctx, func(q *store.Queries) error {
		dup, err := q.FindDuplicateApplication(ctx, store.FindDuplicateApplicationParams{
			JobUrl: jobURL, CompanyName: company, PositionTitle: title,
		})
		if err == nil {
			return &DuplicateError{ExistingID: dup}
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		c, err := q.GetCompanyByName(ctx, company)
		if errors.Is(err, pgx.ErrNoRows) {
			c, err = q.CreateCompany(ctx, store.CreateCompanyParams{Name: company, Website: cleanOptional(in.CompanyWebsite)})
		}
		if err != nil {
			return err
		}
		app, err := q.CreateApplication(ctx, store.CreateApplicationParams{
			CompanyID:      c.ID,
			PositionTitle:  title,
			JobUrl:         jobURL,
			Location:       cleanOptional(in.Location),
			Source:         cleanOptional(in.Source),
			CurrentStatus:  string(first.Type),
			ContactEmail:   cleanOptional(in.ContactEmail),
			PostingText:    cleanOptional(in.PostingText),
			FitScore:       &score,
			FitReason:      &reason,
			CreatedByAgent: true,
		})
		if isUniqueViolation(err) {
			return errDuplicateURL
		}
		if err != nil {
			return err
		}
		id = app.ID
		_, err = q.InsertEvent(ctx, insertParams(app.ID, first, nil))
		return err
	})
	if err != nil {
		return Application{}, err
	}
	return s.GetApplication(ctx, id)
}
