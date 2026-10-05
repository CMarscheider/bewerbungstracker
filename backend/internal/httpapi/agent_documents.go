package httpapi

import (
	"context"

	"bewerbungsmanager/internal/service"
)

func (s *Server) AgentListApplications(ctx context.Context, req AgentListApplicationsRequestObject) (AgentListApplicationsResponseObject, error) {
	as, err := s.svc.ListAgentApplications(ctx, string(req.Params.DocumentsState))
	if err != nil {
		return nil, err
	}
	out := make(AgentListApplications200JSONResponse, 0, len(as))
	for _, a := range as {
		out = append(out, agentApplicationDTO(a))
	}
	return out, nil
}

func (s *Server) AgentPutDocuments(ctx context.Context, req AgentPutDocumentsRequestObject) (AgentPutDocumentsResponseObject, error) {
	b := req.Body
	d, err := s.svc.SaveAgentDocuments(ctx, req.Id, service.DocumentsInput{
		Version: b.Version, Language: string(b.Language), CoverLetter: b.CoverLetter, ProfileLine: b.ProfileLine,
		Highlights: derefStrings(b.Highlights), MailSubject: b.MailSubject, MailBody: b.MailBody,
	})
	if err != nil {
		return nil, err
	}
	return AgentPutDocuments200JSONResponse(documentsDTO(d)), nil
}
