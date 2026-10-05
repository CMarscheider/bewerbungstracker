package service_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func proposalJSON(t *testing.T, summary string) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"person": map[string]any{"name": "Erika Mustermann", "links": []any{}}, "summary": summary,
		"experience": []any{}, "education": []any{}, "skills": []any{}, "projects": []any{}, "languages": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRequestCVReviewNeedsSavedCV(t *testing.T) {
	svc := newService(t)
	_, err := svc.RequestCVReview(ctx)
	var ce *service.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("erwartet ConflictError, bekommen %v", err)
	}
}

func TestCVReviewLifecycle(t *testing.T) {
	svc := newService(t)
	saveSampleCV(t, svc)
	cv, err := svc.GetCV(ctx)
	if err != nil {
		t.Fatal(err)
	}

	r, err := svc.RequestCVReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r.State != service.CVReviewRequested || !r.BasedOnUpdatedAt.Equal(cv.UpdatedAt) || r.Proposal != nil || len(r.Notes) != 0 {
		t.Errorf("angefordert = %+v", r)
	}
	var ce *service.ConflictError
	if _, err := svc.RequestCVReview(ctx); !errors.As(err, &ce) {
		t.Fatalf("zweite Anforderung: erwartet ConflictError, bekommen %v", err)
	}

	open, err := svc.ListCVReviews(ctx, service.CVReviewRequested)
	if err != nil || len(open) != 1 || open[0].ID != r.ID {
		t.Fatalf("ListCVReviews = %+v, %v", open, err)
	}

	done, err := svc.CompleteCVReview(ctx, r.ID, proposalJSON(t, "Neues Profil"), []string{"Kennzahlen ergänzen"})
	if err != nil {
		t.Fatal(err)
	}
	if done.State != service.CVReviewReady || done.CompletedAt == nil || len(done.Notes) != 1 || !json.Valid(done.Proposal) {
		t.Errorf("fertig = %+v", done)
	}
	if _, err := svc.CompleteCVReview(ctx, r.ID, proposalJSON(t, "x"), nil); !errors.As(err, &ce) {
		t.Fatalf("zweites Abliefern: erwartet ConflictError, bekommen %v", err)
	}

	got, err := svc.OpenCVReview(ctx)
	if err != nil || got.State != service.CVReviewReady {
		t.Fatalf("OpenCVReview = %+v, %v", got, err)
	}

	if err := svc.CloseCVReview(ctx); err != nil {
		t.Fatal(err)
	}
	var nf *service.NotFoundError
	if _, err := svc.OpenCVReview(ctx); !errors.As(err, &nf) {
		t.Fatalf("nach Abschluss: erwartet NotFoundError, bekommen %v", err)
	}
	if err := svc.CloseCVReview(ctx); err != nil {
		t.Fatalf("zweites Abschließen soll nicht fehlschlagen: %v", err)
	}
	if _, err := svc.RequestCVReview(ctx); err != nil {
		t.Fatalf("neue Anforderung nach Abschluss: %v", err)
	}
}

func TestCompleteCVReviewValidates(t *testing.T) {
	svc := newService(t)
	saveSampleCV(t, svc)
	r, err := svc.RequestCVReview(ctx)
	if err != nil {
		t.Fatal(err)
	}

	blank, _ := json.Marshal(map[string]any{"person": map[string]any{"name": "  "}})
	var ve *domain.ValidationError
	if _, err := svc.CompleteCVReview(ctx, r.ID, blank, nil); !errors.As(err, &ve) || ve.Field != "person.name" {
		t.Fatalf("leerer Name: erwartet ValidationError(person.name), bekommen %v", err)
	}

	var nf *service.NotFoundError
	if _, err := svc.CompleteCVReview(ctx, uuid.New(), proposalJSON(t, "x"), nil); !errors.As(err, &nf) {
		t.Fatalf("unbekannte ID: erwartet NotFoundError, bekommen %v", err)
	}
}
