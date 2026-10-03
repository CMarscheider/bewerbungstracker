package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"bewerbungsmanager/internal/service"
)

type fakeConverter struct{}

func (fakeConverter) Convert(context.Context, []byte, map[string][]byte) ([]byte, error) {
	return []byte("%PDF-fake"), nil
}

func TestCVPDF(t *testing.T) {
	srv := newTestServerWith(t, service.WithPDFConverter(fakeConverter{}))
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/pdf", nil), http.StatusNotFound, "/problems/not-found")

	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/cv/pdf", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("Status %d, Content-Type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	cd := res.Header.Get("Content-Disposition")
	if !strings.HasPrefix(cd, "inline") || !strings.Contains(cd, `filename="Lebenslauf_Erika_Muster.pdf"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestCVPDFWithoutConverterIsUnavailable(t *testing.T) {
	srv := newTestServer(t)
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/pdf", nil), http.StatusServiceUnavailable, "/problems/service-unavailable")
}
