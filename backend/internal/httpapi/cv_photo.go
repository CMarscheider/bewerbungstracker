package httpapi

import (
	"bytes"
	"context"
	"io"
)

func (s *Server) GetCvPhoto(ctx context.Context, _ GetCvPhotoRequestObject) (GetCvPhotoResponseObject, error) {
	p, err := s.svc.GetCVPhoto(ctx)
	if err != nil {
		return nil, err
	}
	noCache := "no-cache"
	return GetCvPhoto200ImagejpegResponse{Headers: GetCvPhoto200ResponseHeaders{CacheControl: &noCache}, Body: bytes.NewReader(p.Data), ContentLength: int64(len(p.Data))}, nil
}

func (s *Server) SaveCvPhoto(ctx context.Context, req SaveCvPhotoRequestObject) (SaveCvPhotoResponseObject, error) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		// Zu große Bodys lehnt schon die Validierung im Router mit 413 ab; hier
		// bleibt nur ein unerwarteter Lesefehler (wird als 500 behandelt).
		return nil, err
	}
	if err := s.svc.SaveCVPhoto(ctx, data); err != nil {
		return nil, err
	}
	return SaveCvPhoto204Response{}, nil
}

func (s *Server) DeleteCvPhoto(ctx context.Context, _ DeleteCvPhotoRequestObject) (DeleteCvPhotoResponseObject, error) {
	if err := s.svc.DeleteCVPhoto(ctx); err != nil {
		return nil, err
	}
	return DeleteCvPhoto204Response{}, nil
}
