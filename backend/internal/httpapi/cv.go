package httpapi

import (
	"context"
	"encoding/json"
	"fmt"

	"bewerbungsmanager/internal/service"
)

// emptyCv ist die Antwort, solange kein Lebenslauf gespeichert ist; Listen sind leer statt null.
func emptyCv() Cv {
	return Cv{
		Person:     CvPerson{Links: []CvLink{}},
		Experience: []CvExperience{},
		Education:  []CvEducation{},
		Skills:     []CvSkillGroup{},
		Projects:   []CvProject{},
		Languages:  []CvLanguage{},
	}
}

// cvDTO wandelt den gespeicherten Lebenslauf in das API-Schema; ohne Daten die leere Struktur.
func cvDTO(c service.CV) (Cv, error) {
	out := emptyCv()
	if c.Data == nil {
		return out, nil
	}
	if err := json.Unmarshal(c.Data, &out); err != nil {
		return Cv{}, fmt.Errorf("gespeicherten Lebenslauf lesen: %w", err)
	}
	out.UpdatedAt = &c.UpdatedAt
	return out, nil
}

func (s *Server) GetCv(ctx context.Context, _ GetCvRequestObject) (GetCvResponseObject, error) {
	c, err := s.svc.GetCV(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cvDTO(c)
	if err != nil {
		return nil, err
	}
	return GetCv200JSONResponse(out), nil
}

func (s *Server) AgentGetCv(ctx context.Context, _ AgentGetCvRequestObject) (AgentGetCvResponseObject, error) {
	c, err := s.svc.GetCV(ctx)
	if err != nil {
		return nil, err
	}
	if c.Data == nil {
		return nil, &service.NotFoundError{Resource: "Lebenslauf"}
	}
	out, err := cvDTO(c)
	if err != nil {
		return nil, err
	}
	return AgentGetCv200JSONResponse(out), nil
}

func (s *Server) SaveCv(ctx context.Context, req SaveCvRequestObject) (SaveCvResponseObject, error) {
	body := *req.Body
	body.UpdatedAt = nil
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("lebenslauf serialisieren: %w", err)
	}
	c, err := s.svc.SaveCV(ctx, data)
	if err != nil {
		return nil, err
	}
	body.UpdatedAt = &c.UpdatedAt
	return SaveCv200JSONResponse(body), nil
}
