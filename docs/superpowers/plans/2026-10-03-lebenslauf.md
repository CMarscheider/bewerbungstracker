# Lebenslauf Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Der Lebenslauf wird als strukturierte Daten in der App gespeichert, über `/api/v1/cv` gelesen und geschrieben und auf einer Seite „Lebenslauf“ gepflegt.

**Architecture:** Eine Tabelle `cv` mit genau einer Zeile (`id = 1`) hält den Lebenslauf als `jsonb`. Die Struktur legt allein die OpenAPI-Spec fest (Schema `Cv`); der Request-Validator prüft sie, der Service prüft nur, dass ein Name gesetzt ist, und speichert das JSON unverändert. Das Frontend bildet `Cv` auf ein Reactive Form ab; Listen (Stichpunkte, Skills, Technologien) werden als Zeilen bzw. kommagetrennt bearbeitet.

**Tech Stack:** Go 1.27, PostgreSQL 17, goose, sqlc 1.29 (per Docker), oapi-codegen (strict server), testcontainers; Angular 22 (zoneless, Signals, Reactive Forms, Material), ng-openapi-gen, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-03-agenten-automatisierung-design.md`, Abschnitte „Datenmodell → cv“ und „Oberfläche → Lebenslauf“. Die Vorschau-PDF der Spec kommt erst mit dem PDF-Plan; die Agent-Route `GET /api/agent/cv` mit dem Agent-API-Plan.

**Umgebung (Windows):**
- Go liegt in `C:\Program Files\Go\bin` und fehlt in Bash oft im PATH: vor Go-Befehlen `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`. `task` liegt in `~/go/bin`.
- Docker Desktop muss laufen (sqlc-Generierung und testcontainers).
- Commits ohne `Co-Authored-By`-Zeile.

---

## Dateien

| Datei | Aktion | Verantwortung |
|---|---|---|
| `backend/migrations/00002_cv.sql` | neu | Tabelle `cv` |
| `backend/internal/store/queries/cv.sql` | neu | `GetCV`, `UpsertCV` |
| `backend/internal/store/cv.sql.go`, `models.go` | generiert | sqlc |
| `backend/internal/testdb/testdb.go` | ändern | `Reset` leert auch `cv` |
| `backend/internal/service/cv.go` | neu | Lesen/Speichern, Namensprüfung |
| `backend/internal/service/cv_test.go` | neu | Service-Tests |
| `api/openapi.yaml` | ändern | Tag `Cv`, Pfad `/api/v1/cv`, Schemas `Cv*` |
| `backend/internal/httpapi/api.gen.go` | generiert | oapi-codegen |
| `backend/internal/httpapi/cv.go` | neu | Handler `GetCv`, `SaveCv` |
| `backend/internal/httpapi/cv_test.go` | neu | HTTP-Tests |
| `frontend/src/app/api/**` | generiert | ng-openapi-gen |
| `frontend/src/app/core/api.ts` | ändern | `getCv`, `saveCv` |
| `frontend/src/app/features/cv/cv-form.ts` | neu | Form-Aufbau und Umwandlung Form ↔ `Cv` |
| `frontend/src/app/features/cv/cv-form.spec.ts` | neu | Tests der Umwandlung |
| `frontend/src/app/features/cv/cv.ts`, `.html`, `.scss` | neu | Seite „Lebenslauf“ |
| `frontend/src/app/features/cv/cv.spec.ts` | neu | Komponententest |
| `frontend/src/app/app.routes.ts`, `app.ts`, `app.spec.ts` | ändern | Route `/lebenslauf`, Navigation |

---

### Task 1: Tabelle, Queries und Service

**Files:**
- Create: `backend/migrations/00002_cv.sql`
- Create: `backend/internal/store/queries/cv.sql`
- Modify: `backend/internal/testdb/testdb.go:67`
- Create: `backend/internal/service/cv.go`
- Test: `backend/internal/service/cv_test.go`

- [ ] **Step 1: Failing Test schreiben**

`backend/internal/service/cv_test.go`:

```go
package service_test

import (
	"encoding/json"
	"errors"
	"testing"

	"bewerbungsmanager/internal/domain"
)

func TestGetCVWithoutDataIsEmpty(t *testing.T) {
	svc := newService(t)
	cv, err := svc.GetCV(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cv.Data != nil {
		t.Errorf("erwartet keinen Lebenslauf, bekommen %s", cv.Data)
	}
}

func TestSaveCVStoresAndOverwrites(t *testing.T) {
	svc := newService(t)
	first, err := svc.SaveCV(ctx, json.RawMessage(`{"person":{"name":"Erika Muster"},"skills":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if first.UpdatedAt.IsZero() {
		t.Error("UpdatedAt fehlt")
	}
	if _, err := svc.SaveCV(ctx, json.RawMessage(`{"person":{"name":"Erika Neu"},"skills":["Go"]}`)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetCV(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Person struct{ Name string } `json:"person"`
		Skills []string             `json:"skills"`
	}
	if err := json.Unmarshal(got.Data, &body); err != nil {
		t.Fatal(err)
	}
	if body.Person.Name != "Erika Neu" || len(body.Skills) != 1 {
		t.Errorf("gespeichert: %s", got.Data)
	}
}

func TestSaveCVRequiresName(t *testing.T) {
	svc := newService(t)
	_, err := svc.SaveCV(ctx, json.RawMessage(`{"person":{"name":"   "}}`))
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person.name" {
		t.Fatalf("erwartet ValidationError(person.name), bekommen %v", err)
	}
}
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/service/ -run CV`
Expected: Kompilierfehler `svc.GetCV undefined` bzw. `svc.SaveCV undefined`.

- [ ] **Step 3: Migration anlegen**

`backend/migrations/00002_cv.sql`:

```sql
-- +goose Up
-- Genau ein Lebenslauf; die Struktur legt das OpenAPI-Schema Cv fest.
CREATE TABLE cv (
    id         int PRIMARY KEY CHECK (id = 1),
    data       jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE cv;
```

- [ ] **Step 4: Queries anlegen und sqlc generieren**

`backend/internal/store/queries/cv.sql`:

```sql
-- name: GetCV :one
SELECT data, updated_at FROM cv WHERE id = 1;

-- name: UpsertCV :one
INSERT INTO cv (id, data, updated_at)
VALUES (1, $1, now())
ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = now()
RETURNING data, updated_at;
```

Run: `task generate:sqlc`
Expected: neue Datei `backend/internal/store/cv.sql.go` mit `GetCV(ctx) (GetCVRow, error)` und `UpsertCV(ctx, data []byte) (UpsertCVRow, error)`; `models.go` enthält `type Cv struct`.

- [ ] **Step 5: Reset der Testdatenbank erweitern**

In `backend/internal/testdb/testdb.go` Zeile 67 ersetzen:

```go
	_, err := pool.Exec(context.Background(), "TRUNCATE companies, applications, application_events, cv CASCADE")
```

- [ ] **Step 6: Service implementieren**

`backend/internal/service/cv.go`:

```go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"bewerbungsmanager/internal/domain"
)

// CV ist der gespeicherte Lebenslauf; Data ist nil, solange keiner gespeichert wurde.
type CV struct {
	Data      json.RawMessage
	UpdatedAt time.Time
}

func (s *Service) GetCV(ctx context.Context) (CV, error) {
	r, err := s.queries().GetCV(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return CV{}, nil
	}
	if err != nil {
		return CV{}, err
	}
	return CV{Data: r.Data, UpdatedAt: r.UpdatedAt}, nil
}

// SaveCV ersetzt den Lebenslauf. Die Struktur prüft der OpenAPI-Validator; hier nur der Name.
func (s *Service) SaveCV(ctx context.Context, data json.RawMessage) (CV, error) {
	var head struct {
		Person struct {
			Name string `json:"name"`
		} `json:"person"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return CV{}, &domain.ValidationError{Field: "person", Detail: "kein gültiges JSON-Objekt"}
	}
	if _, err := requireText("person.name", head.Person.Name); err != nil {
		return CV{}, err
	}
	r, err := s.queries().UpsertCV(ctx, data)
	if err != nil {
		return CV{}, err
	}
	return CV{Data: r.Data, UpdatedAt: r.UpdatedAt}, nil
}
```

- [ ] **Step 7: Tests laufen lassen**

Run: `cd backend && go test ./internal/service/`
Expected: `ok  bewerbungsmanager/internal/service` (alle Tests, auch die bestehenden).

- [ ] **Step 8: Commit**

```bash
git add backend/migrations/00002_cv.sql backend/internal/store backend/internal/testdb/testdb.go backend/internal/service/cv.go backend/internal/service/cv_test.go
git commit -m "feat(cv): Tabelle und Service für den Lebenslauf"
```

---

### Task 2: OpenAPI und HTTP-Handler

**Files:**
- Modify: `api/openapi.yaml` (Tag, Pfad, Schemas)
- Generate: `backend/internal/httpapi/api.gen.go`
- Create: `backend/internal/httpapi/cv.go`
- Test: `backend/internal/httpapi/cv_test.go`

- [ ] **Step 1: Failing Test schreiben**

`backend/internal/httpapi/cv_test.go`:

```go
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
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/httpapi/ -run CV`
Expected: FAIL – `GET /api/v1/cv` liefert 404 („Unbekannter Endpunkt“).

- [ ] **Step 3: OpenAPI erweitern**

In `api/openapi.yaml` unter `tags:` nach `Dashboard` ergänzen:

```yaml
  - name: Cv
    description: Lebenslauf
```

Unter `paths:` (nach dem letzten `/api/v1/...`-Pfad, vor `components:`) ergänzen:

```yaml
  /api/v1/cv:
    get:
      operationId: getCv
      tags: [Cv]
      summary: Lebenslauf lesen (leere Struktur, solange keiner gespeichert ist)
      responses:
        "200":
          description: Lebenslauf
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Cv" }
        default: { $ref: "#/components/responses/Problem" }
    put:
      operationId: saveCv
      tags: [Cv]
      summary: Lebenslauf ersetzen
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Cv" }
      responses:
        "200":
          description: Gespeichert
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Cv" }
        default: { $ref: "#/components/responses/Problem" }
```

Unter `components.schemas:` ergänzen:

```yaml
    CvPeriod:
      type: string
      description: Jahr oder Jahr-Monat, z. B. 2024 oder 2024-03
      pattern: '^\d{4}(-(0[1-9]|1[0-2]))?$'
    CvLink:
      type: object
      required: [label, url]
      properties:
        label: { type: string, minLength: 1, maxLength: 100 }
        url: { type: string, minLength: 1, maxLength: 500 }
    CvPerson:
      type: object
      required: [name, links]
      properties:
        name: { type: string, minLength: 1, maxLength: 200 }
        headline: { type: string, maxLength: 200 }
        email: { type: string, maxLength: 200 }
        phone: { type: string, maxLength: 100 }
        location: { type: string, maxLength: 200 }
        links:
          type: array
          maxItems: 10
          items: { $ref: "#/components/schemas/CvLink" }
    CvExperience:
      type: object
      required: [role, organization, start, highlights]
      properties:
        role: { type: string, minLength: 1, maxLength: 200 }
        organization: { type: string, minLength: 1, maxLength: 200 }
        location: { type: string, maxLength: 200 }
        start: { $ref: "#/components/schemas/CvPeriod" }
        end: { $ref: "#/components/schemas/CvPeriod" }
        highlights:
          type: array
          maxItems: 15
          items: { type: string, maxLength: 500 }
    CvEducation:
      type: object
      required: [degree, institution, start]
      properties:
        degree: { type: string, minLength: 1, maxLength: 200 }
        institution: { type: string, minLength: 1, maxLength: 200 }
        start: { $ref: "#/components/schemas/CvPeriod" }
        end: { $ref: "#/components/schemas/CvPeriod" }
        details: { type: string, maxLength: 1000 }
    CvSkillGroup:
      type: object
      required: [category, items]
      properties:
        category: { type: string, minLength: 1, maxLength: 100 }
        items:
          type: array
          maxItems: 40
          items: { type: string, maxLength: 100 }
    CvProject:
      type: object
      required: [name, technologies]
      properties:
        name: { type: string, minLength: 1, maxLength: 200 }
        url: { type: string, maxLength: 500 }
        description: { type: string, maxLength: 2000 }
        technologies:
          type: array
          maxItems: 20
          items: { type: string, maxLength: 100 }
    CvLanguage:
      type: object
      required: [language, level]
      properties:
        language: { type: string, minLength: 1, maxLength: 100 }
        level: { type: string, minLength: 1, maxLength: 100 }
    Cv:
      type: object
      description: Lebenslauf. end fehlt = bis heute. updated_at wird beim Speichern ignoriert.
      required: [person, experience, education, skills, projects, languages]
      properties:
        person: { $ref: "#/components/schemas/CvPerson" }
        summary: { type: string, maxLength: 3000 }
        experience:
          type: array
          maxItems: 30
          items: { $ref: "#/components/schemas/CvExperience" }
        education:
          type: array
          maxItems: 20
          items: { $ref: "#/components/schemas/CvEducation" }
        skills:
          type: array
          maxItems: 20
          items: { $ref: "#/components/schemas/CvSkillGroup" }
        projects:
          type: array
          maxItems: 20
          items: { $ref: "#/components/schemas/CvProject" }
        languages:
          type: array
          maxItems: 10
          items: { $ref: "#/components/schemas/CvLanguage" }
        updated_at: { type: string, format: date-time }
```

- [ ] **Step 4: Server-Code generieren**

Run: `task generate:api`
Expected: `api.gen.go` enthält `type Cv struct`, `GetCvRequestObject`, `SaveCvRequestObject` (mit `Body *SaveCvJSONRequestBody`), `GetCv200JSONResponse`, `SaveCv200JSONResponse`. `go build ./...` schlägt jetzt fehl, weil `Server` `GetCv`/`SaveCv` noch nicht implementiert – das ist erwartet.

- [ ] **Step 5: Handler implementieren**

`backend/internal/httpapi/cv.go`:

```go
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
)

// emptyCv ist die Antwort, solange kein Lebenslauf gespeichert ist; Listen sind leer statt null.
func emptyCv() Cv {
	return Cv{
		Person:     CvPerson{Links: []CvLink{}},
		Experience: []CvExperience{},
		Education:  []CvEducation{},
		Skills:     []CvSkillGroup{},
		Projects:   []CvProject{},
		Languages:  []CvLanguage{},
	}
}

func (s *Server) GetCv(ctx context.Context, _ GetCvRequestObject) (GetCvResponseObject, error) {
	c, err := s.svc.GetCV(ctx)
	if err != nil {
		return nil, err
	}
	out := emptyCv()
	if c.Data != nil {
		if err := json.Unmarshal(c.Data, &out); err != nil {
			return nil, fmt.Errorf("gespeicherten Lebenslauf lesen: %w", err)
		}
		out.UpdatedAt = &c.UpdatedAt
	}
	return GetCv200JSONResponse(out), nil
}

func (s *Server) SaveCv(ctx context.Context, req SaveCvRequestObject) (SaveCvResponseObject, error) {
	body := Cv(*req.Body)
	body.UpdatedAt = nil
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("lebenslauf serialisieren: %w", err)
	}
	c, err := s.svc.SaveCV(ctx, data)
	if err != nil {
		return nil, err
	}
	body.UpdatedAt = &c.UpdatedAt
	return SaveCv200JSONResponse(body), nil
}
```

Hinweis: Falls oapi-codegen `SaveCvJSONRequestBody` als Alias (`= Cv`) erzeugt, ist die Umwandlung `Cv(*req.Body)` überflüssig, aber harmlos.

- [ ] **Step 6: Tests laufen lassen**

Run: `cd backend && go test ./... && golangci-lint run ./...`
Expected: alle Pakete `ok`, Linter ohne Befunde. (Ist `golangci-lint` lokal nicht installiert, übernimmt die CI den Linter.)

- [ ] **Step 7: Commit**

```bash
git add api/openapi.yaml backend/internal/httpapi/api.gen.go backend/internal/httpapi/cv.go backend/internal/httpapi/cv_test.go
git commit -m "feat(cv): API GET/PUT /api/v1/cv"
```

---

### Task 3: Frontend – Client und Form-Umwandlung

**Files:**
- Generate: `frontend/src/app/api/**`
- Modify: `frontend/src/app/core/api.ts`
- Create: `frontend/src/app/features/cv/cv-form.ts`
- Test: `frontend/src/app/features/cv/cv-form.spec.ts`

- [ ] **Step 1: Client generieren**

Run: `task generate:client`
Expected: neue Dateien `frontend/src/app/api/fn/cv/get-cv.ts`, `save-cv.ts`, `services/cv.service.ts`; `models.ts` exportiert `Cv`, `CvPerson`, `CvLink`, `CvExperience`, `CvEducation`, `CvSkillGroup`, `CvProject`, `CvLanguage`, `CvPeriod`.

- [ ] **Step 2: Api-Fassade erweitern**

In `frontend/src/app/core/api.ts`:
- im Model-Import `Cv,` (alphabetisch nach `CompanyPatch,`) ergänzen,
- Service-Import ändern zu `import { ApplicationsService, CompaniesService, CvService, DashboardService } from '../api/services';`,
- im Klassenrumpf nach `private readonly dashboard = inject(DashboardService);` ergänzen:

```ts
  private readonly cv = inject(CvService);
```

- am Ende der Klasse ergänzen:

```ts

  getCv(): Observable<Cv> {
    return this.cv.getCv();
  }
  saveCv(body: Cv): Observable<Cv> {
    return this.cv.saveCv({ body });
  }
```

- [ ] **Step 3: Failing Test für die Umwandlung schreiben**

`frontend/src/app/features/cv/cv-form.spec.ts`:

```ts
import { Cv } from '../../api/models';
import { cvForm, formToCv, fromLines, fromList } from './cv-form';

const cv: Cv = {
  person: { name: 'Erika Muster', email: 'erika@example.com', links: [{ label: 'GitHub', url: 'https://github.com/erika' }] },
  summary: 'Baut gern Oberflächen.',
  experience: [{ role: 'Werkstudentin', organization: 'Acme', start: '2024-03', highlights: ['Komponenten gebaut', 'Tests geschrieben'] }],
  education: [{ degree: 'B.Sc. Informatik', institution: 'FH Bielefeld', start: '2021', end: '2025' }],
  skills: [{ category: 'Frontend', items: ['Angular', 'TypeScript'] }],
  projects: [{ name: 'Tracker', technologies: ['Go', 'Angular'] }],
  languages: [{ language: 'Deutsch', level: 'Muttersprache' }],
  updated_at: '2026-10-03T12:00:00+02:00',
};

describe('cv-form', () => {
  it('fromLines trennt Zeilen und verwirft leere', () => {
    expect(fromLines(' a \n\n b\r\n')).toEqual(['a', 'b']);
  });

  it('fromList trennt an Kommas und verwirft leere', () => {
    expect(fromList('Go, Angular ,, ')).toEqual(['Go', 'Angular']);
  });

  it('übernimmt einen Lebenslauf verlustfrei hin und zurück', () => {
    const { updated_at: _ignored, ...expected } = cv;
    expect(formToCv(cvForm(cv))).toEqual(expected);
  });

  it('lässt leere optionale Felder weg', () => {
    const form = cvForm();
    form.controls.person.controls.name.setValue('  Erika  ');
    form.controls.person.controls.phone.setValue('   ');
    const out = formToCv(form);
    expect(out.person).toEqual({ name: 'Erika', links: [] });
    expect(out.summary).toBeUndefined();
  });

  it('meldet ungültige Zeiträume', () => {
    const form = cvForm(cv);
    form.controls.experience.at(0).controls.start.setValue('März 2024');
    expect(form.invalid).toBe(true);
  });
});
```

- [ ] **Step 4: Test laufen lassen, er muss fehlschlagen**

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv-form.spec.ts`
Expected: FAIL – `Cannot find module './cv-form'`.

- [ ] **Step 5: Umwandlung implementieren**

`frontend/src/app/features/cv/cv-form.ts`:

```ts
import { FormArray, FormControl, FormGroup, Validators } from '@angular/forms';
import { Cv, CvEducation, CvExperience, CvLanguage, CvLink, CvProject, CvSkillGroup } from '../../api/models';

const PERIOD = /^\d{4}(-(0[1-9]|1[0-2]))?$/;

const text = (value = '') => new FormControl(value, { nonNullable: true });
const required = (value = '') => new FormControl(value, { nonNullable: true, validators: [Validators.required] });
const period = (value = '', mandatory = false) =>
  new FormControl(value, { nonNullable: true, validators: mandatory ? [Validators.required, Validators.pattern(PERIOD)] : [Validators.pattern(PERIOD)] });

/** Eine Zeile je Eintrag; leere Zeilen fallen weg. */
export function fromLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter((s) => s !== '');
}

