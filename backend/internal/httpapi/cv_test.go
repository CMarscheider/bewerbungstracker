package httpapi_test

import (
	"net/http"
	"testing"
)

func sampleCV() map[string]any {
	return map[string]any{
		"person": map[string]any{
			"name": "Erika Muster", "headline": "Frontend-Entwicklerin", "email": "erika@example.com",
			"links": []any{map[string]any{"label": "GitHub", "url": "https://github.com/erika"}},
		},
		"summary": "Baut gern Oberflächen.",
		"experience": []any{map[string]any{
			"role": "Werkstudentin", "organization": "Acme", "start": "2024-03",
			"highlights": []any{"Angular-Komponenten gebaut"},
		}},
		"education": []any{map[string]any{"degree": "B.Sc. Informatik", "institution": "FH Bielefeld", "start": "2021", "end": "2025"}},
		"skills":    []any{map[string]any{"category": "Frontend", "items": []any{"Angular", "TypeScript"}}},
		"projects":  []any{map[string]any{"name": "Bewerbungs-Tracker", "description": "Go + Angular", "technologies": []any{"Go"}}},
		"languages": []any{map[string]any{"language": "Deutsch", "level": "Muttersprache"}},
	}
}

func TestGetCVWithoutDataReturnsEmptyStructure(t *testing.T) {
	srv := newTestServer(t)
	r := call(t, srv, http.MethodGet, "/api/v1/cv", nil)
	expectStatus(t, r, http.StatusOK)
	body := r.object(t)
	if _, ok := body["updated_at"]; ok {
		t.Errorf("updated_at darf ohne Lebenslauf fehlen: %v", body)
	}
	if exp, ok := body["experience"].([]any); !ok || len(exp) != 0 {
		t.Errorf("experience soll leere Liste sein: %v", body["experience"])
	}
}

func TestSaveAndGetCV(t *testing.T) {
	srv := newTestServer(t)
	saved := call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV())
	expectStatus(t, saved, http.StatusOK)
	if saved.object(t)["updated_at"] == nil {
		t.Error("updated_at fehlt nach dem Speichern")
	}

	got := call(t, srv, http.MethodGet, "/api/v1/cv", nil)
	expectStatus(t, got, http.StatusOK)
	body := got.object(t)
	if body["updated_at"] == nil {
		t.Error("updated_at fehlt beim Lesen")
	}
	person := body["person"].(map[string]any)
	if person["name"] != "Erika Muster" || len(person["links"].([]any)) != 1 {
		t.Errorf("person = %v", person)
	}
	exp := body["experience"].([]any)[0].(map[string]any)
	if exp["start"] != "2024-03" || exp["end"] != nil {
		t.Errorf("experience = %v", exp)
	}
}

func TestSaveCVWithoutNameIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	cv := sampleCV()
	cv["person"] = map[string]any{"name": "", "links": []any{}}
	expectProblem(t, call(t, srv, http.MethodPut, "/api/v1/cv", cv), http.StatusBadRequest, "")
}

func TestSaveCVWithInvalidPeriodIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	cv := sampleCV()
	cv["education"] = []any{map[string]any{"degree": "Abitur", "institution": "Gymnasium", "start": "März 2018"}}
	expectProblem(t, call(t, srv, http.MethodPut, "/api/v1/cv", cv), http.StatusBadRequest, "")
}

func TestSaveCVWithBlankNameIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	cv := sampleCV()
	cv["person"] = map[string]any{"name": "   ", "links": []any{}}
	p := expectProblem(t, call(t, srv, http.MethodPut, "/api/v1/cv", cv), http.StatusBadRequest, "/problems/validation-error")
	if p["field"] != "person.name" {
		t.Errorf("field = %v", p["field"])
	}
}
