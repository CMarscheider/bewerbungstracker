package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"bewerbungsmanager/internal/documents"
	"bewerbungsmanager/internal/service"
	"bewerbungsmanager/internal/testdb"
)

type fakeConverter struct {
	html   []byte
	assets map[string][]byte
	err    error
}

func (f *fakeConverter) Convert(_ context.Context, html []byte, assets map[string][]byte) ([]byte, error) {
	f.html, f.assets = html, assets
	if f.err != nil {
		return nil, f.err
	}
	return []byte("%PDF-fake"), nil
}

func newServiceWith(t *testing.T, opts ...service.Option) *service.Service {
	t.Helper()
	testdb.Reset(t, testPool)
	return service.New(testPool, time.Now, opts...)
}

func saveSampleCV(t *testing.T, svc *service.Service) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{
		"person":     map[string]any{"name": "Erika Mustermann", "links": []any{}},
		"experience": []any{}, "education": []any{}, "skills": []any{}, "projects": []any{}, "languages": []any{},
	})
	if _, err := svc.SaveCV(ctx, data); err != nil {
		t.Fatal(err)
	}
}

func TestCVPDFRendersWithConverter(t *testing.T) {
	conv := &fakeConverter{}
	svc := newServiceWith(t, service.WithPDFConverter(conv))
	saveSampleCV(t, svc)
	if err := svc.SaveCVPhoto(ctx, jpegHeader); err != nil {
		t.Fatal(err)
	}

	pdf, name, err := svc.CVPDF(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(pdf) != "%PDF-fake" || name != "Lebenslauf_Erika_Mustermann.pdf" {
		t.Errorf("pdf=%q name=%q", pdf, name)
	}
	if !strings.Contains(string(conv.html), "Erika Mustermann") || !strings.Contains(string(conv.html), "data:image/jpeg;base64,") {
		t.Error("HTML ohne Name oder Foto an den Konverter übergeben")
	}
	if _, ok := conv.assets["Carlito-Regular.ttf"]; !ok {
		t.Error("Schriften wurden nicht mitgeschickt")
	}
}

func TestCVPDFWithoutCVIsNotFound(t *testing.T) {
	svc := newServiceWith(t, service.WithPDFConverter(&fakeConverter{}))
	_, _, err := svc.CVPDF(ctx)
	var nf *service.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
}

func TestCVPDFUnavailable(t *testing.T) {
	var ue *service.UnavailableError

	svc := newServiceWith(t) // ohne Konverter
	saveSampleCV(t, svc)
	if _, _, err := svc.CVPDF(ctx); !errors.As(err, &ue) {
		t.Fatalf("ohne Konverter: erwartet UnavailableError, bekommen %v", err)
	}

	svc = newServiceWith(t, service.WithPDFConverter(&fakeConverter{err: documents.ErrUnavailable}))
	saveSampleCV(t, svc)
	if _, _, err := svc.CVPDF(ctx); !errors.As(err, &ue) {
		t.Fatalf("Gotenberg weg: erwartet UnavailableError, bekommen %v", err)
	}
	if _, _, err := svc.CVPDF(ctx); !errors.Is(err, documents.ErrUnavailable) {
		t.Errorf("Ursache geht verloren: %v", err)
	}
}
