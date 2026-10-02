package httpapi

import (
	"context"

	"bewerbungsmanager/internal/service"
)

func (s *Server) ListCompanies(ctx context.Context, _ ListCompaniesRequestObject) (ListCompaniesResponseObject, error) {
	cs, err := s.svc.ListCompanies(ctx)
	if err != nil {
		return nil, err
	}
	out := make(ListCompanies200JSONResponse, 0, len(cs))
	for _, c := range cs {
		out = append(out, companyDTO(c))
	}
	return out, nil
}

func (s *Server) CreateCompany(ctx context.Context, req CreateCompanyRequestObject) (CreateCompanyResponseObject, error) {
	c, err := s.svc.CreateCompany(ctx, service.CompanyInput{
		Name: req.Body.Name, Website: req.Body.Website, Notes: req.Body.Notes,
	})
	if err != nil {
		return nil, err
	}
	return CreateCompany201JSONResponse(companyDTO(c)), nil
}

func (s *Server) GetCompany(ctx context.Context, req GetCompanyRequestObject) (GetCompanyResponseObject, error) {
	c, err := s.svc.GetCompany(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return GetCompany200JSONResponse(companyDTO(c)), nil
}

func (s *Server) UpdateCompany(ctx context.Context, req UpdateCompanyRequestObject) (UpdateCompanyResponseObject, error) {
	c, err := s.svc.UpdateCompany(ctx, req.Id, service.CompanyPatch{
		Name: req.Body.Name, Website: req.Body.Website, Notes: req.Body.Notes,
	})
	if err != nil {
		return nil, err
	}
	return UpdateCompany200JSONResponse(companyDTO(c)), nil
}

func (s *Server) DeleteCompany(ctx context.Context, req DeleteCompanyRequestObject) (DeleteCompanyResponseObject, error) {
	if err := s.svc.DeleteCompany(ctx, req.Id); err != nil {
		return nil, err
	}
	return DeleteCompany204Response{}, nil
}
