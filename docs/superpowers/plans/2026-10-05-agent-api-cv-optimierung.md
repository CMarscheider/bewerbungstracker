# Agent-API und Lebenslauf-Optimierung Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eine per Token geschützte Agent-API (`/api/agent/*`) entsteht, und der Lebenslauf lässt sich „mit Claude optimieren“: Die Seite fordert eine Optimierung an, ein Agent liefert über die Agent-API einen Vorschlag mit Hinweisen, die Seite zeigt Vorher/Nachher je Abschnitt und übernimmt ausgewählte Abschnitte ins Formular.

**Architecture:** Die Agent-API liegt in derselben OpenAPI-Spec (Tag `Agent`, Security-Schema `agentToken`), eine eigene Middleware prüft vor dem Router `Authorization: Bearer <AGENT_TOKEN>` (ohne Token: 404, falsches Token: 401); der Validator bekommt eine No-op-Authentifizierung. Optimierungsläufe liegen in `cv_reviews` (`angefordert` → `fertig` → `abgeschlossen`, höchstens ein offener Lauf). Die Oberfläche vergleicht den Vorschlag abschnittsweise mit dem aktuellen Formular und übernimmt per Klick; gespeichert wird wie bisher mit „Speichern“.

**Tech Stack:** Go 1.27, PostgreSQL 17, goose, sqlc, oapi-codegen (strict), kin-openapi; Angular 22 (Signals, Reactive Forms, Material), ng-openapi-gen, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-03-agenten-automatisierung-design.md` – Abschnitte „Agent-API“ (Token, `GET /cv`, `GET/PUT /cv-reviews`), „Datenmodell → cv_reviews“, „Oberfläche → Lebenslauf“ (Knopf „Mit Claude optimieren“), „Offene Punkte“ (Pflicht-Strings mit `pattern: '\S'`). Bewerbungs-Endpunkte der Agent-API folgen in Plan 3.

**Umgebung (Windows):** vor Go-/task-Befehlen `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`; Docker Desktop läuft; Commits ohne Attributionszeilen; Branch `feature/agent-api` (legt der Controller an).

---

## Dateien

| Datei | Aktion | Verantwortung |
|---|---|---|
| `backend/sqlc.yaml` | ändern | Override für nullable `timestamptz` → `*time.Time` |
| `backend/migrations/00004_cv_reviews.sql` | neu | Tabelle `cv_reviews` |
| `backend/internal/store/queries/cv_reviews.sql` | neu | Queries |
| `backend/internal/testdb/testdb.go` | ändern | `Reset` leert `cv_reviews` |
| `backend/internal/service/cv.go` | ändern | `validateCVData` herauslösen |
| `backend/internal/service/cv_review.go` (+ `_test.go`) | neu | Optimierungs-Anwendungsfälle |
| `backend/internal/config/config.go` (+ Test) | ändern | `AGENT_TOKEN` |
| `backend/internal/httpapi/agent_auth.go` (+ `_test.go`) | neu | Token-Middleware |
| `backend/internal/httpapi/router.go` | ändern | `RouterOption`, `WithAgentToken`, No-op-Auth im Validator |
| `backend/internal/httpapi/cv.go` | ändern | `cvDTO` gemeinsam nutzen, `AgentGetCv` |
| `backend/internal/httpapi/cv_review.go` (+ `_test.go`) | neu | Review-Handler (UI und Agent) |
| `backend/internal/httpapi/helpers_test.go` | ändern | `callWith` mit Token, `newAgentTestServer` |
| `backend/cmd/server/main.go` | ändern | Token verdrahten |
| `api/openapi.yaml` | ändern | Security-Schema, Agent- und Review-Pfade, `pattern: '\S'` |
| `docker-compose.yml`, `.env.example`, `README.md` | ändern | `AGENT_TOKEN` |
| `frontend/src/app/core/api.ts` | ändern | Review-Methoden |
| `frontend/src/app/features/cv/cv-review-model.ts` (+ `.spec.ts`) | neu | Abschnitte, Vergleich, Textzeilen |
| `frontend/src/app/features/cv/cv-form.ts` (+ Spec) | ändern | `applySection` |
| `frontend/src/app/features/cv/cv-review.ts`, `.html`, `.scss`, `.spec.ts` | neu | Bereich „Mit Claude optimieren“ |
| `frontend/src/app/features/cv/cv.html`, `cv.ts`, `cv.spec.ts` | ändern | Einbindung |

---

### Task 1: Optimierungsläufe – Tabelle, Queries, Service

**Files:**
- Modify: `backend/sqlc.yaml`, `backend/internal/testdb/testdb.go`, `backend/internal/service/cv.go`
- Create: `backend/migrations/00004_cv_reviews.sql`, `backend/internal/store/queries/cv_reviews.sql`, `backend/internal/service/cv_review.go`
- Test: `backend/internal/service/cv_review_test.go`

- [ ] **Step 1: Failing Test schreiben**

`backend/internal/service/cv_review_test.go` (nutzt `saveSampleCV` aus `cv_pdf_test.go`):

```go
package service_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

func proposalJSON(t *testing.T, summary string) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"person": map[string]any{"name": "Erika Mustermann", "links": []any{}}, "summary": summary,
		"experience": []any{}, "education": []any{}, "skills": []any{}, "projects": []any{}, "languages": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRequestCVReviewNeedsSavedCV(t *testing.T) {
	svc := newService(t)
	_, err := svc.RequestCVReview(ctx)
	var ce *service.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("erwartet ConflictError, bekommen %v", err)
	}
}

func TestCVReviewLifecycle(t *testing.T) {
	svc := newService(t)
	saveSampleCV(t, svc)
	cv, err := svc.GetCV(ctx)
	if err != nil {
		t.Fatal(err)
	}

	r, err := svc.RequestCVReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r.State != service.CVReviewRequested || !r.BasedOnUpdatedAt.Equal(cv.UpdatedAt) || r.Proposal != nil || len(r.Notes) != 0 {
		t.Errorf("angefordert = %+v", r)
	}
	var ce *service.ConflictError
	if _, err := svc.RequestCVReview(ctx); !errors.As(err, &ce) {
		t.Fatalf("zweite Anforderung: erwartet ConflictError, bekommen %v", err)
	}

	open, err := svc.ListCVReviews(ctx, service.CVReviewRequested)
	if err != nil || len(open) != 1 || open[0].ID != r.ID {
		t.Fatalf("ListCVReviews = %+v, %v", open, err)
	}

	done, err := svc.CompleteCVReview(ctx, r.ID, proposalJSON(t, "Neues Profil"), []string{"Kennzahlen ergänzen"})
	if err != nil {
		t.Fatal(err)
	}
	if done.State != service.CVReviewReady || done.CompletedAt == nil || len(done.Notes) != 1 || !json.Valid(done.Proposal) {
		t.Errorf("fertig = %+v", done)
	}
	if _, err := svc.CompleteCVReview(ctx, r.ID, proposalJSON(t, "x"), nil); !errors.As(err, &ce) {
		t.Fatalf("zweites Abliefern: erwartet ConflictError, bekommen %v", err)
	}

	got, err := svc.OpenCVReview(ctx)
	if err != nil || got.State != service.CVReviewReady {
		t.Fatalf("OpenCVReview = %+v, %v", got, err)
	}

	if err := svc.CloseCVReview(ctx); err != nil {
		t.Fatal(err)
	}
	var nf *service.NotFoundError
	if _, err := svc.OpenCVReview(ctx); !errors.As(err, &nf) {
		t.Fatalf("nach Abschluss: erwartet NotFoundError, bekommen %v", err)
	}
	if err := svc.CloseCVReview(ctx); err != nil {
		t.Fatalf("zweites Abschließen soll nicht fehlschlagen: %v", err)
	}
	if _, err := svc.RequestCVReview(ctx); err != nil {
		t.Fatalf("neue Anforderung nach Abschluss: %v", err)
	}
}

