package service_test

import (
	"encoding/json"
	"errors"
	"testing"

	"bewerbungsmanager/internal/domain"
)

func TestGetCVWithoutDataIsEmpty(t *testing.T) {
	svc := newService(t)
	cv, err := svc.GetCV(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Data != nil {
		t.Errorf("erwartet keinen Lebenslauf, bekommen %s", cv.Data)
	}
}

func TestSaveCVStoresAndOverwrites(t *testing.T) {
	svc := newService(t)
	first, err := svc.SaveCV(ctx, json.RawMessage(`{"person":{"name":"Erika Muster"},"skills":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if first.UpdatedAt.IsZero() {
		t.Error("UpdatedAt fehlt")
	}
	if _, err := svc.SaveCV(ctx, json.RawMessage(`{"person":{"name":"Erika Neu"},"skills":["Go"]}`)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetCV(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Person struct{ Name string } `json:"person"`
		Skills []string              `json:"skills"`
	}
	if err := json.Unmarshal(got.Data, &body); err != nil {
		t.Fatal(err)
	}
	if body.Person.Name != "Erika Neu" || len(body.Skills) != 1 {
		t.Errorf("gespeichert: %s", got.Data)
	}
}

func TestSaveCVRequiresName(t *testing.T) {
	svc := newService(t)
	_, err := svc.SaveCV(ctx, json.RawMessage(`{"person":{"name":"   "}}`))
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person.name" {
		t.Fatalf("erwartet ValidationError(person.name), bekommen %v", err)
	}
}
