package service_test

import (
	"context"
	"testing"
	"time"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
	"bewerbungsmanager/internal/testdb"
)

var ctx = context.Background()

func newService(t *testing.T) *service.Service {
	t.Helper()
	testdb.Reset(t, testPool)
	return service.New(testPool, time.Now)
}

func ptr[T any](v T) *T { return &v }

// day liefert heute + offset Tage (negativ = Vergangenheit).
func day(offset int) time.Time { return domain.DateOf(time.Now()).AddDate(0, 0, offset) }

func mustCompany(t *testing.T, svc *service.Service, name string) service.Company {
	t.Helper()
	c, err := svc.CreateCompany(ctx, service.CompanyInput{Name: name})
	if err != nil {
		t.Fatalf("Firma anlegen: %v", err)
	}
	return c
}