/** Kommagetrennte Liste; leere Einträge fallen weg. */
export function fromList(value: string): string[] {
  return value
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '');
}

/** Getrimmter Text oder undefined, damit leere optionale Felder im JSON fehlen. */
function opt(value: string): string | undefined {
  const t = value.trim();
  return t === '' ? undefined : t;
}

/** Entfernt Schlüssel mit undefined, damit das Ergebnis dem gespeicherten JSON entspricht. */
function compact<T extends object>(obj: T): T {
  return Object.fromEntries(Object.entries(obj).filter(([, v]) => v !== undefined)) as T;
}

export const linkGroup = (l?: CvLink) => new FormGroup({ label: required(l?.label), url: required(l?.url) });

export const experienceGroup = (e?: CvExperience) =>
  new FormGroup({
    role: required(e?.role),
    organization: required(e?.organization),
    location: text(e?.location),
    start: period(e?.start, true),
    end: period(e?.end),
    highlights: text(e?.highlights.join('\n')),
  });

export const educationGroup = (e?: CvEducation) =>
  new FormGroup({
    degree: required(e?.degree),
    institution: required(e?.institution),
    start: period(e?.start, true),
    end: period(e?.end),
    details: text(e?.details),
  });

export const skillGroup = (s?: CvSkillGroup) => new FormGroup({ category: required(s?.category), items: text(s?.items.join(', ')) });

