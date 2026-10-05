package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsSlowRoute(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{http.MethodPut, "/api/agent/applications/0b6c9d3e-1f2a-4b5c-8d7e-9f0a1b2c3d4e/documents", true},
		{http.MethodPut, "/api/v1/applications/0b6c9d3e-1f2a-4b5c-8d7e-9f0a1b2c3d4e/documents", true},
		{http.MethodPost, "/api/v1/applications/0b6c9d3e-1f2a-4b5c-8d7e-9f0a1b2c3d4e/documents/draft", true},
		{http.MethodGet, "/api/v1/applications/0b6c9d3e-1f2a-4b5c-8d7e-9f0a1b2c3d4e/documents", false},
		{http.MethodPost, "/api/v1/applications/0b6c9d3e-1f2a-4b5c-8d7e-9f0a1b2c3d4e/documents", false},
		{http.MethodGet, "/api/v1/applications/0b6c9d3e-1f2a-4b5c-8d7e-9f0a1b2c3d4e/documents/pdf", false},
		{http.MethodPut, "/api/v1/applications/x/y/documents", false},
		{http.MethodPut, "/api/v1/applications/0b6c/documents/extra", false},
		{http.MethodGet, "/api/v1/cv/pdf", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, c.path, nil)
		if got := isSlowRoute(r); got != c.want {
			t.Errorf("%s %s: %v, erwartet %v", c.method, c.path, got, c.want)
		}
	}
}

// TestExtendWriteDeadline: Mit kurzem WriteTimeout bricht eine langsame Antwort ab – außer auf
// den Unterlagen-Routen, deren Frist die Middleware verlängert.
func TestExtendWriteDeadline(t *testing.T) {
	slow := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = io.WriteString(w, "fertig")
	})
	srv := httptest.NewUnstartedServer(extendWriteDeadline(2*time.Second, slow))
	srv.Config.WriteTimeout = 50 * time.Millisecond
	srv.Start()
	defer srv.Close()

	do := func(method, path string) (string, error) {
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		res, err := srv.Client().Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = res.Body.Close() }()
		b, err := io.ReadAll(res.Body)
		return string(b), err
	}

	if body, err := do(http.MethodPut, "/api/v1/applications/abc/documents"); err != nil || body != "fertig" {
		t.Errorf("Unterlagen-Route: %q, %v", body, err)
	}
	if body, err := do(http.MethodGet, "/api/v1/companies"); err == nil && body == "fertig" {
		t.Error("andere Route: Antwort trotz abgelaufenem WriteTimeout")
	}
}
