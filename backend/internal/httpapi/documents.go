package httpapi

import (
	"bytes"
	"context"

	"bewerbungsmanager/internal/service"
)

func (s *Server) RequestDocuments(ctx context.Context, req RequestDocumentsRequestObject) (RequestDocumentsResponseObject, error) {
	a, err := s.svc.RequestDocuments(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return RequestDocuments200JSONResponse(applicationDTO(a)), nil
}

func (s *Server) GetDocuments(ctx context.Context, req GetDocumentsRequestObject) (GetDocumentsResponseObject, error) {
	d, err := s.svc.GetDocuments(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return GetDocuments200JSONResponse(documentsDTO(d)), nil
}

func (s *Server) UpdateDocuments(ctx context.Context, req UpdateDocumentsRequestObject) (UpdateDocumentsResponseObject, error) {
	b := req.Body
	d, err := s.svc.UpdateDocuments(ctx, req.Id, service.DocumentsInput{
		Language: string(b.Language), CoverLetter: b.CoverLetter, ProfileLine: b.ProfileLine,
		Highlights: derefStrings(b.Highlights), MailSubject: b.MailSubject, MailBody: b.MailBody,
	})
	if err != nil {
		return nil, err
	}
	return UpdateDocuments200JSONResponse(documentsDTO(d)), nil
}

func (s *Server) GetDocumentsPdf(ctx context.Context, req GetDocumentsPdfRequestObject) (GetDocumentsPdfResponseObject, error) {
	pdf, name, err := s.svc.DocumentsPDF(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	noStore := "no-store"
	disposition := `inline; filename="` + name + `"`
	return GetDocumentsPdf200ApplicationpdfResponse{
		Body:          bytes.NewReader(pdf),
		ContentLength: int64(len(pdf)),
		Headers:       GetDocumentsPdf200ResponseHeaders{CacheControl: &noStore, ContentDisposition: &disposition},
	}, nil
}

func (s *Server) CreateDraft(ctx context.Context, req CreateDraftRequestObject) (CreateDraftResponseObject, error) {
	a, err := s.svc.CreateDraft(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return CreateDraft200JSONResponse(applicationDTO(a)), nil
}
