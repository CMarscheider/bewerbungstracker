package httpapi

import (
	"bytes"
	"context"
	"io"

	"bewerbungsmanager/internal/domain"
)

func (s *Server) GetCvPhoto(ctx context.Context, _ GetCvPhotoRequestObject) (GetCvPhotoResponseObject, error) {
	p, err := s.svc.GetCVPhoto(ctx)
	if err != nil {
		return nil, err
	}
	return GetCvPhoto200ImagejpegResponse{Body: bytes.NewReader(p.Data), ContentLength: int64(len(p.Data))}, nil
}

func (s *Server) SaveCvPhoto(ctx context.Context, req SaveCvPhotoRequestObject) (SaveCvPhotoResponseObject, error) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		// http.MaxBytesReader (router.go) bricht bei mehr als 1 MB ab.
		return nil, &domain.ValidationError{Field: "photo", Detail: "zu groß oder unvollständig (max. 1 MB)"}
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