func TestCompleteCVReviewValidates(t *testing.T) {
	svc := newService(t)
	saveSampleCV(t, svc)
	r, err := svc.RequestCVReview(ctx)
	if err != nil {
		t.Fatal(err)
	}

	blank, _ := json.Marshal(map[string]any{"person": map[string]any{"name": "  "}})
	var ve *domain.ValidationError
	if _, err := svc.CompleteCVReview(ctx, r.ID, blank, nil); !errors.As(err, &ve) || ve.Field != "person.name" {
		t.Fatalf("leerer Name: erwartet ValidationError(person.name), bekommen %v", err)
	}

	var nf *service.NotFoundError
	if _, err := svc.CompleteCVReview(ctx, uuid.New(), proposalJSON(t, "x"), nil); !errors.As(err, &nf) {
		t.Fatalf("unbekannte ID: erwartet NotFoundError, bekommen %v", err)
	}
}
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/service/ -run CVReview`
Expected: Kompilierfehler `svc.RequestCVReview undefined`.

- [ ] **Step 3: sqlc-Override**

In `backend/sqlc.yaml` unter `overrides` ergänzen (nach dem `timestamptz`-Eintrag):

```yaml
          - db_type: "timestamptz"
            nullable: true
            go_type:
              type: "time.Time"
              pointer: true
```

- [ ] **Step 4: Migration**

`backend/migrations/00004_cv_reviews.sql`:

```sql
-- +goose Up
-- Optimierungsläufe für den Lebenslauf: angefordert (Seite) → fertig (Agent) → abgeschlossen (Seite).
CREATE TABLE cv_reviews (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    state               text NOT NULL CHECK (state IN ('angefordert', 'fertig', 'abgeschlossen')),
    based_on_updated_at timestamptz NOT NULL,
    proposal            jsonb,
    notes               jsonb NOT NULL DEFAULT '[]',
    requested_at        timestamptz NOT NULL DEFAULT now(),
    completed_at        timestamptz,
    CHECK (state <> 'fertig' OR proposal IS NOT NULL)
);

-- Höchstens ein offener Lauf.
CREATE UNIQUE INDEX cv_reviews_one_open ON cv_reviews ((true)) WHERE state IN ('angefordert', 'fertig');

-- +goose Down
DROP TABLE cv_reviews;
```

- [ ] **Step 5: Queries und sqlc**

`backend/internal/store/queries/cv_reviews.sql`:

```sql
-- name: CreateCVReview :one
INSERT INTO cv_reviews (state, based_on_updated_at)
VALUES ('angefordert', $1)
RETURNING *;

-- name: GetCVReview :one
SELECT * FROM cv_reviews WHERE id = $1;

-- name: GetOpenCVReview :one
SELECT * FROM cv_reviews WHERE state IN ('angefordert', 'fertig');

-- name: ListCVReviewsByState :many
SELECT * FROM cv_reviews WHERE state = $1 ORDER BY requested_at;

-- name: CompleteCVReview :one
UPDATE cv_reviews
SET state = 'fertig', proposal = $2, notes = $3, completed_at = now()
WHERE id = $1 AND state = 'angefordert'
RETURNING *;

-- name: CloseOpenCVReviews :exec
UPDATE cv_reviews
SET state = 'abgeschlossen', completed_at = coalesce(completed_at, now())
WHERE state IN ('angefordert', 'fertig');
```

Run: `task generate:sqlc`
Expected: `cv_reviews.sql.go`; `models.go` enthält `type CvReview struct` mit `ID uuid.UUID`, `State string`, `BasedOnUpdatedAt time.Time`, `Proposal []byte`, `Notes []byte`, `RequestedAt time.Time`, `CompletedAt *time.Time`; `CompleteCVReviewParams{ID, Proposal, Notes}`. Bestehende generierte Dateien ändern sich nicht (sonst prüfen, warum). Abweichende Namen übernehmen.

- [ ] **Step 6: testdb und gemeinsame Prüfung**

In `backend/internal/testdb/testdb.go` `Reset` ergänzen um `cv_reviews`:

```go
	_, err := pool.Exec(context.Background(), "TRUNCATE companies, applications, application_events, cv, cv_photo, cv_reviews CASCADE")
