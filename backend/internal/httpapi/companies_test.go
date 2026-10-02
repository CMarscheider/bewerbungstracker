package httpapi_test

import (
	"net/http"
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
