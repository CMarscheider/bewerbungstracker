package service

import (
	"context"
	"errors"

	"bewerbungsmanager/internal/documents"
)

// CVPDF erzeugt den gespeicherten Lebenslauf als PDF und liefert dazu den Dateinamen.
func (s *Service) CVPDF(ctx context.Context) ([]byte, string, error) {
	cv, err := s.GetCV(ctx)
	if err != nil {
		return nil, "", err
	}
	if cv.Data == nil {
		return nil, "", &NotFoundError{Resource: "Lebenslauf"}
	}
	doc, err := documents.ParseCV(cv.Data)
	if err != nil {
		return nil, "", err
	}
	var photo []byte
	p, err := s.GetCVPhoto(ctx)
	var nf *NotFoundError
	switch {
	case err == nil:
		photo = p.Data
	case !errors.As(err, &nf):
		return nil, "", err
	}
	html, err := documents.RenderCV(doc, photo)
	if err != nil {
		return nil, "", err
	}
	if s.pdf == nil {
		return nil, "", &UnavailableError{Detail: "PDF-Erzeugung ist nicht eingerichtet (GOTENBERG_URL fehlt)"}
	}
	pdf, err := s.pdf.Convert(ctx, html, documents.Fonts())
	if errors.Is(err, documents.ErrUnavailable) {
		return nil, "", &UnavailableError{Detail: "PDF-Dienst ist gerade nicht erreichbar"}
	}
	if err != nil {
		return nil, "", err
	}
	return pdf, documents.CVFileName(doc), nil
}