export const projectGroup = (p?: CvProject) =>
  new FormGroup({
    name: required(p?.name),
    url: text(p?.url),
    description: text(p?.description),
    technologies: text(p?.technologies.join(', ')),
  });

export const languageGroup = (l?: CvLanguage) => new FormGroup({ language: required(l?.language), level: required(l?.level) });

export function cvForm(cv?: Cv) {
  return new FormGroup({
    person: new FormGroup({
      name: required(cv?.person.name),
      headline: text(cv?.person.headline),
      email: text(cv?.person.email),
      phone: text(cv?.person.phone),
      location: text(cv?.person.location),
      links: new FormArray((cv?.person.links ?? []).map(linkGroup)),
    }),
    summary: text(cv?.summary),
    experience: new FormArray((cv?.experience ?? []).map(experienceGroup)),
    education: new FormArray((cv?.education ?? []).map(educationGroup)),
    skills: new FormArray((cv?.skills ?? []).map(skillGroup)),
    projects: new FormArray((cv?.projects ?? []).map(projectGroup)),
    languages: new FormArray((cv?.languages ?? []).map(languageGroup)),
  });
}

export type CvForm = ReturnType<typeof cvForm>;

export function formToCv(form: CvForm): Cv {
  const v = form.getRawValue();
  return compact({
    person: compact({
      name: v.person.name.trim(),
      headline: opt(v.person.headline),
      email: opt(v.person.email),
      phone: opt(v.person.phone),
      location: opt(v.person.location),
      links: v.person.links.map((l) => ({ label: l.label.trim(), url: l.url.trim() })),
    }),
    summary: opt(v.summary),
    experience: v.experience.map((e) =>
      compact({
        role: e.role.trim(),
        organization: e.organization.trim(),
        location: opt(e.location),
        start: e.start.trim(),
        end: opt(e.end),
        highlights: fromLines(e.highlights),
      }),
    ),
    education: v.education.map((e) =>
      compact({ degree: e.degree.trim(), institution: e.institution.trim(), start: e.start.trim(), end: opt(e.end), details: opt(e.details) }),
    ),
    skills: v.skills.map((s) => ({ category: s.category.trim(), items: fromList(s.items) })),
    projects: v.projects.map((p) =>
      compact({ name: p.name.trim(), url: opt(p.url), description: opt(p.description), technologies: fromList(p.technologies) }),
    ),
    languages: v.languages.map((l) => ({ language: l.language.trim(), level: l.level.trim() })),
  });
}
```

- [ ] **Step 6: Tests laufen lassen**

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv-form.spec.ts`
Expected: 5 Tests PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app/api frontend/src/app/core/api.ts frontend/src/app/features/cv/cv-form.ts frontend/src/app/features/cv/cv-form.spec.ts
git commit -m "feat(cv): API-Client und Form-Umwandlung für den Lebenslauf"
```

---

### Task 4: Frontend – Seite „Lebenslauf“

**Files:**
- Create: `frontend/src/app/features/cv/cv.ts`, `cv.html`, `cv.scss`
- Test: `frontend/src/app/features/cv/cv.spec.ts`
- Modify: `frontend/src/app/app.routes.ts`, `frontend/src/app/app.ts:14-19`, `frontend/src/app/app.spec.ts:14`

- [ ] **Step 1: Failing Test schreiben**

`frontend/src/app/features/cv/cv.spec.ts`:

```ts
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { Cv as CvModel } from '../../api/models';
import { Api } from '../../core/api';
import { CvPage } from './cv';

