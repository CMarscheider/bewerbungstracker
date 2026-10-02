package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/store"
)

// Company ist eine Firma inklusive Anzahl ihrer Bewerbungen.
type Company struct {
	ID               uuid.UUID
	Name             string
	Website          *string
	Notes            *string
	CreatedAt        time.Time
	ApplicationCount int
}

// CompanyInput sind die Daten zum Anlegen einer Firma.
type CompanyInput struct {
	Name    string
	Website *string
	Notes   *string
}

// CompanyPatch ändert nur gesetzte Felder; "" löscht optionale Felder.
type CompanyPatch struct {
	Name    *string
	Website *string
	Notes   *string
}

func (s *Service) ListCompanies(ctx context.Context) ([]Company, error) {
	rows, err := s.queries().ListCompanies(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Company, 0, len(rows))
	for _, r := range rows {
		out = append(out, Company{
			ID: r.ID, Name: r.Name, Website: r.Website, Notes: r.Notes,
			CreatedAt: r.CreatedAt, ApplicationCount: int(r.ApplicationCount),
		})
	}
	return out, nil
}

func (s *Service) GetCompany(ctx context.Context, id uuid.UUID) (Company, error) {
	r, err := s.queries().GetCompany(ctx, id)
	if err != nil {
		return Company{}, notFoundIfNoRows(err, "Firma")
	}
	return Company{
		ID: r.ID, Name: r.Name, Website: r.Website, Notes: r.Notes,
		CreatedAt: r.CreatedAt, ApplicationCount: int(r.ApplicationCount),
	}, nil
}

func (s *Service) CreateCompany(ctx context.Context, in CompanyInput) (Company, error) {
	name, err := requireText("name", in.Name)
	if err != nil {
		return Company{}, err
	}
	c, err := s.queries().CreateCompany(ctx, store.CreateCompanyParams{
		Name: name, Website: cleanOptional(in.Website), Notes: cleanOptional(in.Notes),
	})
	if isUniqueViolation(err) {
		return Company{}, &ConflictError{Detail: fmt.Sprintf("Firma %q existiert bereits", name)}
	}
	if err != nil {
		return Company{}, err
	}
	return Company{ID: c.ID, Name: c.Name, Website: c.Website, Notes: c.Notes, CreatedAt: c.CreatedAt}, nil
}

func (s *Service) UpdateCompany(ctx context.Context, id uuid.UUID, p CompanyPatch) (Company, error) {
	current, err := s.GetCompany(ctx, id)
	if err != nil {
		return Company{}, err
	}
	name := current.Name
	if p.Name != nil {
		if name, err = requireText("name", *p.Name); err != nil {
			return Company{}, err
		}
	}
	c, err := s.queries().UpdateCompany(ctx, store.UpdateCompanyParams{
		ID: id, Name: name,
		Website: applyOptional(current.Website, p.Website),
		Notes:   applyOptional(current.Notes, p.Notes),
	})
	if isUniqueViolation(err) {
		return Company{}, &ConflictError{Detail: fmt.Sprintf("Firma %q existiert bereits", name)}
	}
	if err != nil {
		return Company{}, notFoundIfNoRows(err, "Firma")
	}
	return Company{
		ID: c.ID, Name: c.Name, Website: c.Website, Notes: c.Notes,
		CreatedAt: c.CreatedAt, ApplicationCount: current.ApplicationCount,
	}, nil
}

func (s *Service) DeleteCompany(ctx context.Context, id uuid.UUID) error {
	current, err := s.GetCompany(ctx, id)
	if err != nil {
		return err
	}
	if current.ApplicationCount > 0 {
		return &ConflictError{Detail: fmt.Sprintf("Firma %q hat noch %d Bewerbung(en)", current.Name, current.ApplicationCount)}
	}
	if _, err := s.queries().DeleteCompany(ctx, id); err != nil {
		if isForeignKeyViolation(err) {
			return &ConflictError{Detail: fmt.Sprintf("Firma %q hat noch Bewerbungen", current.Name)}
		}
		return err
	}
	return nil
}
