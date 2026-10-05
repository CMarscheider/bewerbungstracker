package httpapi_test

import (
	"net/http"
	"testing"
)

func TestCVReviewFlow(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectProblem(t, call(t, srv, http.MethodPost, "/api/v1/cv/review", nil), http.StatusConflict, "/problems/conflict")
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/review", nil), http.StatusNotFound, "/problems/not-found")

	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	created := call(t, srv, http.MethodPost, "/api/v1/cv/review", nil)
	expectStatus(t, created, http.StatusCreated)
	review := created.object(t)
	if review["state"] != "angefordert" || review["proposal"] != nil {
		t.Fatalf("angefordert = %v", review)
	}
	id := review["id"].(string)

	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv-reviews?state=angefordert", "", nil), http.StatusUnauthorized, "/problems/unauthorized")
	list := callWith(t, srv, http.MethodGet, "/api/agent/cv-reviews?state=angefordert", agentToken, nil)
	expectStatus(t, list, http.StatusOK)
	if items := list.array(t); len(items) != 1 || items[0].(map[string]any)["id"] != id {
		t.Fatalf("Liste = %v", items)
	}

	proposal := sampleCV()
	proposal["summary"] = "Geschärftes Profil."
	result := map[string]any{"proposal": proposal, "notes": []any{"Kennzahlen zu Projekten ergänzen"}}
	done := callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, agentToken, result)
	expectStatus(t, done, http.StatusOK)
	if done.object(t)["state"] != "fertig" {
		t.Fatalf("fertig = %s", done.Body)
	}
	expectProblem(t, callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, agentToken, result), http.StatusConflict, "/problems/conflict")

	got := call(t, srv, http.MethodGet, "/api/v1/cv/review", nil)
	expectStatus(t, got, http.StatusOK)
	body := got.object(t)
	if body["proposal"].(map[string]any)["summary"] != "Geschärftes Profil." || len(body["notes"].([]any)) != 1 || body["completed_at"] == nil {
		t.Errorf("GET review = %v", body)
	}

	expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/cv/review", nil), http.StatusNoContent)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/review", nil), http.StatusNotFound, "/problems/not-found")
}

func TestCVReviewProposalRejectsBlankRequiredStrings(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	id := call(t, srv, http.MethodPost, "/api/v1/cv/review", nil).object(t)["id"].(string)

	proposal := sampleCV()
	proposal["experience"] = []any{map[string]any{"role": "   ", "organization": "Acme", "start": "2024", "highlights": []any{}}}
	r := callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, agentToken, map[string]any{"proposal": proposal, "notes": []any{}})
	expectProblem(t, r, http.StatusBadRequest, "")
}
