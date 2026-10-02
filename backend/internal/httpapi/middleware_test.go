package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverPanicsWritesInternalProblem(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	h := recoverPanics(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaputt")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("Status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p["type"] != "/problems/internal" {
		t.Errorf("Problem = %v", p)
	}
	if !strings.Contains(logs.String(), "kaputt") || !strings.Contains(logs.String(), "/api/v1/companies") {
		t.Errorf("Log = %s", logs.String())
	}
}

func TestLimitBodyRejectsOversizedBody(t *testing.T) {
	var readErr error
	h := limitBody(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	big := strings.NewReader(strings.Repeat("x", maxBodyBytes+1))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/companies", big))
	var tooLarge *http.MaxBytesError
	if !errors.As(readErr, &tooLarge) {
		t.Errorf("Fehler = %v, erwartet MaxBytesError", readErr)
	}
}
