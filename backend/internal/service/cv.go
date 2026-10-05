package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"bewerbungsmanager/internal/domain"
)

// CV ist der gespeicherte Lebenslauf; Data ist nil, solange keiner gespeichert wurde.
type CV struct {
	Data      json.RawMessage
	UpdatedAt time.Time
}

func (s *Service) GetCV(ctx context.Context) (CV, error) {
	r, err := s.queries().GetCV(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return CV{}, nil
	}
	if err != nil {
		return CV{}, err
	}
	return CV{Data: r.Data, UpdatedAt: r.UpdatedAt}, nil
}

// SaveCV ersetzt den Lebenslauf. Die Struktur prüft der OpenAPI-Validator; hier nur der Name.
func (s *Service) SaveCV(ctx context.Context, data json.RawMessage) (CV, error) {
	if err := validateCVData(data); err != nil {
		return CV{}, err
	}
	r, err := s.queries().UpsertCV(ctx, data)
	if err != nil {
		return CV{}, err
	}
	return CV{Data: r.Data, UpdatedAt: r.UpdatedAt}, nil
}

// validateCVData prüft gespeicherte oder vorgeschlagene Lebenslauf-Daten; die Struktur prüft der OpenAPI-Validator.
func validateCVData(data json.RawMessage) error {
	var head struct {
		Person struct {
			Name string `json:"name"`
		} `json:"person"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return &domain.ValidationError{Field: "person", Detail: "kein gültiges JSON-Objekt"}
	}
	_, err := requireText("person.name", head.Person.Name)
	return err
}
