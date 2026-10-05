package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	expectProblem(t, r, http.StatusBadRequest, "/problems/bad-request")
}

// requestedReview speichert einen Lebenslauf und fordert eine Optimierung an; liefert deren ID.
func requestedReview(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	created := call(t, srv, http.MethodPost, "/api/v1/cv/review", nil)
	expectStatus(t, created, http.StatusCreated)
	return created.object(t)["id"].(string)
}

func reviewResult(notes ...any) map[string]any {
	if notes == nil {
		notes = []any{}
	}
	return map[string]any{"proposal": sampleCV(), "notes": notes}
}

func TestCVReviewSecondRequestConflicts(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	requestedReview(t, srv)
	expectProblem(t, call(t, srv, http.MethodPost, "/api/v1/cv/review", nil), http.StatusConflict, "/problems/conflict")
}

func TestCVReviewCloseWithoutOpenReviewIsNoContent(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/cv/review", nil), http.StatusNoContent)
}

func TestCVReviewUIEndpointsNeedNoToken(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	requestedReview(t, srv)
	expectStatus(t, call(t, srv, http.MethodGet, "/api/v1/cv/review", nil), http.StatusOK)
	expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/cv/review", nil), http.StatusNoContent)
}

func TestAgentCompleteCVReviewErrors(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := requestedReview(t, srv)

	expectProblem(t, callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, "", reviewResult()), http.StatusUnauthorized, "/problems/unauthorized")
	expectProblem(t, callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/3f0e8c5a-1b2c-4d3e-8f40-123456789abc", agentToken, reviewResult()), http.StatusNotFound, "/problems/not-found")
	expectProblem(t, callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/keine-uuid", agentToken, reviewResult()), http.StatusBadRequest, "/problems/bad-request")

	tooMany := make([]any, 31)
	for i := range tooMany {
		tooMany[i] = "Hinweis"
	}
	for name, notes := range map[string][]any{
		"mehr als 30 Hinweise":     tooMany,
		"Hinweis über 500 Zeichen": {strings.Repeat("x", 501)},
		"leerer Hinweis":           {""},
		"Hinweis nur Leerzeichen":  {"   "},
	} {
		t.Run(name, func(t *testing.T) {
			r := callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, agentToken, reviewResult(notes...))
			expectProblem(t, r, http.StatusBadRequest, "/problems/bad-request")
		})
	}

	// Die Grenzen selbst sind erlaubt.
	atLimit := make([]any, 30)
	for i := range atLimit {
		atLimit[i] = strings.Repeat("x", 500)
	}
	expectStatus(t, callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, agentToken, reviewResult(atLimit...)), http.StatusOK)
}

func TestAgentCompleteCVReviewAfterCloseConflicts(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := requestedReview(t, srv)
	expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/cv/review", nil), http.StatusNoContent)
	expectProblem(t, callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, agentToken, reviewResult()), http.StatusConflict, "/problems/conflict")
}

func TestAgentListCVReviewsNeedsValidState(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv-reviews", agentToken, nil), http.StatusBadRequest, "/problems/bad-request")
	expectProblem(t, callWith(t, srv, http.MethodGet, "/api/agent/cv-reviews?state=foo", agentToken, nil), http.StatusBadRequest, "/problems/bad-request")
}

func TestAgentCVReviewResponsesAreNotCached(t *testing.T) {
	srv := newAgentTestServer(t, agentToken)
	id := requestedReview(t, srv)
	for _, r := range []response{
		callWith(t, srv, http.MethodGet, "/api/agent/cv-reviews?state=angefordert", agentToken, nil),
		callWith(t, srv, http.MethodPut, "/api/agent/cv-reviews/"+id, agentToken, reviewResult()),
	} {
		expectStatus(t, r, http.StatusOK)
		if got := r.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q", got)
		}
	}
}
