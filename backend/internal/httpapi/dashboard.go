package httpapi

import "context"

func daysOr(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}

func (s *Server) ListDeadlines(ctx context.Context, req ListDeadlinesRequestObject) (ListDeadlinesResponseObject, error) {
	list, err := s.svc.Deadlines(ctx, daysOr(req.Params.WithinDays, 7))
	if err != nil {
		return nil, err
	}
	out := make(ListDeadlines200JSONResponse, 0, len(list))
	for _, d := range list {
		out = append(out, Deadline{
			ApplicationId: d.ApplicationID, CompanyName: d.CompanyName, PositionTitle: d.PositionTitle,
			EventType: EventType(d.EventType), DueOn: toDate(d.DueOn), Overdue: d.Overdue,
		})
	}
	return out, nil
}

func (s *Server) ListAppointments(ctx context.Context, req ListAppointmentsRequestObject) (ListAppointmentsResponseObject, error) {
	list, err := s.svc.Appointments(ctx, daysOr(req.Params.WithinDays, 14))
	if err != nil {
		return nil, err
	}
	out := make(ListAppointments200JSONResponse, 0, len(list))
	for _, a := range list {
		out = append(out, Appointment{
			ApplicationId: a.ApplicationID, CompanyName: a.CompanyName, PositionTitle: a.PositionTitle,
			EventType: EventType(a.EventType), Date: toDate(a.Date), InterviewRound: a.InterviewRound,
		})
	}
	return out, nil
}

func (s *Server) GetFunnel(ctx context.Context, _ GetFunnelRequestObject) (GetFunnelResponseObject, error) {
	steps, err := s.svc.Funnel(ctx)
	if err != nil {
		return nil, err
	}
	out := make(GetFunnel200JSONResponse, 0, len(steps))
	for _, st := range steps {
		out = append(out, FunnelStep{Stage: st.Stage, Reached: st.Reached, Rate: st.Rate})
	}
	return out, nil
}

func (s *Server) GetSummary(ctx context.Context, _ GetSummaryRequestObject) (GetSummaryResponseObject, error) {
	sum, err := s.svc.Summary(ctx)
	if err != nil {
		return nil, err
	}
	rejections := make([]RejectionCount, 0, len(sum.RejectionsAfter))
	for _, r := range sum.RejectionsAfter {
		rejections = append(rejections, RejectionCount{EventType: EventType(r.Key), Count: r.Count})
	}
	sources := make([]SourceStats, 0, len(sum.BySource))
	for _, src := range sum.BySource {
		sources = append(sources, SourceStats{
			Source: src.Source, Applied: src.Applied,
			ReachedInterview: src.ReachedInterview, ReachedOffer: src.ReachedOffer,
		})
	}
	return GetSummary200JSONResponse{
		Applied: sum.Applied, Responded: sum.Responded, ResponseRate: sum.ResponseRate,
		MedianDaysToResponse: sum.MedianDaysToResponse, AvgDaysToResponse: sum.AvgDaysToResponse,
		RejectionsAfter: rejections, BySource: sources,
	}, nil
}
