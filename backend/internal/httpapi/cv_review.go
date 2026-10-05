package httpapi

import (
	"context"
	"encoding/json"
	"fmt"

	"bewerbungsmanager/internal/service"
)

func cvReviewDTO(r service.CVReview) (CvReview, error) {
	out := CvReview{
		Id: r.ID, State: CvReviewState(r.State), BasedOnUpdatedAt: r.BasedOnUpdatedAt,
		Notes: r.Notes, RequestedAt: r.RequestedAt, CompletedAt: r.CompletedAt,
	}
	if r.Proposal != nil {
		p := emptyCv()
		if err := json.Unmarshal(r.Proposal, &p); err != nil {
			return CvReview{}, fmt.Errorf("vorschlag lesen: %w", err)
		}
		out.Proposal = &p
	}
	return out, nil
}

func (s *Server) GetCvReview(ctx context.Context, _ GetCvReviewRequestObject) (GetCvReviewResponseObject, error) {
	r, err := s.svc.OpenCVReview(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cvReviewDTO(r)
	if err != nil {
		return nil, err
	}
	return GetCvReview200JSONResponse(out), nil
}

func (s *Server) RequestCvReview(ctx context.Context, _ RequestCvReviewRequestObject) (RequestCvReviewResponseObject, error) {
	r, err := s.svc.RequestCVReview(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cvReviewDTO(r)
	if err != nil {
		return nil, err
	}
	return RequestCvReview201JSONResponse(out), nil
}

func (s *Server) CloseCvReview(ctx context.Context, _ CloseCvReviewRequestObject) (CloseCvReviewResponseObject, error) {
	if err := s.svc.CloseCVReview(ctx); err != nil {
		return nil, err
	}
	return CloseCvReview204Response{}, nil
}

func (s *Server) AgentListCvReviews(ctx context.Context, req AgentListCvReviewsRequestObject) (AgentListCvReviewsResponseObject, error) {
	rs, err := s.svc.ListCVReviews(ctx, string(req.Params.State))
	if err != nil {
		return nil, err
	}
	out := make(AgentListCvReviews200JSONResponse, 0, len(rs))
	for _, r := range rs {
		dto, err := cvReviewDTO(r)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *Server) AgentCompleteCvReview(ctx context.Context, req AgentCompleteCvReviewRequestObject) (AgentCompleteCvReviewResponseObject, error) {
	proposal := req.Body.Proposal
	proposal.UpdatedAt = nil
	data, err := json.Marshal(proposal)
	if err != nil {
		return nil, fmt.Errorf("vorschlag serialisieren: %w", err)
	}
	r, err := s.svc.CompleteCVReview(ctx, req.Id, data, req.Body.Notes)
	if err != nil {
		return nil, err
	}
	out, err := cvReviewDTO(r)
	if err != nil {
		return nil, err
	}
	return AgentCompleteCvReview200JSONResponse(out), nil
}
