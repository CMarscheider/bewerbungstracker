package service_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func TestRequestDocumentsSetsRequested(t *testing.T) {
	svc := newService(t)
	a, err := svc.CreateAgentJob(ctx, sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	if a.DocumentsState != service.DocsNone {
		t.Fatalf("Start: %s", a.DocumentsState)
	}
	got, err := svc.RequestDocuments(ctx, a.ID)
	if err != nil || got.DocumentsState != service.DocsRequested {
		t.Fatalf("angefordert: %v %s", err, got.DocumentsState)
	}
	if _, err := svc.RequestDocuments(ctx, a.ID); err != nil { // idempotent
		t.Fatal(err)
	}
	list, err := svc.ListAgentApplications(ctx, service.DocsRequested)
	if err != nil || len(list) != 1 || list[0].ID != a.ID || list[0].PostingText == nil {
		t.Fatalf("Agent-Liste: %v %+v", err, list)
	}
	if list[0].CompanyName != "Acme GmbH" || list[0].DocumentsVersion != 0 || list[0].DocumentsState != service.DocsRequested {
		t.Fatalf("Agent-Felder: %+v", list[0])
	}
	none, err := svc.ListAgentApplications(ctx, service.DocsNone)
	if err != nil || len(none) != 0 {
		t.Fatalf("keine: %v %+v", err, none)
	}
}

func TestRequestDocumentsUnknownApplication(t *testing.T) {
	svc := newService(t)
	var nf *service.NotFoundError
	if _, err := svc.RequestDocuments(ctx, uuid.New()); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFound, bekommen %v", err)
	}
}

func TestListAgentApplicationsUnknownState(t *testing.T) {
	svc := newService(t)
	var ve *domain.ValidationError
	if _, err := svc.ListAgentApplications(ctx, "quatsch"); !errors.As(err, &ve) || ve.Field != "documents_state" {
		t.Fatalf("erwartet ValidationError documents_state, bekommen %v", err)
	}
}
