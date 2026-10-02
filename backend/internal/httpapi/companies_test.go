package httpapi_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCompanyCRUD(t *testing.T) {
	srv := newTestServer(t)
	id := createCompany(t, srv, "Acme")

	list := call(t, srv, http.MethodGet, "/api/v1/companies", nil)
	expectStatus(t, list, http.StatusOK)
	if n := len(list.array(t)); n != 1 {
		t.Fatalf("erwartet 1 Firma, bekommen %d", n)
	}

	patched := call(t, srv, http.MethodPatch, "/api/v1/companies/"+id, map[string]any{"website": "https://acme.example"})
	expectStatus(t, patched, http.StatusOK)
	if body := patched.object(t); body["website"] != "https://acme.example" || body["name"] != "Acme" {
		t.Errorf("PATCH = %v", body)
	}

	expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/companies/"+id, nil), http.StatusNoContent)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/companies/"+id, nil), http.StatusNotFound, "/problems/not-found")
}

func TestCreateCompanyDuplicateIsConflict(t *testing.T) {
	srv := newTestServer(t)
	createCompany(t, srv, "Acme")
	r := call(t, srv, http.MethodPost, "/api/v1/companies", map[string]any{"name": "Acme"})
	expectProblem(t, r, http.StatusConflict, "/problems/conflict")
}

func TestCreateCompanyWithoutNameIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	r := call(t, srv, http.MethodPost, "/api/v1/companies", map[string]any{"website": "x"})
	expectProblem(t, r, http.StatusBadRequest, "/problems/bad-request")
}

func TestInvalidUUIDIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	r := call(t, srv, http.MethodGet, "/api/v1/companies/keine-uuid", nil)
	expectProblem(t, r, http.StatusBadRequest, "")
}

func TestPatchCompanyEmptyStringClearsWebsite(t *testing.T) {
	srv := newTestServer(t)
	id := createCompany(t, srv, "Acme")
	expectStatus(t, call(t, srv, http.MethodPatch, "/api/v1/companies/"+id, map[string]any{"website": "https://acme.example"}), http.StatusOK)

	cleared := call(t, srv, http.MethodPatch, "/api/v1/companies/"+id, map[string]any{"website": ""})
	expectStatus(t, cleared, http.StatusOK)
	if body := cleared.object(t); body["name"] != "Acme" {
		t.Errorf("PATCH = %v", body)
	} else if _, ok := body["website"]; ok {
		t.Errorf("website-Schlüssel sollte fehlen: %v", body)
	}
}

func TestDeleteCompanyWithApplicationIsConflict(t *testing.T) {
	srv := newTestServer(t)
	companyID := createCompany(t, srv, "Acme")
	r := call(t, srv, http.MethodPost, "/api/v1/applications", map[string]any{
		"company_id": companyID, "position_title": "Go-Entwickler",
		"first_event": map[string]any{"type": "Beworben", "occurred_on": today()},
	})
	expectStatus(t, r, http.StatusCreated)
	expectProblem(t, call(t, srv, http.MethodDelete, "/api/v1/companies/"+companyID, nil), http.StatusConflict, "/problems/conflict")
}

func TestMalformedJSONIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/companies", strings.NewReader(`{"name":`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	expectProblem(t, response{Status: res.StatusCode, ContentType: res.Header.Get("Content-Type"), Body: data}, http.StatusBadRequest, "")
}

func TestUnknownEndpointIsNotFoundProblem(t *testing.T) {
	srv := newTestServer(t)
	p := expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/nope", nil), http.StatusNotFound, "/problems/not-found")
	if p["detail"] != "Unbekannter Endpunkt" {
		t.Errorf("detail = %v", p["detail"])
	}
}
