package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bewerbungsmanager/internal/httpapi"
	"bewerbungsmanager/internal/service"
	"bewerbungsmanager/internal/testdb"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWith(t)
}

func newTestServerWith(t *testing.T, opts ...service.Option) *httptest.Server {
	t.Helper()
	testdb.Reset(t, testPool)
	h, err := httpapi.NewRouter(service.New(testPool, time.Now, opts...), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

type response struct {
	Status      int
	ContentType string
	Header      http.Header
	Body        []byte
}

func call(t *testing.T, srv *httptest.Server, method, path string, body any) response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{Status: res.StatusCode, ContentType: res.Header.Get("Content-Type"), Body: data, Header: res.Header}
}

func (r response) object(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("kein JSON-Objekt: %s", r.Body)
	}
	return m
}

func (r response) array(t *testing.T) []any {
	t.Helper()
	var a []any
	if err := json.Unmarshal(r.Body, &a); err != nil {
		t.Fatalf("kein JSON-Array: %s", r.Body)
	}
	return a
}

func expectStatus(t *testing.T, r response, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("Status %d, erwartet %d; Body: %s", r.Status, want, r.Body)
	}
}

func expectProblem(t *testing.T, r response, status int, typ string) map[string]any {
	t.Helper()
	expectStatus(t, r, status)
	if r.ContentType != "application/problem+json" {
		t.Errorf("Content-Type = %q", r.ContentType)
	}
	p := r.object(t)
	if typ != "" && p["type"] != typ {
		t.Errorf("Problem-Typ = %v, erwartet %s", p["type"], typ)
	}
	return p
}

func today() string { return time.Now().Format("2006-01-02") }

func createCompany(t *testing.T, srv *httptest.Server, name string) string {
	t.Helper()
	r := call(t, srv, http.MethodPost, "/api/v1/companies", map[string]any{"name": name})
	expectStatus(t, r, http.StatusCreated)
	return r.object(t)["id"].(string)
}
