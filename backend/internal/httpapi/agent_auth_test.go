package httpapi_test

import (
	"net/http"
	"testing"
)

func TestAgentAPIDisabledWithoutToken(t *testing.T) {
	srv := newAgentTestServer(t, "")
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv", agentToken, nil), http.StatusNotFound, "/problems/not-found")
}

func TestAgentAPIRequiresToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	r := callWith(t, srv, http.MethodGet, "/api/agent/cv", "", nil)
	expectProblem(t, r, http.StatusUnauthorized, "/problems/unauthorized")
	if r.Header.Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate fehlt")
	}
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv", "falsch-falsch-falsch-falsch-falsch!", nil), http.StatusUnauthorized, "/problems/unauthorized")
}

func TestAgentGetCV(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv", agentToken, nil), http.StatusNotFound, "/problems/not-found")

	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	r := callWith(t, srv, http.MethodGet, "/api/agent/cv", agentToken, nil)
	expectStatus(t, r, http.StatusOK)
	body := r.object(t)
	if body["person"].(map[string]any)["name"] != "Erika Muster" || body["updated_at"] == nil {
		t.Errorf("GET /api/agent/cv = %v", body)
	}
}

func TestUIAPIStaysWithoutToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectStatus(t, call(t, srv, http.MethodGet, "/api/v1/cv", nil), http.StatusOK)
}
