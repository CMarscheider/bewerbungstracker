package service

import (
	"context"
	"net/http"
	"time"

	"bewerbungsmanager/internal/domain"
)

// Photo ist das gespeicherte Bewerbungsfoto (JPEG).
type Photo struct {
	Data      []byte
	UpdatedAt time.Time
}

func (s *Service) GetCVPhoto(ctx context.Context) (Photo, error) {
	r, err := s.queries().GetCVPhoto(ctx)
	if err != nil {
		return Photo{}, notFoundIfNoRows(err, "Foto")
	}
	return Photo{Data: r.Image, UpdatedAt: r.UpdatedAt}, nil
}

// SaveCVPhoto ersetzt das Foto. Erlaubt ist nur JPEG; der Browser wandelt vor dem Hochladen um.
func (s *Service) SaveCVPhoto(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return &domain.ValidationError{Field: "photo", Detail: "darf nicht leer sein"}
	}
	if http.DetectContentType(data) != "image/jpeg" {
		return &domain.ValidationError{Field: "photo", Detail: "muss ein JPEG-Bild sein"}
	}
	return s.queries().UpsertCVPhoto(ctx, data)
}

// DeleteCVPhoto entfernt das Foto; ohne Foto passiert nichts.
func (s *Service) DeleteCVPhoto(ctx context.Context) error {
	return s.queries().DeleteCVPhoto(ctx)
}
