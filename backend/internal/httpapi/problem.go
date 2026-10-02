package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

const problemBase = "/problems/"

type problem struct {
	Type   string
	Title  string
	Status int
	Detail string
	Extra  map[string]any
}

func writeProblem(w http.ResponseWriter, p problem) {
	body := map[string]any{"type": p.Type, "title": p.Title, "status": p.Status}
	if p.Detail != "" {
		body["detail"] = p.Detail
	}
	for k, v := range p.Extra {
		body[k] = v
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(body)
}

// problemFor übersetzt Fehler aus Domain und Service an genau einer Stelle in HTTP.
func problemFor(err error) problem {
	var (
		validation *domain.ValidationError
		transition *domain.TransitionError
		rule       *domain.RuleError
		notFound   *service.NotFoundError
		conflict   *service.ConflictError
	)
	switch {
	case errors.As(err, &validation):
		return problem{Type: problemBase + "validation-error", Title: "Ungültige Eingabe", Status: http.StatusBadRequest,
			Detail: validation.Detail, Extra: map[string]any{"field": validation.Field}}
	case errors.As(err, &transition):
		extra := map[string]any{"attempted": string(transition.Attempted)}
		if transition.From != "" {
			extra["from"] = string(transition.From)
		}
		return problem{Type: problemBase + "invalid-transition", Title: "Statuswechsel nicht erlaubt",
			Status: http.StatusUnprocessableEntity, Detail: transition.Error(), Extra: extra}
	case errors.As(err, &rule):
		return problem{Type: problemBase + rule.Code, Title: "Fachliche Regel verletzt",
			Status: http.StatusUnprocessableEntity, Detail: rule.Detail}
	case errors.As(err, &notFound):
		return problem{Type: problemBase + "not-found", Title: "Nicht gefunden", Status: http.StatusNotFound, Detail: notFound.Error()}
	case errors.As(err, &conflict):
		return problem{Type: problemBase + "conflict", Title: "Konflikt", Status: http.StatusConflict, Detail: conflict.Detail}
	}
	return problem{Type: problemBase + "internal", Title: "Interner Fehler", Status: http.StatusInternalServerError}
}

func badRequest(detail string) problem {
	return problem{Type: problemBase + "bad-request", Title: "Ungültige Anfrage", Status: http.StatusBadRequest, Detail: detail}
}

func requestErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	writeProblem(w, badRequest(err.Error()))
}

func responseErrorHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		p := problemFor(err)
		if p.Status == http.StatusInternalServerError {
			logger.Error("anfrage fehlgeschlagen", "method", r.Method, "path", r.URL.Path, "err", err)
		}
		writeProblem(w, p)
	}
}
