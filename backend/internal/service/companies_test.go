package service_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func TestCreateAndGetCompany(t *testing.T) {
	svc := newService(t)
	created, err := svc.CreateCompany(ctx, service.CompanyInput{Name: "  Acme GmbH ", Website: ptr("https://acme.example")})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Acme GmbH" {
		t.Errorf("Name nicht getrimmt: %q", created.Name)
	}
	got, err := svc.GetCompany(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Website == nil || *got.Website != "https://acme.example" || got.ApplicationCount != 0 {
		t.Errorf("GetCompany = %+v", got)
	}
}

func TestCreateCompanyRejectsEmptyName(t *testing.T) {
	svc := newService(t)
	_, err := svc.CreateCompany(ctx, service.CompanyInput{Name: "   "})
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("erwartet ValidationError(name), bekommen %v", err)
	}
}

func TestCreateCompanyDuplicateNameConflicts(t *testing.T) {
	svc := newService(t)
	mustCompany(t, svc, "Acme")
	_, err := svc.CreateCompany(ctx, service.CompanyInput{Name: "Acme"})
	var ce *service.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("erwartet ConflictError, bekommen %v", err)
	}
}

func TestListCompaniesSortedByName(t *testing.T) {
	svc := newService(t)
	mustCompany(t, svc, "beta")
	mustCompany(t, svc, "Alpha")
	list, err := svc.ListCompanies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "beta" {
		t.Errorf("ListCompanies = %+v", list)
	}
}

func TestUpdateCompanyPatchesOnlyGivenFields(t *testing.T) {
	svc := newService(t)
	c, err := svc.CreateCompany(ctx, service.CompanyInput{Name: "Acme", Notes: ptr("alt")})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateCompany(ctx, c.ID, service.CompanyPatch{Website: ptr("https://acme.example"), Notes: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Acme" || updated.Website == nil || updated.Notes != nil {
		t.Errorf("UpdateCompany = %+v", updated)
	}
}

func TestDeleteCompany(t *testing.T) {
	svc := newService(t)
	c := mustCompany(t, svc, "Acme")
	if err := svc.DeleteCompany(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.GetCompany(ctx, c.ID)
	var nf *service.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
	if err := svc.DeleteCompany(ctx, c.ID); !errors.As(err, &nf) {
		t.Fatalf("zweites Löschen: erwartet NotFoundError, bekommen %v", err)
	}
}

func TestUpdateCompanyDuplicateNameConflicts(t *testing.T) {
	svc := newService(t)
	mustCompany(t, svc, "Acme")
	b := mustCompany(t, svc, "Beta")
	_, err := svc.UpdateCompany(ctx, b.ID, service.CompanyPatch{Name: ptr("Acme")})
	var ce *service.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("erwartet ConflictError, bekommen %v", err)
	}
}

func TestUpdateCompanyUnknownID(t *testing.T) {
	svc := newService(t)
	_, err := svc.UpdateCompany(ctx, uuid.New(), service.CompanyPatch{Name: ptr("X")})
	var nf *service.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
}
