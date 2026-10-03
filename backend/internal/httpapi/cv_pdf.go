package httpapi

import (
	"bytes"
	"context"
)

func (s *Server) GetCvPdf(ctx context.Context, _ GetCvPdfRequestObject) (GetCvPdfResponseObject, error) {
	pdf, name, err := s.svc.CVPDF(ctx)
	if err != nil {
		return nil, err
	}
	disposition := `inline; filename="` + name + `"`
	return GetCvPdf200ApplicationpdfResponse{
		Body:          bytes.NewReader(pdf),
		ContentLength: int64(len(pdf)),
		Headers:       GetCvPdf200ResponseHeaders{ContentDisposition: &disposition},
	}, nil
}
