package httpapi

import "context"

func (s *Server) ListSuggestions(ctx context.Context, _ ListSuggestionsRequestObject) (ListSuggestionsResponseObject, error) {
	sgs, err := s.svc.ListOpenSuggestions(ctx)
	if err != nil {
		return nil, err
	}
	out := make(ListSuggestions200JSONResponse, 0, len(sgs))
	for _, sg := range sgs {
		out = append(out, suggestionDTO(sg))
	}
	return out, nil
}

func (s *Server) AcceptSuggestion(ctx context.Context, req AcceptSuggestionRequestObject) (AcceptSuggestionResponseObject, error) {
	a, err := s.svc.AcceptSuggestion(ctx, req.Id, req.Body.ApplicationId)
	if err != nil {
		return nil, err
	}
	return AcceptSuggestion200JSONResponse(applicationDTO(a)), nil
}

func (s *Server) DismissSuggestion(ctx context.Context, req DismissSuggestionRequestObject) (DismissSuggestionResponseObject, error) {
	if err := s.svc.DismissSuggestion(ctx, req.Id); err != nil {
		return nil, err
	}
	return DismissSuggestion204Response{}, nil
}

func (s *Server) ClearGmailThread(ctx context.Context, req ClearGmailThreadRequestObject) (ClearGmailThreadResponseObject, error) {
	if err := s.svc.ClearGmailThread(ctx, req.Id); err != nil {
		return nil, err
	}
	return ClearGmailThread204Response{}, nil
}

// defaultProcessedMailLimit entspricht dem Standardwert von limit in der Spec.
const defaultProcessedMailLimit = 50

func (s *Server) ListProcessedMails(ctx context.Context, req ListProcessedMailsRequestObject) (ListProcessedMailsResponseObject, error) {
	limit := defaultProcessedMailLimit
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	ms, err := s.svc.ListProcessedMails(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make(ListProcessedMails200JSONResponse, 0, len(ms))
	for _, m := range ms {
		out = append(out, processedMailDTO(m))
	}
	return out, nil
}

func (s *Server) DeleteProcessedMail(ctx context.Context, req DeleteProcessedMailRequestObject) (DeleteProcessedMailResponseObject, error) {
	if err := s.svc.DeleteProcessedMail(ctx, req.MessageId); err != nil {
		return nil, err
	}
	return DeleteProcessedMail204Response{}, nil
}
