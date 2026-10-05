package httpapi

import (
	"context"

	"bewerbungsmanager/internal/service"
)

func (s *Server) AgentCreateApplication(ctx context.Context, req AgentCreateApplicationRequestObject) (AgentCreateApplicationResponseObject, error) {
	b := req.Body
	a, err := s.svc.CreateAgentJob(ctx, service.AgentJob{
		CompanyName: b.CompanyName, CompanyWebsite: b.CompanyWebsite, PositionTitle: b.PositionTitle,
		JobURL: b.JobUrl, Location: b.Location, Source: b.Source, ContactEmail: b.ContactEmail,
		PostingText: b.PostingText, FitScore: b.FitScore, FitReason: b.FitReason,
	})
	if err != nil {
		return nil, err
	}
	return AgentCreateApplication201JSONResponse(applicationDTO(a)), nil
}