```

In `backend/internal/service/cv.go` die Namensprüfung aus `SaveCV` in eine Funktion herauslösen (Verhalten unverändert):

```go
// validateCVData prüft gespeicherte oder vorgeschlagene Lebenslauf-Daten; die Struktur prüft der OpenAPI-Validator.
func validateCVData(data json.RawMessage) error {
	var head struct {
		Person struct {
			Name string `json:"name"`
		} `json:"person"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return &domain.ValidationError{Field: "person", Detail: "kein gültiges JSON-Objekt"}
	}
	_, err := requireText("person.name", head.Person.Name)
	return err
}
```

und in `SaveCV` den bisherigen Block durch `if err := validateCVData(data); err != nil { return CV{}, err }` ersetzen.

- [ ] **Step 7: Service**

`backend/internal/service/cv_review.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"bewerbungsmanager/internal/store"
)

// Zustände einer Lebenslauf-Optimierung.
const (
	CVReviewRequested = "angefordert"
	CVReviewReady     = "fertig"
	CVReviewClosed    = "abgeschlossen"
)

// CVReview ist ein Optimierungslauf; Proposal ist nil, solange er angefordert ist.
type CVReview struct {
	ID               uuid.UUID
	State            string
	BasedOnUpdatedAt time.Time
	Proposal         json.RawMessage
	Notes            []string
	RequestedAt      time.Time
	CompletedAt      *time.Time
}

func toCVReview(r store.CvReview) (CVReview, error) {
	notes := []string{}
	if len(r.Notes) > 0 {
		if err := json.Unmarshal(r.Notes, &notes); err != nil {
			return CVReview{}, fmt.Errorf("hinweise lesen: %w", err)
		}
	}
	return CVReview{
		ID: r.ID, State: r.State, BasedOnUpdatedAt: r.BasedOnUpdatedAt, Proposal: r.Proposal,
		Notes: notes, RequestedAt: r.RequestedAt, CompletedAt: r.CompletedAt,
	}, nil
}

// RequestCVReview fordert eine Optimierung des gespeicherten Lebenslaufs an.
func (s *Service) RequestCVReview(ctx context.Context) (CVReview, error) {
	cv, err := s.GetCV(ctx)
	if err != nil {
		return CVReview{}, err
	}
	if cv.Data == nil {
		return CVReview{}, &ConflictError{Detail: "Bitte zuerst den Lebenslauf speichern"}
	}
	r, err := s.queries().CreateCVReview(ctx, cv.UpdatedAt)
	if isUniqueViolation(err) {
		return CVReview{}, &ConflictError{Detail: "Es läuft bereits eine Optimierung"}
	}
	if err != nil {
		return CVReview{}, err
	}
	return toCVReview(r)
}

// OpenCVReview liefert den offenen Lauf (angefordert oder fertig).
func (s *Service) OpenCVReview(ctx context.Context) (CVReview, error) {
	r, err := s.queries().GetOpenCVReview(ctx)
	if err != nil {
		return CVReview{}, notFoundIfNoRows(err, "Optimierung")
	}
	return toCVReview(r)
}

// CloseCVReview schließt den offenen Lauf ab oder zieht die Anfrage zurück; ohne offenen Lauf passiert nichts.
func (s *Service) CloseCVReview(ctx context.Context) error {
	return s.queries().CloseOpenCVReviews(ctx)
}

// ListCVReviews liefert alle Läufe in einem Zustand (für den Agenten).
func (s *Service) ListCVReviews(ctx context.Context, state string) ([]CVReview, error) {
	rows, err := s.queries().ListCVReviewsByState(ctx, state)
	if err != nil {
		return nil, err
	}
	out := make([]CVReview, 0, len(rows))
	for _, r := range rows {
		rv, err := toCVReview(r)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, nil
}

// CompleteCVReview speichert den Vorschlag des Agenten; nur für angeforderte Läufe.
func (s *Service) CompleteCVReview(ctx context.Context, id uuid.UUID, proposal json.RawMessage, notes []string) (CVReview, error) {
	if err := validateCVData(proposal); err != nil {
		return CVReview{}, err
	}
	if notes == nil {
		notes = []string{}
	}
	notesJSON, err := json.Marshal(notes)
	if err != nil {
		return CVReview{}, err
	}
	r, err := s.queries().CompleteCVReview(ctx, store.CompleteCVReviewParams{ID: id, Proposal: proposal, Notes: notesJSON})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, gerr := s.queries().GetCVReview(ctx, id); gerr != nil {
			return CVReview{}, notFoundIfNoRows(gerr, "Optimierung")
		}
		return CVReview{}, &ConflictError{Detail: "Die Optimierung ist nicht (mehr) angefordert"}
	}
	if err != nil {
		return CVReview{}, err
	}
	return toCVReview(r)
}
```

- [ ] **Step 8: Tests und Linter**

Run: `cd backend && go test ./... && golangci-lint run ./...`
Expected: alles `ok`, 0 Befunde.

- [ ] **Step 9: Commit**

```bash
git add backend/sqlc.yaml backend/migrations/00004_cv_reviews.sql backend/internal/store backend/internal/testdb/testdb.go backend/internal/service
git commit -m "feat(cv): Optimierungsläufe für den Lebenslauf"
```

---

### Task 2: Agent-API – Token, Middleware, `GET /api/agent/cv`

**Files:**
- Modify: `backend/internal/config/config.go`, `config_test.go`, `backend/internal/httpapi/router.go`, `cv.go`, `helpers_test.go`, `backend/cmd/server/main.go`, `api/openapi.yaml`, `docker-compose.yml`, `.env.example`
- Create: `backend/internal/httpapi/agent_auth.go`
- Test: `backend/internal/httpapi/agent_auth_test.go`

- [ ] **Step 1: Config – failing Test**

In `backend/internal/config/config_test.go` (Paket `config`, vorhandenen Helfer `env(...)` nutzen) ergänzen:

```go
func TestLoadAgentToken(t *testing.T) {
	c, err := Load(env("DATABASE_URL", "postgres://x", "AGENT_TOKEN", strings.Repeat("a", 32)))
	if err != nil || c.AgentToken != strings.Repeat("a", 32) {
		t.Fatalf("AgentToken = %q, err %v", c.AgentToken, err)
	}
	if _, err := Load(env("DATABASE_URL", "postgres://x", "AGENT_TOKEN", "zu-kurz")); err == nil {
		t.Fatal("zu kurzes AGENT_TOKEN muss abgelehnt werden")
	}
	c, err = Load(env("DATABASE_URL", "postgres://x"))
	if err != nil || c.AgentToken != "" {
		t.Fatalf("ohne AGENT_TOKEN: %q, %v", c.AgentToken, err)
	}
}
```

(Signatur von `env` vorher in der Datei nachsehen und den Aufruf ggf. anpassen; `strings` importieren.)

Run: `cd backend && go test ./internal/config/` → FAIL (`c.AgentToken undefined`).

- [ ] **Step 2: Config implementieren**

In `backend/internal/config/config.go`:

```go
// minAgentTokenLen schützt vor schwachen Tokens; z. B. `openssl rand -hex 32` erzeugt 64 Zeichen.
const minAgentTokenLen = 32

// Config enthält alle Einstellungen des Servers.
type Config struct {
	DatabaseURL  string
	Port         string
	GotenbergURL string // optional; leer = keine PDF-Erzeugung
	AgentToken   string // optional; leer = keine Agent-API
}
```

In `Load` `AgentToken: getenv("AGENT_TOKEN")` setzen und vor `return` prüfen:

```go
	if c.AgentToken != "" && len(c.AgentToken) < minAgentTokenLen {
		return Config{}, fmt.Errorf("AGENT_TOKEN muss mindestens %d Zeichen haben", minAgentTokenLen)
	}
```

(Doc-Kommentar von `Load` um `AGENT_TOKEN (optional, mind. 32 Zeichen)` ergänzen; `fmt` importieren.)

Run: `cd backend && go test ./internal/config/` → PASS.

- [ ] **Step 3: HTTP – failing Tests**

In `backend/internal/httpapi/helpers_test.go`:
- `call` in eine allgemeinere Funktion umbauen:

```go
func call(t *testing.T, srv *httptest.Server, method, path string, body any) response {
	t.Helper()
	return callWith(t, srv, method, path, "", body)
}

// callWith schickt optional ein Agent-Token als Bearer mit.
func callWith(t *testing.T, srv *httptest.Server, method, path, token string, body any) response {
	// bisheriger Rumpf von call, zusätzlich vor dem Senden:
	//   if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
}
```

- und ergänzen:

```go
const agentToken = "test-token-0123456789abcdefghijklmnop"

func newAgentTestServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	testdb.Reset(t, testPool)
	h, err := httpapi.NewRouter(service.New(testPool, time.Now), slog.New(slog.NewTextHandler(io.Discard, nil)), httpapi.WithAgentToken(token))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}
```

`backend/internal/httpapi/agent_auth_test.go`:

```go
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
```

Run: `cd backend && go test ./internal/httpapi/ -run Agent` → FAIL (Kompilierfehler `httpapi.WithAgentToken undefined`).

- [ ] **Step 4: OpenAPI – Security-Schema und `/api/agent/cv`**

In `api/openapi.yaml`:
- unter `tags:` ergänzen:

```yaml
  - name: Agent
    description: "Für Claude-Agenten; verlangt Authorization: Bearer <AGENT_TOKEN>"
```

- unter `components:` (neben `parameters`, `responses`, `schemas`) ergänzen:

```yaml
  securitySchemes:
    agentToken:
      type: http
      scheme: bearer
```

- nach den `/api/v1/cv…`-Pfaden ergänzen:

```yaml
  /api/agent/cv:
    get:
      operationId: agentGetCv
      tags: [Agent]
      summary: Gespeicherter Lebenslauf (404, solange keiner gespeichert ist)
      security: [{ agentToken: [] }]
      responses:
        "200":
          description: Lebenslauf
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Cv" }
        default: { $ref: "#/components/responses/Problem" }
```

Run: `task generate:api` → `AgentGetCvRequestObject`, `AgentGetCv200JSONResponse`.

- [ ] **Step 5: Middleware, Router-Option, Handler**

`backend/internal/httpapi/agent_auth.go`:

```go
package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// agentPrefix ist der Pfadpräfix der Agent-API.
const agentPrefix = "/api/agent/"

// requireAgentToken schützt die Agent-API: Ohne konfiguriertes Token gibt es sie nicht (404),
// sonst ist "Authorization: Bearer <token>" Pflicht (401). Andere Pfade bleiben unberührt.
func requireAgentToken(token string, logger *slog.Logger, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, agentPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		if token == "" {
			writeProblem(w, problem{Type: problemBase + "not-found", Title: "Nicht gefunden",
				Status: http.StatusNotFound, Detail: "Die Agent-API ist nicht eingerichtet"})
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			logger.Warn("agent-api: ungültiges oder fehlendes token", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent"`)
			writeProblem(w, problem{Type: problemBase + "unauthorized", Title: "Nicht angemeldet",
				Status: http.StatusUnauthorized, Detail: "Gültiges Agent-Token erforderlich"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

In `backend/internal/httpapi/router.go`:
- Optionen ergänzen:

```go
// RouterOption konfiguriert optionale Teile des Routers.
type RouterOption func(*routerConfig)

type routerConfig struct {
	agentToken string
}

// WithAgentToken aktiviert die Agent-API (/api/agent/*) mit diesem Bearer-Token.
func WithAgentToken(token string) RouterOption {
	return func(c *routerConfig) { c.agentToken = token }
}
```

- Signatur: `func NewRouter(svc *service.Service, logger *slog.Logger, opts ...RouterOption) (http.Handler, error)`, am Anfang `var cfg routerConfig; for _, o := range opts { o(&cfg) }`.
- Im Validator-Optionsblock ergänzen (Authentifizierung übernimmt `requireAgentToken`):

```go
		Options: openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
```

(Import `github.com/getkin/kin-openapi/openapi3filter`.)
- Rückgabe: `return logRequests(logger, recoverPanics(logger, limitBody(requireAgentToken(cfg.agentToken, logger, mux)))), nil`

In `backend/internal/httpapi/cv.go` die Umwandlung in eine gemeinsame Funktion ziehen und `AgentGetCv` ergänzen:

```go
// cvDTO wandelt den gespeicherten Lebenslauf in das API-Schema; ohne Daten die leere Struktur.
func cvDTO(c service.CV) (Cv, error) {
	out := emptyCv()
	if c.Data == nil {
		return out, nil
	}
	if err := json.Unmarshal(c.Data, &out); err != nil {
		return Cv{}, fmt.Errorf("gespeicherten Lebenslauf lesen: %w", err)
	}
	out.UpdatedAt = &c.UpdatedAt
	return out, nil
}

func (s *Server) GetCv(ctx context.Context, _ GetCvRequestObject) (GetCvResponseObject, error) {
	c, err := s.svc.GetCV(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cvDTO(c)
	if err != nil {
		return nil, err
	}
	return GetCv200JSONResponse(out), nil
}

func (s *Server) AgentGetCv(ctx context.Context, _ AgentGetCvRequestObject) (AgentGetCvResponseObject, error) {
	c, err := s.svc.GetCV(ctx)
	if err != nil {
		return nil, err
	}
	if c.Data == nil {
		return nil, &service.NotFoundError{Resource: "Lebenslauf"}
	}
	out, err := cvDTO(c)
	if err != nil {
		return nil, err
	}
	return AgentGetCv200JSONResponse(out), nil
}
```

(Import `bewerbungsmanager/internal/service`.)

In `backend/cmd/server/main.go`:

```go
	if cfg.AgentToken == "" {
		logger.Info("AGENT_TOKEN nicht gesetzt – Agent-API ist deaktiviert")
	}
	handler, err := httpapi.NewRouter(service.New(pool, time.Now, opts...), logger, httpapi.WithAgentToken(cfg.AgentToken))
```

In `docker-compose.yml` beim Dienst `backend` unter `environment` ergänzen: `AGENT_TOKEN: ${AGENT_TOKEN:-}`.

In `.env.example` ergänzen:

```
# Token für die Agent-API (/api/agent/*), mind. 32 Zeichen, z. B. `openssl rand -hex 32`. Leer = Agent-API aus.
AGENT_TOKEN=
```

- [ ] **Step 6: Tests, Linter, Compose**

Run: `cd backend && go test ./... && golangci-lint run ./... && cd .. && docker compose config -q && echo ok`
Expected: alles grün.

- [ ] **Step 7: Commit**

```bash
git add api/openapi.yaml backend docker-compose.yml .env.example
git commit -m "feat(agent): Agent-API mit Token und GET /api/agent/cv"
```

---

### Task 3: Optimierungs-API (Oberfläche und Agent)

**Files:**
- Modify: `api/openapi.yaml`, `README.md`
- Create: `backend/internal/httpapi/cv_review.go`
- Test: `backend/internal/httpapi/cv_review_test.go`

- [ ] **Step 1: Failing Test schreiben**

`backend/internal/httpapi/cv_review_test.go`:

```go
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
```

Run: `cd backend && go test ./internal/httpapi/ -run CVReview` → FAIL (404).

- [ ] **Step 2: OpenAPI**

In `api/openapi.yaml`:

1. In den CV-Schemas bei allen Pflicht-Strings `pattern: '\S'` ergänzen (verhindert reine Leerzeichen, auch für Agenten): `CvPerson.name`, `CvLink.label`, `CvLink.url`, `CvExperience.role`, `CvExperience.organization`, `CvEducation.degree`, `CvEducation.institution`, `CvSkillGroup.category`, `CvProject.name`, `CvLanguage.language`, `CvLanguage.level`. Beispiel: `name: { type: string, minLength: 1, maxLength: 200, pattern: '\S' }`.

2. Schemas ergänzen:

```yaml
    CvReviewState:
      type: string
      enum: [angefordert, fertig, abgeschlossen]
    CvReview:
      type: object
      description: Optimierungslauf; proposal fehlt, solange er angefordert ist.
      required: [id, state, based_on_updated_at, notes, requested_at]
      properties:
        id: { type: string, format: uuid }
        state: { $ref: "#/components/schemas/CvReviewState" }
        based_on_updated_at: { type: string, format: date-time, description: Stand des Lebenslaufs bei der Anforderung }
        proposal: { $ref: "#/components/schemas/Cv" }
        notes:
          type: array
          items: { type: string }
        requested_at: { type: string, format: date-time }
        completed_at: { type: string, format: date-time }
    CvReviewResult:
      type: object
      required: [proposal, notes]
      properties:
        proposal: { $ref: "#/components/schemas/Cv" }
        notes:
          type: array
          maxItems: 30
          items: { type: string, minLength: 1, maxLength: 500 }
```

3. Pfade ergänzen (nach `/api/v1/cv/pdf` bzw. nach `/api/agent/cv`):

```yaml
  /api/v1/cv/review:
    get:
      operationId: getCvReview
      tags: [Cv]
      summary: Offene Optimierung (angefordert oder fertig); 404, wenn keine offen ist
      responses:
        "200":
          description: Optimierung
          content:
            application/json:
              schema: { $ref: "#/components/schemas/CvReview" }
        default: { $ref: "#/components/responses/Problem" }
    post:
      operationId: requestCvReview
      tags: [Cv]
      summary: Optimierung anfordern (409 ohne gespeicherten Lebenslauf oder bei offener Optimierung)
      responses:
        "201":
          description: Angefordert
          content:
            application/json:
              schema: { $ref: "#/components/schemas/CvReview" }
        default: { $ref: "#/components/responses/Problem" }
    delete:
      operationId: closeCvReview
      tags: [Cv]
      summary: Offene Optimierung abschließen bzw. Anfrage zurückziehen
      responses:
        "204":
          description: Abgeschlossen
        default: { $ref: "#/components/responses/Problem" }
  /api/agent/cv-reviews:
    get:
      operationId: agentListCvReviews
      tags: [Agent]
      summary: Optimierungen in einem Zustand (für den Agenten meist „angefordert“)
      security: [{ agentToken: [] }]
      parameters:
        - name: state
          in: query
          required: true
          schema: { $ref: "#/components/schemas/CvReviewState" }
      responses:
        "200":
          description: Optimierungen
          content:
            application/json:
              schema:
                type: array
                items: { $ref: "#/components/schemas/CvReview" }
        default: { $ref: "#/components/responses/Problem" }
  /api/agent/cv-reviews/{id}:
    parameters:
      - $ref: "#/components/parameters/Id"
    put:
      operationId: agentCompleteCvReview
      tags: [Agent]
      summary: Vorschlag und Hinweise abliefern (409, wenn nicht angefordert)
      security: [{ agentToken: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/CvReviewResult" }
      responses:
        "200":
          description: Abgeliefert
          content:
            application/json:
              schema: { $ref: "#/components/schemas/CvReview" }
        default: { $ref: "#/components/responses/Problem" }
```

Run: `task generate:api`.

- [ ] **Step 3: Handler**

`backend/internal/httpapi/cv_review.go`:

```go
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"

	"bewerbungsmanager/internal/service"
)

func cvReviewDTO(r service.CVReview) (CvReview, error) {
	out := CvReview{
		Id: r.ID, State: CvReviewState(r.State), BasedOnUpdatedAt: r.BasedOnUpdatedAt,
		Notes: r.Notes, RequestedAt: r.RequestedAt, CompletedAt: r.CompletedAt,
	}
	if r.Proposal != nil {
		p := emptyCv()
		if err := json.Unmarshal(r.Proposal, &p); err != nil {
			return CvReview{}, fmt.Errorf("vorschlag lesen: %w", err)
		}
		out.Proposal = &p
	}
	return out, nil
}

func (s *Server) GetCvReview(ctx context.Context, _ GetCvReviewRequestObject) (GetCvReviewResponseObject, error) {
	r, err := s.svc.OpenCVReview(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cvReviewDTO(r)
	if err != nil {
		return nil, err
	}
	return GetCvReview200JSONResponse(out), nil
}

func (s *Server) RequestCvReview(ctx context.Context, _ RequestCvReviewRequestObject) (RequestCvReviewResponseObject, error) {
	r, err := s.svc.RequestCVReview(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cvReviewDTO(r)
	if err != nil {
		return nil, err
	}
	return RequestCvReview201JSONResponse(out), nil
}

func (s *Server) CloseCvReview(ctx context.Context, _ CloseCvReviewRequestObject) (CloseCvReviewResponseObject, error) {
	if err := s.svc.CloseCVReview(ctx); err != nil {
		return nil, err
	}
	return CloseCvReview204Response{}, nil
}

func (s *Server) AgentListCvReviews(ctx context.Context, req AgentListCvReviewsRequestObject) (AgentListCvReviewsResponseObject, error) {
	rs, err := s.svc.ListCVReviews(ctx, string(req.Params.State))
	if err != nil {
		return nil, err
	}
	out := make(AgentListCvReviews200JSONResponse, 0, len(rs))
	for _, r := range rs {
		dto, err := cvReviewDTO(r)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *Server) AgentCompleteCvReview(ctx context.Context, req AgentCompleteCvReviewRequestObject) (AgentCompleteCvReviewResponseObject, error) {
	proposal := req.Body.Proposal
	proposal.UpdatedAt = nil
	data, err := json.Marshal(proposal)
	if err != nil {
		return nil, fmt.Errorf("vorschlag serialisieren: %w", err)
	}
	r, err := s.svc.CompleteCVReview(ctx, req.Id, data, req.Body.Notes)
	if err != nil {
		return nil, err
	}
	out, err := cvReviewDTO(r)
	if err != nil {
		return nil, err
	}
	return AgentCompleteCvReview200JSONResponse(out), nil
}
```

Generierte Feld- und Typnamen (z. B. `Id`, `CompletedAt` als `*time.Time`, `Notes []string`) prüfen und bei Abweichung anpassen.

- [ ] **Step 4: README**

In `README.md` einen kurzen Abschnitt „Agent-API“ ergänzen (bei der Konfiguration bzw. nach `GOTENBERG_URL`): `AGENT_TOKEN` (mind. 32 Zeichen, `openssl rand -hex 32`, in `.env`), Aufruf mit `Authorization: Bearer <token>`, Endpunkte `GET /api/agent/cv`, `GET /api/agent/cv-reviews?state=angefordert`, `PUT /api/agent/cv-reviews/{id}`; ohne Token ist die Agent-API aus (404).

- [ ] **Step 5: Tests, Linter**

Run: `cd backend && go test ./... && golangci-lint run ./...`
Expected: alles grün.

- [ ] **Step 6: Commit**

```bash
git add api/openapi.yaml backend README.md
git commit -m "feat(cv): API für die Lebenslauf-Optimierung"
```

---

### Task 4: Frontend – Client, Vergleich, Abschnitte übernehmen

**Files:**
- Generate: `frontend/src/app/api/**`
- Modify: `frontend/src/app/core/api.ts`, `frontend/src/app/features/cv/cv-form.ts`, `cv-form.spec.ts`
- Create: `frontend/src/app/features/cv/cv-review-model.ts`, `cv-review-model.spec.ts`

- [ ] **Step 1: Client und Fassade**

Run: `task generate:client`

In `frontend/src/app/core/api.ts` (Model-Import um `CvReview` ergänzen):

```ts
  /** Offene Optimierung; 404 (keine offen) zeigt keine Fehlermeldung. */
  getCvReview(): Observable<CvReview> {
    return this.cv.getCvReview(undefined, new HttpContext().set(SILENT_NOT_FOUND, true));
  }
  requestCvReview(): Observable<CvReview> {
    return this.cv.requestCvReview();
  }
  closeCvReview(): Observable<void> {
    return this.cv.closeCvReview();
  }
```

(Muster wie `getCvPhoto`; Signatur des generierten `getCvReview` prüfen.)

- [ ] **Step 2: Failing Tests**

`frontend/src/app/features/cv/cv-review-model.spec.ts`:

```ts
import { Cv } from '../../api/models';
import { changedSections, sectionLines } from './cv-review-model';

const base: Cv = {
  person: { name: 'Erika', headline: 'Entwicklerin', links: [] },
  summary: 'Alt.',
  experience: [{ role: 'Agentin', organization: 'Acme', start: '2020-09', highlights: ['Kundenservice'] }],
  education: [{ degree: 'Ausbildung', institution: 'JET', start: '2012-08', end: '2014-07' }],
  skills: [{ category: 'Frontend', items: ['Angular', 'TypeScript'] }],
  projects: [{ name: 'Join', description: 'Kanban', technologies: ['JS'] }],
  languages: [{ language: 'Englisch', level: 'gut' }],
};

describe('cv-review-model', () => {
  it('meldet nur geänderte Abschnitte', () => {
    const proposal: Cv = { ...base, summary: 'Neu.', skills: [{ category: 'Frontend', items: ['Angular', 'TypeScript', 'RxJS'] }] };
    expect(changedSections(base, proposal)).toEqual(['summary', 'skills']);
  });

  it('ignoriert die Reihenfolge der Schlüssel', () => {
    const reordered = JSON.parse(
      '{"languages":[{"level":"gut","language":"Englisch"}],"projects":[{"technologies":["JS"],"name":"Join","description":"Kanban"}],' +
        '"skills":[{"items":["Angular","TypeScript"],"category":"Frontend"}],"education":[{"end":"2014-07","start":"2012-08","institution":"JET","degree":"Ausbildung"}],' +
        '"experience":[{"highlights":["Kundenservice"],"start":"2020-09","organization":"Acme","role":"Agentin"}],"summary":"Alt.","person":{"links":[],"headline":"Entwicklerin","name":"Erika"}}',
    ) as Cv;
    expect(changedSections(base, reordered)).toEqual([]);
  });

  it('erkennt eine geänderte Berufsbezeichnung', () => {
    expect(changedSections(base, { ...base, person: { ...base.person, headline: 'Junior Frontend-Entwicklerin' } })).toEqual(['headline']);
  });

  it('formatiert Abschnitte als lesbare Zeilen', () => {
    expect(sectionLines(base, 'experience')).toEqual(['09/2020 – heute: Agentin, Acme', '• Kundenservice']);
    expect(sectionLines(base, 'education')).toEqual(['08/2012 – 07/2014: Ausbildung, JET']);
    expect(sectionLines(base, 'skills')).toEqual(['Frontend: Angular, TypeScript']);
    expect(sectionLines(base, 'projects')).toEqual(['Join: Kanban', 'Technologien: JS']);
    expect(sectionLines(base, 'languages')).toEqual(['Englisch: gut']);
    expect(sectionLines(base, 'summary')).toEqual(['Alt.']);
    expect(sectionLines(base, 'headline')).toEqual(['Entwicklerin']);
  });
});
```

In `frontend/src/app/features/cv/cv-form.spec.ts` ergänzen (Import `applySection`):

```ts
  it('übernimmt einen Abschnitt aus einem Vorschlag und markiert das Formular als geändert', () => {
    const form = cvForm(cv);
    const proposal = { ...cv, summary: 'Neues Profil', experience: [{ role: 'Entwickler', organization: 'Neu GmbH', start: '2025', highlights: ['A', 'B'] }] };
    applySection(form, 'experience', proposal);
    applySection(form, 'summary', proposal);
    const out = formToCv(form);
    expect(out.experience).toEqual(proposal.experience);
    expect(out.summary).toBe('Neues Profil');
    expect(out.education).toEqual(cv.education);
    expect(form.dirty).toBe(true);
  });
```

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv-review-model.spec.ts --include src/app/features/cv/cv-form.spec.ts` → FAIL.

- [ ] **Step 3: Implementieren**

`frontend/src/app/features/cv/cv-review-model.ts`:

```ts
import { Cv } from '../../api/models';

/** Abschnitte, die eine Optimierung einzeln ändern kann (Kontaktdaten bleiben unberührt). */
export type CvSection = 'headline' | 'summary' | 'projects' | 'experience' | 'education' | 'skills' | 'languages';

export const SECTION_LABELS: Record<CvSection, string> = {
  headline: 'Berufsbezeichnung',
  summary: 'Profil',
  projects: 'Projekte',
  experience: 'Berufserfahrung',
  education: 'Ausbildung',
  skills: 'Kenntnisse',
  languages: 'Sprachen',
};

export const SECTIONS = Object.keys(SECTION_LABELS) as CvSection[];

export function sectionValue(cv: Cv, section: CvSection): unknown {
  switch (section) {
    case 'headline':
      return cv.person.headline ?? '';
    case 'summary':
      return cv.summary ?? '';
    default:
      return cv[section];
  }
}

/** JSON mit sortierten Schlüsseln, damit die Reihenfolge (Go vs. Browser) keine Rolle spielt. */
function stable(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stable).join(',')}]`;
  }
  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => a.localeCompare(b));
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${stable(v)}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

export function changedSections(current: Cv, proposal: Cv): CvSection[] {
  return SECTIONS.filter((s) => stable(sectionValue(current, s)) !== stable(sectionValue(proposal, s)));
}

function month(p: string | undefined): string {
  if (!p) {
    return 'heute';
  }
  return /^\d{4}-\d{2}$/.test(p) ? `${p.slice(5)}/${p.slice(0, 4)}` : p;
}

/** Lesbare Zeilen eines Abschnitts für die Vorher/Nachher-Ansicht. */
export function sectionLines(cv: Cv, section: CvSection): string[] {
  switch (section) {
    case 'headline':
      return [cv.person.headline ?? ''].filter(Boolean);
    case 'summary':
      return [cv.summary ?? ''].filter(Boolean);
    case 'projects':
      return cv.projects.flatMap((p) => [`${p.name}${p.description ? `: ${p.description}` : ''}`, ...(p.technologies.length ? [`Technologien: ${p.technologies.join(', ')}`] : [])]);
    case 'experience':
      return cv.experience.flatMap((e) => [
        `${month(e.start)} – ${month(e.end)}: ${e.role}, ${e.organization}${e.location ? ` (${e.location})` : ''}`,
        ...e.highlights.map((h) => `• ${h}`),
      ]);
    case 'education':
      return cv.education.flatMap((e) => [`${month(e.start)} – ${month(e.end)}: ${e.degree}, ${e.institution}`, ...(e.details ? [e.details] : [])]);
    case 'skills':
      return cv.skills.map((s) => `${s.category}: ${s.items.join(', ')}`);
    case 'languages':
      return cv.languages.map((l) => `${l.language}: ${l.level}`);
  }
}
```

In `frontend/src/app/features/cv/cv-form.ts` ergänzen (Import `AbstractControl` aus `@angular/forms`, `CvSection` aus `./cv-review-model`):

```ts
function replaceAll<T extends AbstractControl>(list: FormArray<T>, items: T[]): void {
  list.clear();
  items.forEach((item) => list.push(item));
}

/** Übernimmt einen Abschnitt eines Vorschlags ins Formular; gespeichert wird erst mit „Speichern“. */
export function applySection(form: CvForm, section: CvSection, cv: Cv): void {
  const c = form.controls;
  switch (section) {
    case 'headline':
      c.person.controls.headline.setValue(cv.person.headline ?? '');
      break;
    case 'summary':
      c.summary.setValue(cv.summary ?? '');
      break;
    case 'projects':
      replaceAll(c.projects, cv.projects.map(projectGroup));
      break;
    case 'experience':
      replaceAll(c.experience, cv.experience.map(experienceGroup));
      break;
    case 'education':
      replaceAll(c.education, cv.education.map(educationGroup));
      break;
    case 'skills':
      replaceAll(c.skills, cv.skills.map(skillGroup));
      break;
    case 'languages':
      replaceAll(c.languages, cv.languages.map(languageGroup));
      break;
  }
  form.markAsDirty();
}
```

Hinweis: `cv.projects.map(projectGroup)` übergibt den Index als zweites Argument – die Fabriken nehmen nur einen Parameter, das ist unschädlich; bei Lint-/Typfehlern `(p) => projectGroup(p)` schreiben.

- [ ] **Step 4: Tests und Build**

Run: `cd frontend && npx ng test --watch=false && npx ng build`
Expected: alles grün.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app
git commit -m "feat(cv): Vorschläge vergleichen und Abschnitte übernehmen"
```

---

### Task 5: Frontend – Bereich „Mit Claude optimieren“

**Files:**
- Create: `frontend/src/app/features/cv/cv-review.ts`, `cv-review.html`, `cv-review.scss`, `cv-review.spec.ts`
- Modify: `frontend/src/app/features/cv/cv.ts`, `cv.html`, `cv.spec.ts`

- [ ] **Step 1: Failing Test**

`frontend/src/app/features/cv/cv-review.spec.ts`:

```ts
import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { of, throwError } from 'rxjs';
import { Cv, CvReview } from '../../api/models';
import { Api } from '../../core/api';
import { cvForm, formToCv } from './cv-form';
import { CvReviewPanel } from './cv-review';

const saved: Cv = {
  person: { name: 'Erika', headline: 'Entwicklerin', links: [] },
  summary: 'Alt.',
  experience: [],
  education: [],
  skills: [],
  projects: [],
  languages: [],
};
const savedAt = '2026-10-05T10:00:00+02:00';

function ready(overrides: Partial<CvReview> = {}): CvReview {
  return {
    id: 'r1',
    state: 'fertig',
    based_on_updated_at: savedAt,
    requested_at: '2026-10-05T10:01:00+02:00',
    completed_at: '2026-10-05T11:00:00+02:00',
    notes: ['Kennzahlen ergänzen'],
    proposal: { ...saved, summary: 'Neu und geschärft.' },
    ...overrides,
  };
}

async function render(review: CvReview | null, opts: { dirty?: boolean; savedAt?: string } = {}) {
  const api = {
    getCvReview: vi.fn(() => (review ? of(review) : throwError(() => ({ status: 404 })))),
    requestCvReview: vi.fn(() => of(ready({ state: 'angefordert', proposal: undefined, notes: [], completed_at: undefined }))),
    closeCvReview: vi.fn(() => of(undefined)),
  };
  TestBed.configureTestingModule({ imports: [CvReviewPanel], providers: [{ provide: Api, useValue: api }] });
  const snackBar = TestBed.inject(MatSnackBar);
  vi.spyOn(snackBar, 'open');
  const fixture = TestBed.createComponent(CvReviewPanel);
  const form = cvForm(saved);
  fixture.componentRef.setInput('form', form);
  fixture.componentRef.setInput('savedAt', 'savedAt' in opts ? opts.savedAt : savedAt);
  fixture.componentRef.setInput('dirty', opts.dirty ?? false);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  return { api, form, settle, el: fixture.nativeElement as HTMLElement };
}

describe('CvReviewPanel', () => {
  afterEach(() => vi.restoreAllMocks());

  it('fordert eine Optimierung an', async () => {
    const { el, api, settle } = await render(null);
    el.querySelector<HTMLButtonElement>('button.request')!.click();
    await settle();
    expect(api.requestCvReview).toHaveBeenCalled();
    expect(el.textContent).toContain('Angefordert am');
  });

  it('sperrt die Anforderung bei ungespeicherten Änderungen oder ohne gespeicherten Lebenslauf', async () => {
    let r = await render(null, { dirty: true });
    expect(r.el.querySelector<HTMLButtonElement>('button.request')!.disabled).toBe(true);
    TestBed.resetTestingModule();
    r = await render(null, { savedAt: undefined });
    expect(r.el.querySelector<HTMLButtonElement>('button.request')!.disabled).toBe(true);
  });

  it('zeigt Hinweise und Vorher/Nachher und übernimmt einen Abschnitt', async () => {
    const { el, form, settle } = await render(ready());
    expect(el.textContent).toContain('Kennzahlen ergänzen');
    const change = el.querySelector('.change')!;
    expect(change.textContent).toContain('Profil');
    expect(change.textContent).toContain('Alt.');
    expect(change.textContent).toContain('Neu und geschärft.');
    change.querySelector<HTMLButtonElement>('button.apply')!.click();
    await settle();
    expect(formToCv(form).summary).toBe('Neu und geschärft.');
    expect(form.dirty).toBe(true);
    expect(change.querySelector<HTMLButtonElement>('button.apply')!.textContent).toContain('Übernommen');
  });

  it('warnt, wenn der Lebenslauf seit der Anforderung geändert wurde', async () => {
    const { el } = await render(ready({ based_on_updated_at: '2026-10-04T09:00:00+02:00' }));
    expect(el.querySelector('.stale')).not.toBeNull();
  });

  it('meldet, wenn es keine Änderungen gibt', async () => {
    const { el } = await render(ready({ proposal: saved }));
    expect(el.textContent).toContain('keine Änderungen');
  });

  it('schließt die Optimierung ab', async () => {
    const { el, api, settle } = await render(ready());
    el.querySelector<HTMLButtonElement>('button.close')!.click();
    await settle();
    expect(api.closeCvReview).toHaveBeenCalled();
    expect(el.querySelector('button.request')).not.toBeNull();
  });
});
```

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv-review.spec.ts` → FAIL.

- [ ] **Step 2: Komponente**

`frontend/src/app/features/cv/cv-review.ts`:

```ts
import { DatePipe } from '@angular/common';
import { Component, DestroyRef, computed, inject, input, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatSnackBar } from '@angular/material/snack-bar';
import { finalize } from 'rxjs';
import { Cv, CvReview } from '../../api/models';
import { Api } from '../../core/api';
import { applySection, CvForm, formToCv } from './cv-form';
import { changedSections, CvSection, SECTION_LABELS, sectionLines } from './cv-review-model';

@Component({
  selector: 'app-cv-review',
  imports: [DatePipe, MatButtonModule],
  templateUrl: './cv-review.html',
  styleUrl: './cv-review.scss',
})
export class CvReviewPanel {
  private readonly api = inject(Api);
  private readonly snackBar = inject(MatSnackBar);
  private readonly destroyRef = inject(DestroyRef);

  readonly form = input.required<CvForm>();
  /** updated_at des gespeicherten Lebenslaufs; undefined = noch nie gespeichert. */
  readonly savedAt = input<string | undefined>();
  readonly dirty = input(false);

  protected readonly review = signal<CvReview | null>(null);
  protected readonly loading = signal(true);
  protected readonly busy = signal(false);
  protected readonly applied = signal<ReadonlySet<CvSection>>(new Set());
  /** Formularstand beim Laden des Vorschlags – Grundlage für „Bisher“. */
  private readonly baseline = signal<Cv | null>(null);

  protected readonly changes = computed(() => {
    const proposal = this.review()?.proposal;
    const before = this.baseline();
    if (!proposal || !before) {
      return [];
    }
    return changedSections(before, proposal).map((section) => ({
      section,
      label: SECTION_LABELS[section],
      before: sectionLines(before, section),
      after: sectionLines(proposal, section),
    }));
  });

  protected readonly stale = computed(() => {
    const r = this.review();
    const saved = this.savedAt();
    return r?.state === 'fertig' && !!saved && Date.parse(saved) !== Date.parse(r.based_on_updated_at);
  });

  constructor() {
    this.api
      .getCvReview()
      .pipe(
        finalize(() => this.loading.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({ next: (r) => this.show(r), error: () => undefined }); // 404 = keine offene Optimierung.
  }

  protected request(): void {
    this.busy.set(true);
    this.api
      .requestCvReview()
      .pipe(
        finalize(() => this.busy.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (r) => {
          this.show(r);
          this.snackBar.open('Optimierung angefordert', undefined, { duration: 3000 });
        },
        error: () => undefined,
      });
  }

  protected close(): void {
    this.busy.set(true);
    this.api
      .closeCvReview()
      .pipe(
        finalize(() => this.busy.set(false)),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({ next: () => this.show(null), error: () => undefined });
  }

  protected apply(section: CvSection): void {
    const proposal = this.review()?.proposal;
    if (!proposal || this.applied().has(section)) {
      return;
    }
    applySection(this.form(), section, proposal);
    this.applied.update((s) => new Set([...s, section]));
    this.snackBar.open('Übernommen – zum Speichern unten auf „Speichern“ klicken', undefined, { duration: 4000 });
  }

  protected applyAll(): void {
    this.changes().forEach((c) => this.apply(c.section));
  }

  private show(r: CvReview | null): void {
    this.review.set(r);
    this.applied.set(new Set());
    this.baseline.set(r?.proposal ? formToCv(this.form()) : null);
  }
}
```

`frontend/src/app/features/cv/cv-review.html`:

```html
<section class="review">
  <h2>Mit Claude optimieren</h2>
  @if (loading()) {
    <p class="muted">Lade …</p>
  } @else if (review() === null) {
    <p class="muted">
      Claude schärft Formulierungen, Stichpunkte und Schlagworte – nur mit Angaben aus deinem Lebenslauf. Du entscheidest je Abschnitt, was
      übernommen wird.
    </p>
    <div class="review-actions">
      <button mat-stroked-button type="button" class="request" (click)="request()" [disabled]="busy() || !savedAt() || dirty()">
        Mit Claude optimieren
      </button>
      @if (dirty()) {
        <span class="muted">Erst speichern.</span>
      }
    </div>
  } @else if (review()!.state === 'angefordert') {
    <p>Angefordert am {{ review()!.requested_at | date: 'dd.MM.yyyy, HH:mm' }} Uhr. Claude bearbeitet die Anfrage beim nächsten Lauf.</p>
    <button mat-button type="button" class="withdraw" (click)="close()" [disabled]="busy()">Anfrage zurückziehen</button>
  } @else {
    @if (stale()) {
      <p class="stale">Der Lebenslauf wurde seit der Anforderung geändert – prüfe die Vorschläge besonders genau.</p>
    }
    @if (review()!.notes.length) {
      <h3>Hinweise von Claude</h3>
      <ul class="notes">
        @for (n of review()!.notes; track $index) {
          <li>{{ n }}</li>
        }
      </ul>
    }
    @for (c of changes(); track c.section) {
      <div class="change">
        <div class="change-head">
          <h3>{{ c.label }}</h3>
          <button mat-stroked-button type="button" class="apply" (click)="apply(c.section)" [disabled]="applied().has(c.section)">
            {{ applied().has(c.section) ? 'Übernommen' : 'Übernehmen' }}
          </button>
        </div>
        <div class="compare">
          <div class="before">
            <h4>Bisher</h4>
            @for (l of c.before; track $index) {
              <p>{{ l }}</p>
            } @empty {
              <p class="muted">–</p>
            }
          </div>
          <div class="after">
            <h4>Vorschlag</h4>
            @for (l of c.after; track $index) {
              <p>{{ l }}</p>
            } @empty {
              <p class="muted">–</p>
            }
          </div>
        </div>
      </div>
    } @empty {
      <p class="muted">Claude schlägt keine Änderungen vor.</p>
    }
    <div class="review-actions">
      @if (changes().length) {
        <button mat-flat-button type="button" class="apply-all" (click)="applyAll()">Alles übernehmen</button>
      }
      <button mat-button type="button" class="close" (click)="close()" [disabled]="busy()">Abschließen</button>
    </div>
  }
</section>
```

`frontend/src/app/features/cv/cv-review.scss`:

```scss
.review {
  margin-bottom: 32px;
}
.review-actions,
.change-head {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
}
.change-head {
  justify-content: space-between;
}
.change {
  border-left: 3px solid var(--mat-sys-primary);
  margin: 16px 0;
  padding-left: 12px;
}
.compare {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
}
.compare h4 {
  margin: 4px 0;
}
.compare p {
  margin: 0 0 4px;
}
.before {
  color: var(--mat-sys-on-surface-variant);
}
.stale {
  color: var(--mat-sys-error);
}
.notes {
  margin-top: 0;
}
```

- [ ] **Step 3: Einbinden**

In `frontend/src/app/features/cv/cv.ts` `CvReviewPanel` importieren und in `imports` ergänzen.

In `frontend/src/app/features/cv/cv.html` direkt nach `@let f = form()!;` (vor `<form …>`) ergänzen:

```html
  <app-cv-review [form]="f" [savedAt]="updatedAt()" [dirty]="f.dirty" />
```

In `frontend/src/app/features/cv/cv.spec.ts` im `Api`-Mock ergänzen: `getCvReview: vi.fn(() => throwError(() => ({ status: 404 })))`, `requestCvReview: vi.fn()`, `closeCvReview: vi.fn()`.

- [ ] **Step 4: Tests und Build**

Run: `cd frontend && npx ng test --watch=false && npx ng build`
Expected: alles grün.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/app
git commit -m "feat(cv): Bereich „Mit Claude optimieren“"
```

---

### Task 6: Ausrollen und erster Optimierungslauf (Haupt-Agent)

- [ ] **Step 1:** Abschlussreview, Branch mergen (nach Freigabe), pushen, CI abwarten.
- [ ] **Step 2:** Auf dem Pi ein Token erzeugen und in `~/bewerbungsmanager/.env` eintragen, ohne es auszugeben: `ssh marsc@pi.local 'cd ~/bewerbungsmanager && grep -q "^AGENT_TOKEN=" .env || echo "AGENT_TOKEN=$(openssl rand -hex 32)" >> .env'`. Dem User ankündigen, dass Backend und Frontend neu gebaut und neu gestartet werden; dann `git pull && docker compose up -d --build`; Log prüfen (`migrationen angewendet … 4`, keine Meldung „Agent-API ist deaktiviert“).
- [ ] **Step 3:** Prüfen: `GET /api/agent/cv` ohne Token → 401; mit Token (per SSH aus `.env` lesen, nie ins Chat-Protokoll schreiben) → 200.
- [ ] **Step 4:** User bittet, auf der Lebenslauf-Seite „Mit Claude optimieren“ zu klicken. Danach übernimmt der Haupt-Agent die Rolle von R2: `GET /api/agent/cv-reviews?state=angefordert`, `GET /api/agent/cv`, optimierte Fassung nach den Regeln der Spec und den Lebenslauf-Vorgaben (Junior Frontend, keine erfundenen Fakten, KI nur bei KI-Anzeige, Developer Akademie nicht erwähnen) erstellen, Hinweise formulieren, `PUT /api/agent/cv-reviews/{id}`.
- [ ] **Step 5:** User prüft Vorher/Nachher auf der Seite und übernimmt, was passt.