const stored: CvModel = {
  person: { name: 'Erika Muster', links: [] },
  experience: [{ role: 'Werkstudentin', organization: 'Acme', start: '2024-03', highlights: ['Komponenten gebaut'] }],
  education: [],
  skills: [],
  projects: [],
  languages: [],
  updated_at: '2026-10-03T12:00:00+02:00',
};

async function render(cv: CvModel = stored) {
  const api = { getCv: vi.fn(() => of(cv)), saveCv: vi.fn((body: CvModel) => of({ ...body, updated_at: '2026-10-03T13:00:00+02:00' })) };
  TestBed.configureTestingModule({ imports: [CvPage], providers: [{ provide: Api, useValue: api }] });
  const fixture = TestBed.createComponent(CvPage);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  return { api, settle, el: fixture.nativeElement as HTMLElement };
}

describe('CvPage', () => {
  afterEach(() => vi.restoreAllMocks());

  it('füllt das Formular mit dem gespeicherten Lebenslauf', async () => {
    const { el } = await render();
    expect(el.querySelector<HTMLInputElement>('input[name="person-name"]')!.value).toBe('Erika Muster');
    expect(el.querySelectorAll('[data-section="experience"] .entry').length).toBe(1);
  });

  it('fügt Einträge hinzu und entfernt sie', async () => {
    const { el, settle } = await render();
    el.querySelector<HTMLButtonElement>('[data-section="experience"] button.add')!.click();
    await settle();
    expect(el.querySelectorAll('[data-section="experience"] .entry').length).toBe(2);
    el.querySelector<HTMLButtonElement>('[data-section="experience"] .entry button.remove')!.click();
    await settle();
    expect(el.querySelectorAll('[data-section="experience"] .entry').length).toBe(1);
  });

  it('speichert den umgewandelten Lebenslauf', async () => {
    const { el, api, settle } = await render();
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit'));
    await settle();
    expect(api.saveCv).toHaveBeenCalledTimes(1);
    const body = api.saveCv.mock.calls[0][0];
    expect(body.person.name).toBe('Erika Muster');
    expect(body.experience[0].highlights).toEqual(['Komponenten gebaut']);
  });

  it('speichert nicht, solange Pflichtfelder fehlen', async () => {
    const { el, api, settle } = await render({ ...stored, person: { name: '', links: [] } });
    el.querySelector<HTMLFormElement>('form')!.dispatchEvent(new Event('submit'));
    await settle();
    expect(api.saveCv).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv.spec.ts`
Expected: FAIL – `Cannot find module './cv'`.

- [ ] **Step 3: Komponente implementieren**

`frontend/src/app/features/cv/cv.ts`:

```ts
import { Component, inject, signal } from '@angular/core';
import { FormArray, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';
import { finalize } from 'rxjs';
import { Api } from '../../core/api';
import { cvForm, CvForm, educationGroup, experienceGroup, formToCv, languageGroup, linkGroup, projectGroup, skillGroup } from './cv-form';

@Component({
  selector: 'app-cv',
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule],
  templateUrl: './cv.html',
  styleUrl: './cv.scss',
})
export class CvPage {
  private readonly api = inject(Api);
  private readonly snackBar = inject(MatSnackBar);

  protected readonly form = signal<CvForm | null>(null);
  protected readonly loadError = signal(false);
  protected readonly saving = signal(false);
  protected readonly updatedAt = signal<string | undefined>(undefined);

  protected readonly newLink = linkGroup;
  protected readonly newExperience = experienceGroup;
  protected readonly newEducation = educationGroup;
  protected readonly newSkill = skillGroup;
  protected readonly newProject = projectGroup;
  protected readonly newLanguage = languageGroup;

  constructor() {
    this.api.getCv().subscribe({
      next: (cv) => {
        this.form.set(cvForm(cv));
        this.updatedAt.set(cv.updated_at);
      },
      error: () => this.loadError.set(true),
    });
  }

  protected add<T extends FormGroup>(list: FormArray<T>, create: () => T): void {
    list.push(create());
  }

  protected remove(list: FormArray, index: number): void {
    list.removeAt(index);
  }

  protected save(): void {
    const form = this.form();
    if (!form || this.saving()) {
      return;
    }
    if (form.invalid) {
      form.markAllAsTouched();
      return;
    }
    this.saving.set(true);
    this.api
      .saveCv(formToCv(form))
      .pipe(finalize(() => this.saving.set(false)))
      .subscribe({
        next: (cv) => {
          this.updatedAt.set(cv.updated_at);
          form.markAsPristine();
          this.snackBar.open('Lebenslauf gespeichert', undefined, { duration: 3000 });
        },
        error: () => undefined, // Meldung zeigt der Interceptor.
      });
  }
}
```

`frontend/src/app/features/cv/cv.html`:

```html
<h1>Lebenslauf</h1>

@if (loadError()) {
  <p class="muted">Konnte nicht geladen werden.</p>
} @else if (form() === null) {
  <p class="muted">Lade …</p>
} @else {
  @let f = form()!;
  <form [formGroup]="f" (ngSubmit)="save()">
    <section formGroupName="person">
      <h2>Person</h2>
      <div class="grid">
        <mat-form-field><mat-label>Name</mat-label><input matInput name="person-name" formControlName="name" maxlength="200" /></mat-form-field>
        <mat-form-field><mat-label>Berufsbezeichnung</mat-label><input matInput formControlName="headline" maxlength="200" /></mat-form-field>
        <mat-form-field><mat-label>E-Mail</mat-label><input matInput type="email" formControlName="email" maxlength="200" /></mat-form-field>
        <mat-form-field><mat-label>Telefon</mat-label><input matInput formControlName="phone" maxlength="100" /></mat-form-field>
        <mat-form-field><mat-label>Wohnort</mat-label><input matInput formControlName="location" maxlength="200" /></mat-form-field>
      </div>
      <div data-section="links">
        <h3>Links</h3>
        @for (g of f.controls.person.controls.links.controls; track g; let i = $index) {
          <div class="entry grid" [formGroup]="g">
            <mat-form-field><mat-label>Bezeichnung</mat-label><input matInput formControlName="label" placeholder="GitHub" /></mat-form-field>
            <mat-form-field><mat-label>URL</mat-label><input matInput formControlName="url" /></mat-form-field>
            <button mat-button type="button" class="remove" (click)="remove(f.controls.person.controls.links, i)">Entfernen</button>
          </div>
        }
        <button mat-button type="button" class="add" (click)="add(f.controls.person.controls.links, newLink)">Link hinzufügen</button>
      </div>
    </section>

    <section>
      <h2>Profil</h2>
      <mat-form-field class="wide">
        <mat-label>Kurzprofil</mat-label>
        <textarea matInput formControlName="summary" rows="4" maxlength="3000"></textarea>
      </mat-form-field>
    </section>

    <section data-section="experience">
      <h2>Berufserfahrung</h2>
      @for (g of f.controls.experience.controls; track g; let i = $index) {
        <div class="entry" [formGroup]="g">
          <div class="grid">
            <mat-form-field><mat-label>Rolle</mat-label><input matInput formControlName="role" /></mat-form-field>
            <mat-form-field><mat-label>Firma</mat-label><input matInput formControlName="organization" /></mat-form-field>
            <mat-form-field><mat-label>Ort</mat-label><input matInput formControlName="location" /></mat-form-field>
            <mat-form-field><mat-label>Von</mat-label><input matInput formControlName="start" placeholder="2024-03" /><mat-hint>JJJJ oder JJJJ-MM</mat-hint></mat-form-field>
            <mat-form-field><mat-label>Bis</mat-label><input matInput formControlName="end" placeholder="leer = heute" /><mat-hint>JJJJ oder JJJJ-MM</mat-hint></mat-form-field>
          </div>
          <mat-form-field class="wide">
            <mat-label>Stichpunkte (eine Zeile je Punkt)</mat-label>
            <textarea matInput formControlName="highlights" rows="4"></textarea>
          </mat-form-field>
          <button mat-button type="button" class="remove" (click)="remove(f.controls.experience, i)">Station entfernen</button>
        </div>
      }
      <button mat-button type="button" class="add" (click)="add(f.controls.experience, newExperience)">Station hinzufügen</button>
    </section>

    <section data-section="education">
      <h2>Ausbildung</h2>
      @for (g of f.controls.education.controls; track g; let i = $index) {
        <div class="entry" [formGroup]="g">
          <div class="grid">
            <mat-form-field><mat-label>Abschluss</mat-label><input matInput formControlName="degree" /></mat-form-field>
            <mat-form-field><mat-label>Einrichtung</mat-label><input matInput formControlName="institution" /></mat-form-field>
            <mat-form-field><mat-label>Von</mat-label><input matInput formControlName="start" placeholder="2021" /><mat-hint>JJJJ oder JJJJ-MM</mat-hint></mat-form-field>
            <mat-form-field><mat-label>Bis</mat-label><input matInput formControlName="end" placeholder="leer = heute" /><mat-hint>JJJJ oder JJJJ-MM</mat-hint></mat-form-field>
          </div>
          <mat-form-field class="wide"><mat-label>Details</mat-label><textarea matInput formControlName="details" rows="2"></textarea></mat-form-field>
          <button mat-button type="button" class="remove" (click)="remove(f.controls.education, i)">Eintrag entfernen</button>
        </div>
      }
      <button mat-button type="button" class="add" (click)="add(f.controls.education, newEducation)">Ausbildung hinzufügen</button>
    </section>

    <section data-section="skills">
      <h2>Kenntnisse</h2>
      @for (g of f.controls.skills.controls; track g; let i = $index) {
        <div class="entry grid" [formGroup]="g">
          <mat-form-field><mat-label>Bereich</mat-label><input matInput formControlName="category" placeholder="Frontend" /></mat-form-field>
          <mat-form-field class="wide"><mat-label>Kenntnisse (kommagetrennt)</mat-label><input matInput formControlName="items" /></mat-form-field>
          <button mat-button type="button" class="remove" (click)="remove(f.controls.skills, i)">Entfernen</button>
        </div>
      }
      <button mat-button type="button" class="add" (click)="add(f.controls.skills, newSkill)">Bereich hinzufügen</button>
    </section>

    <section data-section="projects">
      <h2>Projekte</h2>
      @for (g of f.controls.projects.controls; track g; let i = $index) {
        <div class="entry" [formGroup]="g">
          <div class="grid">
            <mat-form-field><mat-label>Name</mat-label><input matInput formControlName="name" /></mat-form-field>
            <mat-form-field><mat-label>URL</mat-label><input matInput formControlName="url" /></mat-form-field>
            <mat-form-field><mat-label>Technologien (kommagetrennt)</mat-label><input matInput formControlName="technologies" /></mat-form-field>
          </div>
          <mat-form-field class="wide"><mat-label>Beschreibung</mat-label><textarea matInput formControlName="description" rows="3"></textarea></mat-form-field>
          <button mat-button type="button" class="remove" (click)="remove(f.controls.projects, i)">Projekt entfernen</button>
        </div>
      }
      <button mat-button type="button" class="add" (click)="add(f.controls.projects, newProject)">Projekt hinzufügen</button>
    </section>

    <section data-section="languages">
      <h2>Sprachen</h2>
      @for (g of f.controls.languages.controls; track g; let i = $index) {
        <div class="entry grid" [formGroup]="g">
          <mat-form-field><mat-label>Sprache</mat-label><input matInput formControlName="language" /></mat-form-field>
          <mat-form-field><mat-label>Niveau</mat-label><input matInput formControlName="level" placeholder="C1" /></mat-form-field>
          <button mat-button type="button" class="remove" (click)="remove(f.controls.languages, i)">Entfernen</button>
        </div>
      }
      <button mat-button type="button" class="add" (click)="add(f.controls.languages, newLanguage)">Sprache hinzufügen</button>
    </section>

    <div class="actions">
      <button mat-flat-button type="submit" [disabled]="saving()">Speichern</button>
      @if (updatedAt(); as at) {
        <span class="muted">Zuletzt gespeichert: {{ at.slice(0, 10) }}</span>
      }
    </div>
  </form>
}
```

`frontend/src/app/features/cv/cv.scss`:

```scss
section {
  margin-bottom: 32px;
}
.grid {
  align-items: start;
  display: grid;
  gap: 0 16px;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
}
.wide {
  width: 100%;
}
.entry {
  border-left: 3px solid var(--mat-sys-outline-variant);
  margin-bottom: 16px;
  padding-left: 12px;
}
.actions {
  align-items: center;
  display: flex;
  gap: 16px;
}
```

- [ ] **Step 4: Tests der Seite laufen lassen**

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv.spec.ts`
Expected: 4 Tests PASS.

- [ ] **Step 5: Route und Navigation ergänzen**

In `frontend/src/app/app.routes.ts` vor der `'**'`-Zeile ergänzen:

```ts
  { path: 'lebenslauf', title: 'Lebenslauf', loadComponent: () => import('./features/cv/cv').then((m) => m.CvPage) },
```

In `frontend/src/app/app.ts` die Liste `links` um einen Eintrag am Ende ergänzen:

```ts
    { path: '/lebenslauf', label: 'Lebenslauf', exact: false },
```

In `frontend/src/app/app.spec.ts` die Erwartung anpassen:

```ts
    expect(links).toEqual(['Übersicht', 'Bewerbungen', 'Statistik', 'Firmen', 'Lebenslauf']);
```

- [ ] **Step 6: Alle Frontend-Tests und Build**

Run: `cd frontend && npx ng test --watch=false && npx ng build`
Expected: alle Tests PASS, Build ohne Fehler.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app/features/cv frontend/src/app/app.routes.ts frontend/src/app/app.ts frontend/src/app/app.spec.ts
git commit -m "feat(cv): Seite Lebenslauf"
```

---

### Task 5: Auf dem Pi ausrollen und Lebenslauf einspielen

Voraussetzung: Der User hat seinen Lebenslauf geliefert (Text, PDF oder Datei).

**Files:**
- Create (nur Scratchpad, nicht im Repo): `cv.json`

- [ ] **Step 1: Push**

Run: `git push`
Expected: CI startet; abwarten, bis sie grün ist (`"/c/Program Files/GitHub CLI/gh.exe" run watch`).

- [ ] **Step 2: Pi aktualisieren**

Dem User vorher ankündigen, dass die Container auf dem Pi neu gebaut und neu gestartet werden.

Run: `ssh marsc@pi.local 'cd ~/bewerbungsmanager && git pull -q && docker compose up -d --build && docker compose ps --format "{{.Service}}: {{.Status}}"'`
Expected: `backend`, `db`, `frontend` jeweils `Up`; Backend-Log enthält `migrationen angewendet` mit Version 2.

- [ ] **Step 3: Lebenslauf in das `Cv`-Schema übertragen und mit dem User optimieren**

Diesen Schritt führt der Haupt-Agent im Gespräch mit dem User aus, nicht ein Subagent.

1. Die Angaben des Users 1:1 in `cv.json` (Scratchpad) übertragen: Zeiträume als `JJJJ` oder `JJJJ-MM`, laufende Stationen ohne `end`, Stichpunkte als Liste.
2. Optimierte Fassung vorschlagen, Ziel Junior-Stellen als Frontend-Developer bzw. KI-Entwickler: Kurzprofil (3–4 Sätze), Stichpunkte als Wirkung statt Tätigkeit, Kennzahlen nur wenn belegt, Keywords (Angular, TypeScript, LLM, Claude API …) nur wo zutreffend, Reihenfolge nach Relevanz, Projekte hervorheben.
3. Nie Fakten erfinden. Fehlende Angaben (Kennzahlen, Zeiträume, Technologien) als Fragen an den User stellen.
4. Vorher/Nachher zeigen, Änderungswünsche einarbeiten, erst die freigegebene Fassung als `cv.json` speichern.

- [ ] **Step 4: Einspielen und prüfen**

Run: `curl -sS -X PUT -H 'Content-Type: application/json' --data @cv.json -w '\nHTTP %{http_code}\n' http://pi.local:4200/api/v1/cv`
Expected: `HTTP 200` und das JSON mit `updated_at`. Bei `400`: Feld aus der Problem-Antwort korrigieren und wiederholen.

- [ ] **Step 5: Dem User zeigen**

User bittet, `http://pi.local:4200/lebenslauf` zu öffnen und die Angaben zu prüfen; Korrekturen macht er direkt auf der Seite.
