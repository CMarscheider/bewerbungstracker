package httpapi

import (
	"context"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func (s *Server) ListApplications(ctx context.Context, req ListApplicationsRequestObject) (ListApplicationsResponseObject, error) {
	f := service.ApplicationFilter{Query: req.Params.Q}
	if req.Params.Phase != nil {
		p := domain.Phase(*req.Params.Phase)
		f.Phase = &p
	}
	if req.Params.Status != nil {
		st := domain.EventType(*req.Params.Status)
		f.Status = &st
	}
	list, err := s.svc.ListApplications(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make(ListApplications200JSONResponse, 0, len(list))
	for _, a := range list {
		out = append(out, summaryDTO(a))
	}
	return out, nil
}

func (s *Server) CreateApplication(ctx context.Context, req CreateApplicationRequestObject) (CreateApplicationResponseObject, error) {
	b := req.Body
	a, err := s.svc.CreateApplication(ctx, service.NewApplication{
		CompanyID: b.CompanyId, PositionTitle: b.PositionTitle,
		JobURL: b.JobUrl, Location: b.Location, Source: b.Source, Notes: b.Notes,
		FirstEvent: newEventFromDTO(b.FirstEvent),
	})
	if err != nil {
		return nil, err
	}
	return CreateApplication201JSONResponse(applicationDTO(a)), nil
}

func (s *Server) GetApplication(ctx context.Context, req GetApplicationRequestObject) (GetApplicationResponseObject, error) {
	a, err := s.svc.GetApplication(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return GetApplication200JSONResponse(applicationDTO(a)), nil
}

func (s *Server) UpdateApplication(ctx context.Context, req UpdateApplicationRequestObject) (UpdateApplicationResponseObject, error) {
	b := req.Body
	a, err := s.svc.UpdateApplication(ctx, req.Id, service.ApplicationPatch{
		CompanyID: b.CompanyId, PositionTitle: b.PositionTitle,
		JobURL: b.JobUrl, Location: b.Location, Source: b.Source, Notes: b.Notes,
	})
	if err != nil {
		return nil, err
	}
	return UpdateApplication200JSONResponse(applicationDTO(a)), nil
}

func (s *Server) DeleteApplication(ctx context.Context, req DeleteApplicationRequestObject) (DeleteApplicationResponseObject, error) {
	if err := s.svc.DeleteApplication(ctx, req.Id); err != nil {
		return nil, err
	}
	return DeleteApplication204Response{}, nil
}

func (s *Server) AddEvent(ctx context.Context, req AddEventRequestObject) (AddEventResponseObject, error) {
	e, err := s.svc.AddEvent(ctx, req.Id, newEventFromDTO(*req.Body))
	if err != nil {
		return nil, err
	}
	return AddEvent201JSONResponse(eventDTO(e)), nil
}

func (s *Server) UndoLastEvent(ctx context.Context, req UndoLastEventRequestObject) (UndoLastEventResponseObject, error) {
	a, err := s.svc.UndoLastEvent(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return UndoLastEvent200JSONResponse(applicationDTO(a)), nil
}

func (s *Server) ListAllowedEvents(ctx context.Context, req ListAllowedEventsRequestObject) (ListAllowedEventsResponseObject, error) {
	types, err := s.svc.AllowedEvents(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	out := make(ListAllowedEvents200JSONResponse, 0, len(types))
	for _, t := range types {
		out = append(out, EventType(t))
	}
	return out, nil
}
