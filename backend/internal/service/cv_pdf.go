package service

import (
	"context"
	"errors"
	"time"

	"bewerbungsmanager/internal/documents"
)

// pdfTimeout liegt unter WriteTimeout des Servers und proxy_read_timeout von nginx (je 60 s).
const pdfTimeout = 45 * time.Second

// CVPDF erzeugt den gespeicherten Lebenslauf als PDF und liefert dazu den Dateinamen.
func (s *Service) CVPDF(ctx context.Context) ([]byte, string, error) {
	doc, photo, err := s.loadCV(ctx)
	if err != nil {
		return nil, "", err
	}
	html, err := documents.RenderCV(doc, photo)
	if err != nil {
		return nil, "", err
	}
	pdf, err := s.convertPDF(ctx, html)
	if err != nil {
		return nil, "", err
	}
	return pdf, documents.CVFileName(doc), nil
}

// loadCV liest Lebenslauf und Foto (nil, wenn keins gespeichert ist).
// Ohne Lebenslauf: NotFoundError{Resource: "Lebenslauf"}.
func (s *Service) loadCV(ctx context.Context) (documents.CV, []byte, error) {
	cv, err := s.GetCV(ctx)
	if err != nil {
		return documents.CV{}, nil, err
	}
	if cv.Data == nil {
		return documents.CV{}, nil, &NotFoundError{Resource: "Lebenslauf"}
	}
	doc, err := documents.ParseCV(cv.Data)
	if err != nil {
		return documents.CV{}, nil, err
	}
	var photo []byte
	p, err := s.GetCVPhoto(ctx)
	var nf *NotFoundError
	switch {
	case err == nil:
		photo = p.Data
	case !errors.As(err, &nf):
		return documents.CV{}, nil, err
	}
	return doc, photo, nil
}

// convertPDF wandelt HTML über den PDF-Dienst um; fehlt er oder ist er nicht erreichbar: UnavailableError.
func (s *Service) convertPDF(ctx context.Context, html []byte) ([]byte, error) {
	if s.pdf == nil {
		return nil, noPDFService()
	}
	ctx, cancel := context.WithTimeout(ctx, pdfTimeout)
	defer cancel()
	pdf, err := s.pdf.Convert(ctx, html, documents.Fonts())
	return pdf, pdfServiceError(err)
}

// applicationPDF wandelt Anschreiben und Lebenslauf getrennt um und fügt sie zusammen (Anschreiben
// zuerst). Alle drei Schritte teilen sich pdfTimeout. Fehler wie bei convertPDF.
func (s *Service) applicationPDF(ctx context.Context, html documents.ApplicationHTML) ([]byte, error) {
	if s.pdf == nil {
		return nil, noPDFService()
	}
	ctx, cancel := context.WithTimeout(ctx, pdfTimeout)
	defer cancel()
	fonts := documents.Fonts()
	letter, err := s.pdf.Convert(ctx, html.Letter, fonts)
	if err != nil {
		return nil, pdfServiceError(err)
	}
	cv, err := s.pdf.Convert(ctx, html.CV, fonts)
	if err != nil {
		return nil, pdfServiceError(err)
	}
	pdf, err := s.pdf.Merge(ctx, letter, cv)
	return pdf, pdfServiceError(err)
}

// noPDFService: GOTENBERG_URL ist nicht gesetzt.
func noPDFService() error {
	return &UnavailableError{Detail: "PDF-Erzeugung ist nicht eingerichtet (GOTENBERG_URL fehlt)"}
}

// pdfServiceError macht aus einem nicht erreichbaren PDF-Dienst einen UnavailableError.
func pdfServiceError(err error) error {
	if errors.Is(err, documents.ErrUnavailable) {
		return &UnavailableError{Detail: "PDF-Dienst ist gerade nicht erreichbar", Err: err}
	}
	return err
}
