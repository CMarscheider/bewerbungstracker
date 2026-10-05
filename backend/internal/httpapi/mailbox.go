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
