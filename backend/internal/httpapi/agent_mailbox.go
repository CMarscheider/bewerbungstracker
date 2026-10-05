package httpapi

import (
	"context"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func (s *Server) AgentListOpenApplications(ctx context.Context, _ AgentListOpenApplicationsRequestObject) (AgentListOpenApplicationsResponseObject, error) {
	as, err := s.svc.ListOpenAgentApplications(ctx)
	if err != nil {
		return nil, err
	}
	out := make(AgentListOpenApplications200JSONResponse, 0, len(as))
	for _, a := range as {
		out = append(out, openApplicationDTO(a))
	}
	return out, nil
}

func (s *Server) AgentAddEvent(ctx context.Context, req AgentAddEventRequestObject) (AgentAddEventResponseObject, error) {
	b := req.Body
	e, err := s.svc.AgentAddEvent(ctx, req.Id, domain.NewEvent{
		Type: domain.EventType(b.Type), OccurredOn: b.OccurredOn.Time, DueOn: fromDatePtr(b.DueOn), Note: b.Note,
	})
	if err != nil {
		return nil, err
	}
	return AgentAddEvent201JSONResponse(eventDTO(e)), nil
}

func (s *Server) AgentSetGmailThread(ctx context.Context, req AgentSetGmailThreadRequestObject) (AgentSetGmailThreadResponseObject, error) {
	if err := s.svc.SetGmailThread(ctx, req.Id, req.Body.GmailThreadId); err != nil {
		return nil, err
	}
	return AgentSetGmailThread204Response{}, nil
}

func (s *Server) AgentCreateSuggestion(ctx context.Context, req AgentCreateSuggestionRequestObject) (AgentCreateSuggestionResponseObject, error) {
	b := req.Body
	sg, created, err := s.svc.CreateSuggestion(ctx, service.NewSuggestion{
		ApplicationID: b.ApplicationId, SuggestedType: domain.EventType(b.SuggestedType),
		OccurredOn: b.OccurredOn.Time, DueOn: fromDatePtr(b.DueOn), Reason: b.Reason,
		MailSubject: b.MailSubject, MailFrom: b.MailFrom, MailURL: b.MailUrl, GmailMessageID: b.GmailMessageId,
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return AgentCreateSuggestion200JSONResponse(suggestionDTO(sg)), nil
	}
	return AgentCreateSuggestion201JSONResponse(suggestionDTO(sg)), nil
}

func (s *Server) AgentGetProcessedMail(ctx context.Context, req AgentGetProcessedMailRequestObject) (AgentGetProcessedMailResponseObject, error) {
	m, err := s.svc.GetProcessedMail(ctx, req.MessageId)
	if err != nil {
		return nil, err
	}
	return AgentGetProcessedMail200JSONResponse(processedMailDTO(m)), nil
}

// AgentMarkMailProcessed antwortet 201 für einen neuen Eintrag, 200 für einen schon vorhandenen.
func (s *Server) AgentMarkMailProcessed(ctx context.Context, req AgentMarkMailProcessedRequestObject) (AgentMarkMailProcessedResponseObject, error) {
	b := req.Body
	m, created, err := s.svc.MarkMailProcessed(ctx, b.GmailMessageId, b.ApplicationId, b.Outcome)
	if err != nil {
		return nil, err
	}
	if created {
		return AgentMarkMailProcessed201JSONResponse(processedMailDTO(m)), nil
	}
	return AgentMarkMailProcessed200JSONResponse(processedMailDTO(m)), nil
}
