# Lebenslauf-PDF und Foto Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ein Bewerbungsfoto lässt sich hochladen, und der gespeicherte Lebenslauf wird als modernes, zweispaltiges PDF (Schrift Carlito/Calibri-kompatibel) unter `/api/v1/cv/pdf` erzeugt und auf der Lebenslauf-Seite per „PDF ansehen“ geöffnet.

**Architecture:** Neues Go-Paket `internal/documents` rendert aus den CV-Daten eine HTML-Seite (`html/template`, eingebettete Vorlage und Schriften) und lässt sie von **Gotenberg** (eigener Container, Chromium) in ein PDF umwandeln. Der Service orchestriert (CV + Foto laden → HTML → PDF) über ein `PDFConverter`-Interface, das per Option gesetzt wird; ohne Konverter antwortet die API mit 503. Das Foto liegt als JPEG in der Tabelle `cv_photo`; der Browser schneidet es vor dem Hochladen auf 3:4 zu und verkleinert es.

**Tech Stack:** Go 1.27, PostgreSQL 17, goose, sqlc, oapi-codegen (strict), kin-openapi-Validator, testcontainers (Postgres und Gotenberg), Gotenberg 8; Angular 22, ng-openapi-gen, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-03-agenten-automatisierung-design.md`, Abschnitt „Lebenslauf-PDF und Foto“.

**Umgebung (Windows):**
- Vor Go-/task-Befehlen: `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`.
- Docker Desktop muss laufen.
- Commits ohne `Co-Authored-By`- oder sonstige Attributionszeilen.
- Branch: `feature/lebenslauf-pdf` (legt der Controller an).

---

## Dateien

| Datei | Aktion | Verantwortung |
|---|---|---|
| `backend/migrations/00003_cv_photo.sql` | neu | Tabelle `cv_photo` |
| `backend/internal/store/queries/cv_photo.sql` | neu | Foto lesen/speichern/löschen |
| `backend/internal/testdb/testdb.go` | ändern | `EnsureDockerHost` exportieren, `Reset` leert `cv_photo` |
| `backend/internal/service/cv_photo.go` (+ `_test.go`) | neu | Foto-Anwendungsfälle |
| `backend/internal/documents/cv.go` | neu | CV-Datenmodell für Dokumente, `ParseCV`, Dateiname |
| `backend/internal/documents/render.go` (+ `_test.go`) | neu | HTML aus Vorlage |
| `backend/internal/documents/templates/cv.html.tmpl` | neu | Lebenslauf-Vorlage (Design) |
| `backend/internal/documents/fonts/*` | neu | Carlito Regular/Bold/Italic + OFL.txt |
| `backend/internal/documents/gotenberg.go` (+ `_test.go`) | neu | HTTP-Client für Gotenberg |
| `backend/internal/service/service.go` | ändern | Option `WithPDFConverter` |
| `backend/internal/service/errors.go` | ändern | `UnavailableError` |
| `backend/internal/service/cv_pdf.go` (+ `_test.go`) | neu | PDF-Anwendungsfall |
| `api/openapi.yaml` | ändern | `/api/v1/cv/photo`, `/api/v1/cv/pdf` |
| `backend/internal/httpapi/cv_photo.go`, `cv_pdf.go` (+ Tests) | neu | Handler |
| `backend/internal/httpapi/problem.go` | ändern | 503-Abbildung |
| `backend/internal/httpapi/helpers_test.go` | ändern | `newTestServerWith` |
| `backend/internal/config/config.go` (+ Test) | ändern | `GOTENBERG_URL` |
| `backend/cmd/server/main.go` | ändern | Konverter verdrahten |
| `docker-compose.yml` | ändern | Dienst `gotenberg` |
| `frontend/src/app/core/api.ts` | ändern | Foto-Methoden |
| `frontend/src/app/core/image-resizer.ts` | neu | Zuschneiden/Verkleinern im Browser |
| `frontend/src/app/features/cv/cv-photo.ts`, `.html`, `.scss`, `.spec.ts` | neu | Foto-Bereich |
| `frontend/src/app/features/cv/cv.html`, `cv.ts`, `cv.spec.ts` | ändern | Foto-Bereich einbinden, „PDF ansehen“ |

---

### Task 1: Foto – Tabelle, Queries, Service

**Files:**
- Create: `backend/migrations/00003_cv_photo.sql`, `backend/internal/store/queries/cv_photo.sql`, `backend/internal/service/cv_photo.go`
- Modify: `backend/internal/testdb/testdb.go`
- Test: `backend/internal/service/cv_photo_test.go`

- [ ] **Step 1: Failing Test schreiben**

`backend/internal/service/cv_photo_test.go`:

```go
package service_test

import (
	"bytes"
	"errors"
	"testing"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

// jpegHeader reicht für die Typ-Erkennung per http.DetectContentType.
var jpegHeader = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00rest")

func TestCVPhotoLifecycle(t *testing.T) {
	svc := newService(t)
	_, err := svc.GetCVPhoto(ctx)
	var nf *service.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("ohne Foto erwartet NotFoundError, bekommen %v", err)
	}

	if err := svc.SaveCVPhoto(ctx, jpegHeader); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetCVPhoto(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, jpegHeader) || got.UpdatedAt.IsZero() {
		t.Errorf("Foto = %d Bytes, UpdatedAt %v", len(got.Data), got.UpdatedAt)
	}

	if err := svc.DeleteCVPhoto(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteCVPhoto(ctx); err != nil {
		t.Fatalf("zweites Löschen soll nicht fehlschlagen: %v", err)
	}
	if _, err := svc.GetCVPhoto(ctx); !errors.As(err, &nf) {
		t.Fatalf("nach dem Löschen erwartet NotFoundError, bekommen %v", err)
	}
}

func TestSaveCVPhotoRejectsNonJPEG(t *testing.T) {
	svc := newService(t)
	for name, data := range map[string][]byte{
		"leer": nil,
		"png":  []byte("\x89PNG\r\n\x1a\n0000"),
		"text": []byte("hallo"),
	} {
		err := svc.SaveCVPhoto(ctx, data)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) || ve.Field != "photo" {
			t.Errorf("%s: erwartet ValidationError(photo), bekommen %v", name, err)
		}
	}
}
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/service/ -run CVPhoto`
Expected: Kompilierfehler `svc.GetCVPhoto undefined`.

- [ ] **Step 3: Migration**

`backend/migrations/00003_cv_photo.sql`:

```sql
-- +goose Up
-- Genau ein Bewerbungsfoto (JPEG), getrennt vom Lebenslauf-JSON.
CREATE TABLE cv_photo (
    id         int PRIMARY KEY CHECK (id = 1),
    image      bytea NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE cv_photo;
```

- [ ] **Step 4: Queries und sqlc**

`backend/internal/store/queries/cv_photo.sql`:

```sql
-- name: GetCVPhoto :one
SELECT image, updated_at FROM cv_photo WHERE id = 1;

-- name: UpsertCVPhoto :exec
INSERT INTO cv_photo (id, image, updated_at)
VALUES (1, $1, now())
ON CONFLICT (id) DO UPDATE SET image = EXCLUDED.image, updated_at = now();

-- name: DeleteCVPhoto :exec
DELETE FROM cv_photo WHERE id = 1;
```

Run: `task generate:sqlc`
Expected: `backend/internal/store/cv_photo.sql.go` mit `GetCVPhoto(ctx) (GetCVPhotoRow, error)`, `UpsertCVPhoto(ctx, image []byte) error`, `DeleteCVPhoto(ctx) error`.

- [ ] **Step 5: testdb anpassen**

In `backend/internal/testdb/testdb.go`:

1. Den Block mit dem DOCKER_HOST-Kommentar und `os.Setenv` aus `Start` in eine exportierte Funktion verschieben (Kommentar mitnehmen):

```go
// EnsureDockerHost setzt unter Windows DOCKER_HOST auf die Named Pipe von Docker Desktop.
// testcontainers erkennt Docker unter Windows per os.Stat auf die Named Pipe. Nutzen mehrere
// Testpakete parallel Docker, scheitert das mit "All pipe instances are busy" und endet in der
// irreführenden Meldung "rootless Docker is not supported on Windows". Mit gesetztem DOCKER_HOST
// entfällt diese Prüfung; der Docker-Client wartet bei belegter Pipe, statt abzubrechen.
func EnsureDockerHost() error {
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		if err := os.Setenv("DOCKER_HOST", windowsDockerHost); err != nil {
			return fmt.Errorf("DOCKER_HOST setzen: %w", err)
		}
	}
	return nil
}
```

und in `Start` stattdessen aufrufen:

```go
	if err := EnsureDockerHost(); err != nil {
		return nil, "", nil, err
	}
```

2. In `Reset` die Tabellenliste ergänzen:

```go
	_, err := pool.Exec(context.Background(), "TRUNCATE companies, applications, application_events, cv, cv_photo CASCADE")
```

- [ ] **Step 6: Service implementieren**

`backend/internal/service/cv_photo.go`:

```go
package service

import (
	"context"
	"net/http"
	"time"

	"bewerbungsmanager/internal/domain"
)

// Photo ist das gespeicherte Bewerbungsfoto (JPEG).
type Photo struct {
	Data      []byte
	UpdatedAt time.Time
}

func (s *Service) GetCVPhoto(ctx context.Context) (Photo, error) {
	r, err := s.queries().GetCVPhoto(ctx)
	if err != nil {
		return Photo{}, notFoundIfNoRows(err, "Foto")
	}
	return Photo{Data: r.Image, UpdatedAt: r.UpdatedAt}, nil
}

// SaveCVPhoto ersetzt das Foto. Erlaubt ist nur JPEG; der Browser wandelt vor dem Hochladen um.
func (s *Service) SaveCVPhoto(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return &domain.ValidationError{Field: "photo", Detail: "darf nicht leer sein"}
	}
	if http.DetectContentType(data) != "image/jpeg" {
		return &domain.ValidationError{Field: "photo", Detail: "muss ein JPEG-Bild sein"}
	}
	return s.queries().UpsertCVPhoto(ctx, data)
}

// DeleteCVPhoto entfernt das Foto; ohne Foto passiert nichts.
func (s *Service) DeleteCVPhoto(ctx context.Context) error {
	return s.queries().DeleteCVPhoto(ctx)
}
```

- [ ] **Step 7: Tests laufen lassen**

Run: `cd backend && go test ./internal/service/ ./internal/httpapi/`
Expected: beide `ok`.

- [ ] **Step 8: Commit**

```bash
git add backend/migrations/00003_cv_photo.sql backend/internal/store backend/internal/testdb/testdb.go backend/internal/service/cv_photo.go backend/internal/service/cv_photo_test.go
git commit -m "feat(cv): Bewerbungsfoto speichern"
```

---

### Task 2: Foto – API

**Files:**
- Modify: `api/openapi.yaml`
- Generate: `backend/internal/httpapi/api.gen.go`
- Create: `backend/internal/httpapi/cv_photo.go`
- Test: `backend/internal/httpapi/cv_photo_test.go`

- [ ] **Step 1: Failing Test schreiben**

`backend/internal/httpapi/cv_photo_test.go`:

```go
package httpapi_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

var jpegHeader = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00rest")

func putPhoto(t *testing.T, srv *httptest.Server, data []byte) response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/cv/photo", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return response{Status: res.StatusCode, ContentType: res.Header.Get("Content-Type"), Body: body}
}

func TestCVPhotoUploadDownloadDelete(t *testing.T) {
	srv := newTestServer(t)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/photo", nil), http.StatusNotFound, "/problems/not-found")

	expectStatus(t, putPhoto(t, srv, jpegHeader), http.StatusNoContent)

	got := call(t, srv, http.MethodGet, "/api/v1/cv/photo", nil)
	expectStatus(t, got, http.StatusOK)
	if got.ContentType != "image/jpeg" || !bytes.Equal(got.Body, jpegHeader) {
		t.Errorf("GET = %s, %d Bytes", got.ContentType, len(got.Body))
	}

	expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/cv/photo", nil), http.StatusNoContent)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/photo", nil), http.StatusNotFound, "/problems/not-found")
}

func TestCVPhotoRejectsNonJPEG(t *testing.T) {
	srv := newTestServer(t)
	p := expectProblem(t, putPhoto(t, srv, []byte("kein bild")), http.StatusBadRequest, "/problems/validation-error")
	if p["field"] != "photo" {
		t.Errorf("field = %v", p["field"])
	}
}

func TestCVPhotoTooLargeIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	big := append(append([]byte{}, jpegHeader...), make([]byte, 2<<20)...)
	r := putPhoto(t, srv, big)
	if r.Status != http.StatusBadRequest && r.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("Status %d, erwartet 400 oder 413; Body %s", r.Status, r.Body)
	}
}
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/httpapi/ -run CVPhoto`
Expected: FAIL – 404 „Unbekannter Endpunkt“.

- [ ] **Step 3: OpenAPI erweitern**

In `api/openapi.yaml` direkt nach dem Pfad `/api/v1/cv` ergänzen:

```yaml
  /api/v1/cv/photo:
    get:
      operationId: getCvPhoto
      tags: [Cv]
      summary: Bewerbungsfoto (JPEG); 404, solange keines gespeichert ist
      responses:
        "200":
          description: Foto
          content:
            image/jpeg:
              schema: { type: string, format: binary }
        default: { $ref: "#/components/responses/Problem" }
    put:
      operationId: saveCvPhoto
      tags: [Cv]
      summary: Bewerbungsfoto ersetzen (JPEG, max. 1 MB)
      requestBody:
        required: true
        content:
          application/octet-stream:
            schema: { type: string, format: binary }
      responses:
        "204":
          description: Gespeichert
        default: { $ref: "#/components/responses/Problem" }
    delete:
      operationId: deleteCvPhoto
      tags: [Cv]
      summary: Bewerbungsfoto entfernen
      responses:
        "204":
          description: Entfernt
        default: { $ref: "#/components/responses/Problem" }
```

Run: `task generate:api`
Expected: `api.gen.go` enthält `GetCvPhoto200ImagejpegResponse` (Felder `Body io.Reader`, `ContentLength int64`), `SaveCvPhotoRequestObject` (Feld `Body io.Reader`), `SaveCvPhoto204Response`, `DeleteCvPhoto204Response`. Weichen die Namen ab, die generierten verwenden.

- [ ] **Step 4: Handler implementieren**

`backend/internal/httpapi/cv_photo.go`:

```go
package httpapi

import (
	"bytes"
	"context"
	"io"

	"bewerbungsmanager/internal/domain"
)

func (s *Server) GetCvPhoto(ctx context.Context, _ GetCvPhotoRequestObject) (GetCvPhotoResponseObject, error) {
	p, err := s.svc.GetCVPhoto(ctx)
	if err != nil {
		return nil, err
	}
	return GetCvPhoto200ImagejpegResponse{Body: bytes.NewReader(p.Data), ContentLength: int64(len(p.Data))}, nil
}

func (s *Server) SaveCvPhoto(ctx context.Context, req SaveCvPhotoRequestObject) (SaveCvPhotoResponseObject, error) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		// http.MaxBytesReader (router.go) bricht bei mehr als 1 MB ab.
		return nil, &domain.ValidationError{Field: "photo", Detail: "zu groß oder unvollständig (max. 1 MB)"}
	}
	if err := s.svc.SaveCVPhoto(ctx, data); err != nil {
		return nil, err
	}
	return SaveCvPhoto204Response{}, nil
}

func (s *Server) DeleteCvPhoto(ctx context.Context, _ DeleteCvPhotoRequestObject) (DeleteCvPhotoResponseObject, error) {
	if err := s.svc.DeleteCVPhoto(ctx); err != nil {
		return nil, err
	}
	return DeleteCvPhoto204Response{}, nil
}
```

- [ ] **Step 5: Tests und Linter**

Run: `cd backend && go test ./... && golangci-lint run ./...`
Expected: alles `ok`, 0 Linter-Befunde. Falls der zu große Upload schon vom Validator oder Router mit einem anderen Status als 400/413 abgewiesen wird, den tatsächlichen Pfad prüfen und im Report nennen.

- [ ] **Step 6: Commit**

```bash
git add api/openapi.yaml backend/internal/httpapi/api.gen.go backend/internal/httpapi/cv_photo.go backend/internal/httpapi/cv_photo_test.go
git commit -m "feat(cv): API für das Bewerbungsfoto"
```

---

### Task 3: Dokumente – Datenmodell, Vorlage, HTML

**Files:**
- Create: `backend/internal/documents/cv.go`, `render.go`, `templates/cv.html.tmpl`, `fonts/Carlito-Regular.ttf`, `fonts/Carlito-Bold.ttf`, `fonts/Carlito-Italic.ttf`, `fonts/OFL.txt`
- Test: `backend/internal/documents/render_test.go`

- [ ] **Step 1: Schriften herunterladen**

```bash
mkdir -p backend/internal/documents/fonts
for f in Carlito-Regular.ttf Carlito-Bold.ttf Carlito-Italic.ttf OFL.txt; do
  curl -sSfL -o backend/internal/documents/fonts/$f https://raw.githubusercontent.com/google/fonts/main/ofl/carlito/$f
done
ls -la backend/internal/documents/fonts
```

Expected: drei TTF-Dateien (je ca. 600–700 KB) und `OFL.txt` (Lizenz, muss mitgeliefert werden).

- [ ] **Step 2: Failing Test schreiben**

`backend/internal/documents/render_test.go`:

```go
package documents_test

import (
	"strings"
	"testing"

	"bewerbungsmanager/internal/documents"
)

const sampleJSON = `{
  "person": {"name": "Erika Mustermann", "headline": "Junior Frontend-Entwicklerin", "email": "erika@example.com",
             "phone": "0170 1234567", "location": "Espelkamp",
             "links": [{"label": "GitHub", "url": "https://github.com/erika"}, {"label": "Böse", "url": "javascript:alert(1)"}]},
  "summary": "Baut gern <b>Oberflächen</b>.",
  "experience": [
    {"role": "Call Center Agent", "organization": "Acme GmbH", "location": "Bremen", "start": "2020-09", "highlights": ["Kundenservice für Samsung"]},
    {"role": "Mechanikerin", "organization": "Rad & Co", "start": "2015", "end": "2020-08", "highlights": []}
  ],
  "education": [{"degree": "Ausbildung", "institution": "JET GmbH", "start": "2012-08", "end": "2014-07"}],
  "skills": [{"category": "Frontend", "items": ["Angular", "TypeScript"]}],
  "projects": [{"name": "Ring of Fire", "url": "https://ring.example", "description": "Kartenspiel", "technologies": ["Angular", "Firebase"]}],
  "languages": [{"language": "Englisch", "level": "gut"}]
}`

func sample(t *testing.T) documents.CV {
	t.Helper()
	cv, err := documents.ParseCV([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}
	return cv
}

func TestRenderCVContainsContent(t *testing.T) {
	html, err := documents.RenderCV(sample(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{
		"Erika Mustermann", "Junior Frontend-Entwicklerin", "Espelkamp", "erika@example.com",
		"Call Center Agent", "Acme GmbH", "Kundenservice für Samsung", "Ring of Fire", "Firebase",
		"Englisch", "gut", "font-family: Carlito",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("HTML enthält %q nicht", want)
		}
	}
}

func TestRenderCVFormatsPeriods(t *testing.T) {
	s := string(must(documents.RenderCV(sample(t), nil)))
	for _, want := range []string{"09/2020 – heute", "2015 – 08/2020", "08/2012 – 07/2014"} {
		if !strings.Contains(s, want) {
			t.Errorf("Zeitraum %q fehlt", want)
		}
	}
}

func TestRenderCVEscapesAndFiltersLinks(t *testing.T) {
	s := string(must(documents.RenderCV(sample(t), nil)))
	if strings.Contains(s, "<b>Oberflächen</b>") {
		t.Error("HTML aus den Daten wurde nicht maskiert")
	}
	if strings.Contains(s, "javascript:") {
		t.Error("javascript:-Link darf nicht im HTML landen")
	}
	if !strings.Contains(s, `href="https://github.com/erika"`) || !strings.Contains(s, `href="https://ring.example"`) {
		t.Error("https-Links fehlen")
	}
	if !strings.Contains(s, "Böse") {
		t.Error("Bezeichnung eines unsicheren Links soll als Text bleiben")
	}
}

func TestRenderCVEmbedsPhoto(t *testing.T) {
	without := string(must(documents.RenderCV(sample(t), nil)))
	if strings.Contains(without, "data:image/jpeg") {
		t.Error("ohne Foto darf kein Bild eingebettet sein")
	}
	with := string(must(documents.RenderCV(sample(t), []byte("\xff\xd8\xff"))))
	if !strings.Contains(with, `src="data:image/jpeg;base64,/9j/`) {
		t.Error("Foto ist nicht als data-URI eingebettet")
	}
}

func TestCVFileName(t *testing.T) {
	cv := sample(t)
	cv.Person.Name = "Jürgen  Groß-Müller"
	if got := documents.CVFileName(cv); got != "Lebenslauf_Juergen_Gross-Mueller.pdf" {
		t.Errorf("CVFileName = %q", got)
	}
}

func TestFontsAreEmbedded(t *testing.T) {
	fonts := documents.Fonts()
	for _, name := range []string{"Carlito-Regular.ttf", "Carlito-Bold.ttf", "Carlito-Italic.ttf"} {
		if len(fonts[name]) < 100_000 {
			t.Errorf("Schrift %s fehlt oder ist zu klein (%d Bytes)", name, len(fonts[name]))
		}
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}
```

- [ ] **Step 3: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/documents/`
Expected: FAIL – Paket `documents` existiert nicht.

- [ ] **Step 4: Datenmodell**

`backend/internal/documents/cv.go`:

```go
// Package documents erzeugt Bewerbungsunterlagen: HTML aus Vorlagen, PDF über Gotenberg.
package documents

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CV entspricht dem OpenAPI-Schema Cv (ohne updated_at); optionale Felder sind leer statt nil.
type CV struct {
	Person     Person       `json:"person"`
	Summary    string       `json:"summary"`
	Experience []Experience `json:"experience"`
	Education  []Education  `json:"education"`
	Skills     []SkillGroup `json:"skills"`
	Projects   []Project    `json:"projects"`
	Languages  []Language   `json:"languages"`
}

type Person struct {
	Name     string `json:"name"`
	Headline string `json:"headline"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Location string `json:"location"`
	Links    []Link `json:"links"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Experience struct {
	Role         string   `json:"role"`
	Organization string   `json:"organization"`
	Location     string   `json:"location"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	Highlights   []string `json:"highlights"`
}

type Education struct {
	Degree      string `json:"degree"`
	Institution string `json:"institution"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Details     string `json:"details"`
}

type SkillGroup struct {
	Category string   `json:"category"`
	Items    []string `json:"items"`
}

type Project struct {
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Description  string   `json:"description"`
	Technologies []string `json:"technologies"`
}

type Language struct {
	Language string `json:"language"`
	Level    string `json:"level"`
}

// ParseCV liest den gespeicherten Lebenslauf (JSON im Schema Cv).
func ParseCV(data []byte) (CV, error) {
	var cv CV
	if err := json.Unmarshal(data, &cv); err != nil {
		return CV{}, fmt.Errorf("lebenslauf lesen: %w", err)
	}
	return cv, nil
}

var fileNameReplacer = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss")

// CVFileName liefert z. B. "Lebenslauf_Christian_Marscheider.pdf" (nur ASCII, für Content-Disposition).
func CVFileName(cv CV) string {
	name := fileNameReplacer.Replace(strings.Join(strings.Fields(cv.Person.Name), "_"))
	var b strings.Builder
	for _, r := range name {
		if r < 128 && (r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			b.WriteRune(r)
		}
	}
	return "Lebenslauf_" + b.String() + ".pdf"
}
```

- [ ] **Step 5: Renderer**

`backend/internal/documents/render.go`:

```go
package documents

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

//go:embed fonts/*.ttf
var fontFS embed.FS

var cvTemplate = template.Must(template.New("cv.html.tmpl").Funcs(template.FuncMap{
	"period":  period,
	"safeURL": safeURL,
}).ParseFS(templateFS, "templates/cv.html.tmpl"))

type cvView struct {
	CV
	Photo template.URL
}

// RenderCV erzeugt die HTML-Seite des Lebenslaufs; photo ist ein JPEG oder nil.
func RenderCV(cv CV, photo []byte) ([]byte, error) {
	view := cvView{CV: cv}
	if len(photo) > 0 {
		view.Photo = template.URL("data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(photo)) //nolint:gosec // selbst erzeugte data-URI aus geprüftem JPEG
	}
	var buf bytes.Buffer
	if err := cvTemplate.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("lebenslauf-vorlage: %w", err)
	}
	return buf.Bytes(), nil
}

// Fonts liefert die Schriftdateien, die die Vorlage per relativer URL einbindet.
func Fonts() map[string][]byte {
	out := map[string][]byte{}
	entries, _ := fontFS.ReadDir("fonts")
	for _, e := range entries {
		data, err := fontFS.ReadFile("fonts/" + e.Name())
		if err == nil {
			out[e.Name()] = data
		}
	}
	return out
}

// period formatiert "2020-09" → "09/2020", "2024" → "2024"; leeres Ende → "heute".
func period(start, end string) string {
	return formatMonth(start) + " – " + formatMonth(end)
}

func formatMonth(p string) string {
	switch {
	case p == "":
		return "heute"
	case len(p) == 7 && p[4] == '-':
		return p[5:] + "/" + p[:4]
	default:
		return p
	}
}

// safeURL gibt nur http(s)-Links frei; alles andere wird nicht verlinkt.
func safeURL(u string) template.URL {
	l := strings.ToLower(strings.TrimSpace(u))
	if strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") {
		return template.URL(strings.TrimSpace(u)) //nolint:gosec // Schema oben auf http(s) geprüft
	}
	return ""
}
```

- [ ] **Step 6: Vorlage (Design)**

`backend/internal/documents/templates/cv.html.tmpl`:

```html
<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<title>Lebenslauf {{.Person.Name}}</title>
<style>
  @font-face { font-family: Carlito; src: url("Carlito-Regular.ttf"); font-weight: 400; font-style: normal; }
  @font-face { font-family: Carlito; src: url("Carlito-Bold.ttf"); font-weight: 700; font-style: normal; }
  @font-face { font-family: Carlito; src: url("Carlito-Italic.ttf"); font-weight: 400; font-style: italic; }
  :root {
    --side: #12343b; --side-text: #e8f1f2; --side-muted: #9fb8bc;
    --accent: #2a9d8f; --text: #1f2a2e; --muted: #5b6b70; --line: #d9e3e5; --chip: #e6f4f2;
    --side-width: 66mm;
  }
  @page { size: A4; margin: 12mm 0; }
  * { box-sizing: border-box; }
  html {
    -webkit-print-color-adjust: exact; print-color-adjust: exact;
    background: linear-gradient(90deg, var(--side) 0 var(--side-width), #fff var(--side-width) 100%);
  }
  body { margin: 0; font-family: Carlito, Calibri, sans-serif; font-size: 10.5pt; line-height: 1.4; color: var(--text); }
  .page { display: grid; grid-template-columns: var(--side-width) 1fr; }
  aside { color: var(--side-text); padding: 0 8mm; }
  main { padding: 0 13mm 0 11mm; }

  .photo { display: block; width: 38mm; height: 38mm; margin: 0 auto 7mm; border-radius: 50%; object-fit: cover;
           border: 1.2mm solid rgba(255, 255, 255, .85); }
  aside h2 { color: #fff; font-size: 10pt; letter-spacing: .14em; text-transform: uppercase; margin: 7mm 0 3mm;
             padding-bottom: 1.5mm; border-bottom: .3mm solid rgba(255, 255, 255, .25); }
  .contact { list-style: none; margin: 0; padding: 0; }
  .contact li { display: flex; align-items: flex-start; gap: 2.5mm; margin-bottom: 2.2mm; word-break: break-word; }
  .contact svg { flex: none; width: 3.6mm; height: 3.6mm; margin-top: .6mm; fill: var(--accent); }
  .contact a { color: var(--side-text); text-decoration: none; }
  .skill-group { margin-bottom: 3.5mm; break-inside: avoid; }
  .skill-group h3 { font-size: 9pt; font-weight: 700; color: var(--side-muted); margin: 0 0 1.5mm; text-transform: uppercase; letter-spacing: .08em; }
  .chips { display: flex; flex-wrap: wrap; gap: 1.5mm; }
  .chip { font-size: 9pt; padding: .6mm 2.4mm; border-radius: 3mm; background: rgba(255, 255, 255, .12); color: #fff; }
  .lang { display: flex; justify-content: space-between; margin-bottom: 1.5mm; }
  .lang span:last-child { color: var(--side-muted); }

  header { margin-bottom: 6mm; }
  header h1 { font-size: 26pt; line-height: 1.1; margin: 0; font-weight: 700; color: var(--side); letter-spacing: -.01em; }
  header p { margin: 1.5mm 0 0; font-size: 13pt; color: var(--accent); font-weight: 700; }
  main h2 { display: flex; align-items: center; gap: 3mm; font-size: 11pt; letter-spacing: .14em; text-transform: uppercase;
            color: var(--side); margin: 7mm 0 3.5mm; break-after: avoid; }
  main h2::after { content: ""; flex: 1; height: .4mm; background: linear-gradient(90deg, var(--accent), var(--line)); }
  .summary { margin: 0; color: var(--text); }

  .project { margin-bottom: 4mm; break-inside: avoid; }
  .project-head { display: flex; justify-content: space-between; gap: 4mm; align-items: baseline; }
  .project h3 { font-size: 11.5pt; margin: 0; color: var(--side); }
  .project a { font-size: 9pt; color: var(--accent); text-decoration: none; white-space: nowrap; }
  .project p { margin: 1mm 0 1.5mm; }
  .project .chip { background: var(--chip); color: var(--side); }

  .timeline { position: relative; margin: 0; padding: 0 0 0 6mm; list-style: none; }
  .timeline::before { content: ""; position: absolute; left: 1.2mm; top: 1.5mm; bottom: 1.5mm; width: .4mm; background: var(--line); }
  .timeline li { position: relative; margin-bottom: 4mm; break-inside: avoid; }
  .timeline li::before { content: ""; position: absolute; left: -6mm; top: 1.4mm; width: 2.8mm; height: 2.8mm; border-radius: 50%;
                         background: #fff; border: .7mm solid var(--accent); }
  .entry-head { display: flex; justify-content: space-between; gap: 4mm; align-items: baseline; }
  .entry-head h3 { font-size: 11pt; margin: 0; color: var(--side); }
  .when { font-size: 9.5pt; color: var(--muted); white-space: nowrap; }
  .where { color: var(--muted); font-size: 10pt; }
  .timeline ul { margin: 1.2mm 0 0; padding-left: 4mm; }
  .timeline ul li { margin: 0 0 .6mm; break-inside: auto; }
  .timeline ul li::before { display: none; }
  .details { margin: 1mm 0 0; color: var(--muted); font-style: italic; }
</style>
</head>
<body>
<div class="page">
  <aside>
    {{if .Photo}}<img class="photo" src="{{.Photo}}" alt="Foto">{{end}}
    <h2>Kontakt</h2>
    <ul class="contact">
      {{with .Person.Location}}<li><svg viewBox="0 0 24 24"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5a2.5 2.5 0 1 1 0-5 2.5 2.5 0 0 1 0 5z"/></svg><span>{{.}}</span></li>{{end}}
      {{with .Person.Phone}}<li><svg viewBox="0 0 24 24"><path d="M6.62 10.79a15.05 15.05 0 0 0 6.59 6.59l2.2-2.2a1 1 0 0 1 1.02-.24c1.12.37 2.33.57 3.57.57a1 1 0 0 1 1 1V20a1 1 0 0 1-1 1A17 17 0 0 1 3 4a1 1 0 0 1 1-1h3.5a1 1 0 0 1 1 1c0 1.25.2 2.45.57 3.57a1 1 0 0 1-.25 1.02l-2.2 2.2z"/></svg><span>{{.}}</span></li>{{end}}
      {{with .Person.Email}}<li><svg viewBox="0 0 24 24"><path d="M20 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/></svg><span>{{.}}</span></li>{{end}}
      {{range .Person.Links}}<li><svg viewBox="0 0 24 24"><path d="M3.9 12a3.1 3.1 0 0 1 3.1-3.1h4V7H7a5 5 0 0 0 0 10h4v-1.9H7A3.1 3.1 0 0 1 3.9 12zM8 13h8v-2H8v2zm9-6h-4v1.9h4a3.1 3.1 0 0 1 0 6.2h-4V17h4a5 5 0 0 0 0-10z"/></svg>{{with safeURL .URL}}<a href="{{.}}">{{$.LinkText .}}</a>{{else}}<span>{{.Label}}</span>{{end}}</li>{{end}}
    </ul>

    {{if .Skills}}<h2>Kenntnisse</h2>
    {{range .Skills}}<div class="skill-group"><h3>{{.Category}}</h3><div class="chips">{{range .Items}}<span class="chip">{{.}}</span>{{end}}</div></div>{{end}}{{end}}

    {{if .Languages}}<h2>Sprachen</h2>
    {{range .Languages}}<div class="lang"><span>{{.Language}}</span><span>{{.Level}}</span></div>{{end}}{{end}}
  </aside>

  <main>
    <header>
      <h1>{{.Person.Name}}</h1>
      {{with .Person.Headline}}<p>{{.}}</p>{{end}}
    </header>

    {{with .Summary}}<h2>Profil</h2><p class="summary">{{.}}</p>{{end}}

    {{if .Projects}}<h2>Projekte</h2>
    {{range .Projects}}<div class="project">
      <div class="project-head"><h3>{{.Name}}</h3>{{with safeURL .URL}}<a href="{{.}}">Live ansehen ↗</a>{{end}}</div>
      {{with .Description}}<p>{{.}}</p>{{end}}
      {{if .Technologies}}<div class="chips">{{range .Technologies}}<span class="chip">{{.}}</span>{{end}}</div>{{end}}
    </div>{{end}}{{end}}

    {{if .Experience}}<h2>Berufserfahrung</h2>
    <ul class="timeline">{{range .Experience}}<li>
      <div class="entry-head"><h3>{{.Role}}</h3><span class="when">{{period .Start .End}}</span></div>
      <div class="where">{{.Organization}}{{with .Location}} · {{.}}{{end}}</div>
      {{if .Highlights}}<ul>{{range .Highlights}}<li>{{.}}</li>{{end}}</ul>{{end}}
    </li>{{end}}</ul>{{end}}

    {{if .Education}}<h2>Ausbildung</h2>
    <ul class="timeline">{{range .Education}}<li>
      <div class="entry-head"><h3>{{.Degree}}</h3><span class="when">{{period .Start .End}}</span></div>
      <div class="where">{{.Institution}}</div>
      {{with .Details}}<p class="details">{{.}}</p>{{end}}
    </li>{{end}}</ul>{{end}}
  </main>
</div>
</body>
</html>
```

Die Vorlage nutzt `$.LinkText` für die Anzeige eines Links (ohne `https://`). Dafür in `render.go` ergänzen:

```go
// LinkText zeigt eine URL ohne Schema und abschließenden Schrägstrich, z. B. "github.com/erika".
func (cvView) LinkText(u template.URL) string {
	s := string(u)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	return strings.TrimSuffix(s, "/")
}
```

- [ ] **Step 7: Tests laufen lassen**

Run: `cd backend && go test ./internal/documents/ && golangci-lint run ./...`
Expected: alle Tests PASS, 0 Linter-Befunde.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/documents
git commit -m "feat(documents): HTML-Vorlage für den Lebenslauf mit Carlito"
```

---

### Task 4: Gotenberg-Client

**Files:**
- Create: `backend/internal/documents/gotenberg.go`
- Test: `backend/internal/documents/gotenberg_test.go`

- [ ] **Step 1: Failing Test schreiben**

`backend/internal/documents/gotenberg_test.go`:

```go
package documents_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"bewerbungsmanager/internal/documents"
	"bewerbungsmanager/internal/testdb"
)

func TestGotenbergSendsFilesAndOptions(t *testing.T) {
	var files []string
	var fields = map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/chromium/convert/html" {
			t.Errorf("Pfad = %s", r.URL.Path)
		}
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if p.FileName() != "" {
				files = append(files, p.FileName())
			} else {
				v, _ := io.ReadAll(p)
				fields[p.FormName()] = string(v)
			}
		}
		_, _ = w.Write([]byte("%PDF-1.7 fake"))
	}))
	defer srv.Close()

	pdf, err := documents.NewGotenberg(srv.URL).Convert(context.Background(), []byte("<html></html>"), map[string][]byte{"a.ttf": {1}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("Antwort = %q", pdf)
	}
	if len(files) != 2 || files[0] != "index.html" || files[1] != "a.ttf" {
		t.Errorf("Dateien = %v", files)
	}
	if fields["printBackground"] != "true" || fields["preferCssPageSize"] != "true" {
		t.Errorf("Felder = %v", fields)
	}
}

func TestGotenbergUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "kaputt", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	_, err := documents.NewGotenberg(srv.URL).Convert(context.Background(), []byte("x"), nil)
	if !errors.Is(err, documents.ErrUnavailable) {
		t.Fatalf("erwartet ErrUnavailable, bekommen %v", err)
	}

	_, err = documents.NewGotenberg("http://127.0.0.1:1").Convert(context.Background(), []byte("x"), nil)
	if !errors.Is(err, documents.ErrUnavailable) {
		t.Fatalf("nicht erreichbar: erwartet ErrUnavailable, bekommen %v", err)
	}
}

// TestGotenbergRendersCV erzeugt mit einem echten Gotenberg-Container ein PDF.
// Mit CV_PDF_OUT=<pfad> wird das Ergebnis zum Ansehen gespeichert.
func TestGotenbergRendersCV(t *testing.T) {
	if testing.Short() {
		t.Skip("braucht Docker")
	}
	if err := testdb.EnsureDockerHost(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ctr, err := testcontainers.Run(ctx, "gotenberg/gotenberg:8",
		testcontainers.WithExposedPorts("3000/tcp"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/health").WithPort("3000/tcp")),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	url, err := ctr.PortEndpoint(ctx, "3000/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}

	html, err := documents.RenderCV(sample(t), samplePhoto(t))
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := documents.NewGotenberg(url).Convert(ctx, html, documents.Fonts())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || len(pdf) < 10_000 {
		t.Fatalf("kein plausibles PDF (%d Bytes)", len(pdf))
	}
	if out := os.Getenv("CV_PDF_OUT"); out != "" {
		if err := os.WriteFile(out, pdf, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func samplePhoto(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 60, 80))
	for y := range 80 {
		for x := range 60 {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: 120, B: uint8(y * 3), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/documents/ -run Gotenberg`
Expected: Kompilierfehler `documents.NewGotenberg undefined`.

- [ ] **Step 3: Client implementieren**

`backend/internal/documents/gotenberg.go`:

```go
package documents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
	"time"
)

// ErrUnavailable: Gotenberg ist nicht erreichbar oder überlastet.
var ErrUnavailable = errors.New("pdf-dienst nicht erreichbar")

// maxPDFBytes begrenzt die gelesene Antwort.
const maxPDFBytes = 20 << 20

// Gotenberg wandelt HTML über den Chromium-Endpunkt von Gotenberg in PDF um.
type Gotenberg struct {
	url    string
	client *http.Client
}

func NewGotenberg(url string) *Gotenberg {
	return &Gotenberg{url: strings.TrimRight(url, "/"), client: &http.Client{Timeout: 60 * time.Second}}
}

// Convert schickt index.html und die Zusatzdateien (z. B. Schriften) und liefert das PDF.
// Seitengröße und Ränder legt das CSS fest (@page).
func (g *Gotenberg) Convert(ctx context.Context, html []byte, assets map[string][]byte) ([]byte, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := addFile(w, "index.html", html); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(assets))
	for name := range assets {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := addFile(w, name, assets[name]); err != nil {
			return nil, err
		}
	}
	for k, v := range map[string]string{
		"printBackground": "true", "preferCssPageSize": "true",
		"marginTop": "0", "marginBottom": "0", "marginLeft": "0", "marginRight": "0",
	} {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url+"/forms/chromium/convert/html", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err) //nolint:errorlint // Ursache nur als Text
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: status %d: %s", ErrUnavailable, res.StatusCode, msg)
		}
		return nil, fmt.Errorf("gotenberg: status %d: %s", res.StatusCode, msg)
	}
	return io.ReadAll(io.LimitReader(res.Body, maxPDFBytes))
}

func addFile(w *multipart.Writer, name string, data []byte) error {
	part, err := w.CreateFormFile("files", name)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}
```

Hinweis: Die Reihenfolge der Formfelder (`map`) ist zufällig; Gotenberg ist das egal.

- [ ] **Step 4: Tests laufen lassen**

Run: `cd backend && go test ./internal/documents/ -count=1 -v -run Gotenberg`
Expected: 3 Tests PASS (der Container-Test lädt beim ersten Mal das Gotenberg-Image, ca. 1–2 Minuten).

- [ ] **Step 5: Ergebnis ansehen**

Run: `cd backend && CV_PDF_OUT="$TEMP/cv-test.pdf" go test ./internal/documents/ -count=1 -run TestGotenbergRendersCV`
Dann das PDF `$TEMP/cv-test.pdf` öffnen bzw. mit dem Read-Tool ansehen und im Report beschreiben: zweispaltig, Seitenleiste petrol durchgehend, rundes Foto, Chips, Zeitleiste, Schrift Carlito. Auffälligkeiten (abgeschnittene Inhalte, fehlende Hintergrundfarbe) beheben, bevor committet wird.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/documents/gotenberg.go backend/internal/documents/gotenberg_test.go backend/go.mod backend/go.sum
git commit -m "feat(documents): Gotenberg-Client mit Integrationstest"
```

---

### Task 5: PDF-Anwendungsfall, API und Betrieb

**Files:**
- Modify: `backend/internal/service/service.go`, `errors.go`
- Create: `backend/internal/service/cv_pdf.go`, `cv_pdf_test.go`
- Modify: `api/openapi.yaml`, `backend/internal/httpapi/problem.go`, `helpers_test.go`
- Create: `backend/internal/httpapi/cv_pdf.go`, `cv_pdf_test.go`
- Modify: `backend/internal/config/config.go`, `config_test.go`, `backend/cmd/server/main.go`, `docker-compose.yml`

- [ ] **Step 1: Failing Service-Test schreiben**

`backend/internal/service/cv_pdf_test.go`:

```go
package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"bewerbungsmanager/internal/documents"
	"bewerbungsmanager/internal/service"
	"bewerbungsmanager/internal/testdb"
)

type fakeConverter struct {
	html   []byte
	assets map[string][]byte
	err    error
}

func (f *fakeConverter) Convert(_ context.Context, html []byte, assets map[string][]byte) ([]byte, error) {
	f.html, f.assets = html, assets
	if f.err != nil {
		return nil, f.err
	}
	return []byte("%PDF-fake"), nil
}

func newServiceWith(t *testing.T, opts ...service.Option) *service.Service {
	t.Helper()
	testdb.Reset(t, testPool)
	return service.New(testPool, time.Now, opts...)
}

func saveSampleCV(t *testing.T, svc *service.Service) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{
		"person": map[string]any{"name": "Erika Mustermann", "links": []any{}},
		"experience": []any{}, "education": []any{}, "skills": []any{}, "projects": []any{}, "languages": []any{},
	})
	if _, err := svc.SaveCV(ctx, data); err != nil {
		t.Fatal(err)
	}
}

func TestCVPDFRendersWithConverter(t *testing.T) {
	conv := &fakeConverter{}
	svc := newServiceWith(t, service.WithPDFConverter(conv))
	saveSampleCV(t, svc)
	if err := svc.SaveCVPhoto(ctx, jpegHeader); err != nil {
		t.Fatal(err)
	}

	pdf, name, err := svc.CVPDF(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(pdf) != "%PDF-fake" || name != "Lebenslauf_Erika_Mustermann.pdf" {
		t.Errorf("pdf=%q name=%q", pdf, name)
	}
	if !strings.Contains(string(conv.html), "Erika Mustermann") || !strings.Contains(string(conv.html), "data:image/jpeg;base64,") {
		t.Error("HTML ohne Name oder Foto an den Konverter übergeben")
	}
	if _, ok := conv.assets["Carlito-Regular.ttf"]; !ok {
		t.Error("Schriften wurden nicht mitgeschickt")
	}
}

func TestCVPDFWithoutCVIsNotFound(t *testing.T) {
	svc := newServiceWith(t, service.WithPDFConverter(&fakeConverter{}))
	_, _, err := svc.CVPDF(ctx)
	var nf *service.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFoundError, bekommen %v", err)
	}
}

func TestCVPDFUnavailable(t *testing.T) {
	var ue *service.UnavailableError

	svc := newServiceWith(t) // ohne Konverter
	saveSampleCV(t, svc)
	if _, _, err := svc.CVPDF(ctx); !errors.As(err, &ue) {
		t.Fatalf("ohne Konverter: erwartet UnavailableError, bekommen %v", err)
	}

	svc = newServiceWith(t, service.WithPDFConverter(&fakeConverter{err: documents.ErrUnavailable}))
	saveSampleCV(t, svc)
	if _, _, err := svc.CVPDF(ctx); !errors.As(err, &ue) {
		t.Fatalf("Gotenberg weg: erwartet UnavailableError, bekommen %v", err)
	}
}
```

- [ ] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./internal/service/ -run CVPDF`
Expected: Kompilierfehler `service.Option undefined` / `svc.CVPDF undefined`.

- [ ] **Step 3: Option und Fehler**

In `backend/internal/service/service.go` die Struktur und `New` ersetzen:

```go
// PDFConverter wandelt HTML (mit Zusatzdateien wie Schriften) in ein PDF um.
type PDFConverter interface {
	Convert(ctx context.Context, html []byte, assets map[string][]byte) ([]byte, error)
}

// Service bündelt alle Anwendungsfälle.
type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
	pdf  PDFConverter
}

// Option konfiguriert optionale Abhängigkeiten.
type Option func(*Service)

// WithPDFConverter aktiviert die PDF-Erzeugung.
func WithPDFConverter(c PDFConverter) Option { return func(s *Service) { s.pdf = c } }

// New erzeugt einen Service; now ist in Produktion time.Now.
func New(pool *pgxpool.Pool, now func() time.Time, opts ...Option) *Service {
	s := &Service{pool: pool, now: now}
	for _, o := range opts {
		o(s)
	}
	return s
}
```

In `backend/internal/service/errors.go` nach `ConflictError` ergänzen:

```go
// UnavailableError: ein benötigter Dienst fehlt oder ist nicht erreichbar (HTTP 503).
type UnavailableError struct{ Detail string }

func (e *UnavailableError) Error() string { return e.Detail }
```

- [ ] **Step 4: Anwendungsfall**

`backend/internal/service/cv_pdf.go`:

```go
package service

import (
	"context"
	"errors"

	"bewerbungsmanager/internal/documents"
)

// CVPDF erzeugt den gespeicherten Lebenslauf als PDF und liefert dazu den Dateinamen.
func (s *Service) CVPDF(ctx context.Context) ([]byte, string, error) {
	cv, err := s.GetCV(ctx)
	if err != nil {
		return nil, "", err
	}
	if cv.Data == nil {
		return nil, "", &NotFoundError{Resource: "Lebenslauf"}
	}
	doc, err := documents.ParseCV(cv.Data)
	if err != nil {
		return nil, "", err
	}
	var photo []byte
	p, err := s.GetCVPhoto(ctx)
	var nf *NotFoundError
	switch {
	case err == nil:
		photo = p.Data
	case !errors.As(err, &nf):
		return nil, "", err
	}
	html, err := documents.RenderCV(doc, photo)
	if err != nil {
		return nil, "", err
	}
	if s.pdf == nil {
		return nil, "", &UnavailableError{Detail: "PDF-Erzeugung ist nicht eingerichtet (GOTENBERG_URL fehlt)"}
	}
	pdf, err := s.pdf.Convert(ctx, html, documents.Fonts())
	if errors.Is(err, documents.ErrUnavailable) {
		return nil, "", &UnavailableError{Detail: "PDF-Dienst ist gerade nicht erreichbar"}
	}
	if err != nil {
		return nil, "", err
	}
	return pdf, documents.CVFileName(doc), nil
}
```

Run: `cd backend && go test ./internal/service/`
Expected: `ok`.

- [ ] **Step 5: Failing HTTP-Test schreiben**

In `backend/internal/httpapi/helpers_test.go` `newTestServer` so umbauen, dass es eine Variante mit Optionen gibt:

```go
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
```

`backend/internal/httpapi/cv_pdf_test.go`:

```go
package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"bewerbungsmanager/internal/service"
)

type fakeConverter struct{}

func (fakeConverter) Convert(context.Context, []byte, map[string][]byte) ([]byte, error) {
	return []byte("%PDF-fake"), nil
}

func TestCVPDF(t *testing.T) {
	srv := newTestServerWith(t, service.WithPDFConverter(fakeConverter{}))
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/pdf", nil), http.StatusNotFound, "/problems/not-found")

	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/cv/pdf", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("Status %d, Content-Type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	cd := res.Header.Get("Content-Disposition")
	if !strings.HasPrefix(cd, "inline") || !strings.Contains(cd, `filename="Lebenslauf_Erika_Muster.pdf"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestCVPDFWithoutConverterIsUnavailable(t *testing.T) {
	srv := newTestServer(t)
	expectStatus(t, call(t, srv, http.MethodPut, "/api/v1/cv", sampleCV()), http.StatusOK)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/pdf", nil), http.StatusServiceUnavailable, "/problems/service-unavailable")
}
```

Run: `cd backend && go test ./internal/httpapi/ -run CVPDF`
Expected: FAIL – 404 „Unbekannter Endpunkt“.

- [ ] **Step 6: OpenAPI, Handler, Problem**

In `api/openapi.yaml` nach `/api/v1/cv/photo` ergänzen:

```yaml
  /api/v1/cv/pdf:
    get:
      operationId: getCvPdf
      tags: [Cv]
      summary: Lebenslauf als PDF (404 ohne gespeicherten Lebenslauf, 503 wenn der PDF-Dienst fehlt)
      responses:
        "200":
          description: PDF
          headers:
            Content-Disposition:
              schema: { type: string }
          content:
            application/pdf:
              schema: { type: string, format: binary }
        default: { $ref: "#/components/responses/Problem" }
```

Run: `task generate:api`
Expected: `GetCvPdf200ApplicationpdfResponse` mit `Body io.Reader`, `ContentLength int64` und `Headers GetCvPdf200ResponseHeaders` (Feld `ContentDisposition string`). Abweichende Namen übernehmen.

`backend/internal/httpapi/cv_pdf.go`:

```go
package httpapi

import (
	"bytes"
	"context"
)

func (s *Server) GetCvPdf(ctx context.Context, _ GetCvPdfRequestObject) (GetCvPdfResponseObject, error) {
	pdf, name, err := s.svc.CVPDF(ctx)
	if err != nil {
		return nil, err
	}
	return GetCvPdf200ApplicationpdfResponse{
		Body:          bytes.NewReader(pdf),
		ContentLength: int64(len(pdf)),
		Headers:       GetCvPdf200ResponseHeaders{ContentDisposition: `inline; filename="` + name + `"`},
	}, nil
}
```

In `backend/internal/httpapi/problem.go` in `problemFor`:
- in der `var`-Liste `unavailable *service.UnavailableError` ergänzen,
- vor der letzten `return`-Zeile einen Fall ergänzen:

```go
	case errors.As(err, &unavailable):
		return problem{Type: problemBase + "service-unavailable", Title: "Dienst nicht verfügbar",
			Status: http.StatusServiceUnavailable, Detail: unavailable.Detail}
```

Run: `cd backend && go test ./... && golangci-lint run ./...`
Expected: alles `ok`, 0 Befunde.

- [ ] **Step 7: Konfiguration und Verdrahtung**

In `backend/internal/config/config.go`:

```go
// Config enthält alle Einstellungen des Servers.
type Config struct {
	DatabaseURL  string
	Port         string
	GotenbergURL string // optional; leer = keine PDF-Erzeugung
}

// Load liest DATABASE_URL (Pflicht), PORT (Standard 8080) und GOTENBERG_URL (optional).
func Load(getenv func(string) string) (Config, error) {
	c := Config{DatabaseURL: getenv("DATABASE_URL"), Port: getenv("PORT"), GotenbergURL: getenv("GOTENBERG_URL")}
```

(Rest von `Load` unverändert.) In `backend/internal/config/config_test.go` einen Test ergänzen:

```go
func TestLoadReadsGotenbergURL(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://x", "GOTENBERG_URL": "http://gotenberg:3000"}
	c, err := config.Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.GotenbergURL != "http://gotenberg:3000" {
		t.Errorf("GotenbergURL = %q", c.GotenbergURL)
	}
}
```

(Paketnamen und Import an die bestehende `config_test.go` anpassen.)

In `backend/cmd/server/main.go` die Zeile mit `httpapi.NewRouter(...)` ersetzen durch:

```go
	var opts []service.Option
	if cfg.GotenbergURL != "" {
		opts = append(opts, service.WithPDFConverter(documents.NewGotenberg(cfg.GotenbergURL)))
	} else {
		logger.Warn("GOTENBERG_URL nicht gesetzt – PDF-Erzeugung ist deaktiviert")
	}
	handler, err := httpapi.NewRouter(service.New(pool, time.Now, opts...), logger)
```

und `"bewerbungsmanager/internal/documents"` importieren.

In `docker-compose.yml`:
- neuen Dienst vor `backend` ergänzen:

```yaml
  gotenberg:
    image: gotenberg/gotenberg:8
    restart: unless-stopped
```

- beim Dienst `backend` unter `environment` ergänzen: `GOTENBERG_URL: http://gotenberg:3000`, und unter `depends_on` ergänzen:

```yaml
      gotenberg:
        condition: service_started
```

- [ ] **Step 8: Alles prüfen**

Run: `cd backend && go test ./... -count=1 && golangci-lint run ./... && cd .. && docker compose config -q && echo compose-ok`
Expected: alles `ok`, `compose-ok`.

- [ ] **Step 9: Commit**

```bash
git add api/openapi.yaml backend docker-compose.yml
git commit -m "feat(cv): Lebenslauf als PDF über Gotenberg"
```

---

### Task 6: Frontend – Foto und „PDF ansehen“

**Files:**
- Generate: `frontend/src/app/api/**`
- Modify: `frontend/src/app/core/api.ts`
- Create: `frontend/src/app/core/image-resizer.ts`
- Create: `frontend/src/app/features/cv/cv-photo.ts`, `cv-photo.html`, `cv-photo.scss`, `cv-photo.spec.ts`
- Modify: `frontend/src/app/features/cv/cv.ts`, `cv.html`, `cv.scss`, `cv.spec.ts`

- [ ] **Step 1: Client generieren, Fassade erweitern**

Run: `task generate:client`
Expected: `fn/cv/get-cv-photo.ts`, `save-cv-photo.ts`, `delete-cv-photo.ts`, `get-cv-pdf.ts`; `CvService` hat `getCvPhoto()`, `saveCvPhoto({ body })`, `deleteCvPhoto()`.

In `frontend/src/app/core/api.ts` am Ende der Klasse ergänzen:

```ts
  getCvPhoto(): Observable<Blob> {
    return this.cv.getCvPhoto();
  }
  saveCvPhoto(image: Blob): Observable<void> {
    return this.cv.saveCvPhoto({ body: image });
  }
  deleteCvPhoto(): Observable<void> {
    return this.cv.deleteCvPhoto();
  }
```

Weichen Rückgabetypen oder Parameter des generierten Clients ab (z. B. `body` als `Blob`), die generierten Typen verwenden.

- [ ] **Step 2: Bild verkleinern (ohne Test – jsdom hat kein Canvas)**

`frontend/src/app/core/image-resizer.ts`:

```ts
import { Injectable } from '@angular/core';

/** Schneidet ein Bild mittig auf 3:4 zu und verkleinert es auf 600×800 Pixel (JPEG). */
@Injectable({ providedIn: 'root' })
export class ImageResizer {
  async toPortraitJpeg(file: Blob, width = 600, height = 800, quality = 0.85): Promise<Blob> {
    const bitmap = await createImageBitmap(file);
    const target = width / height;
    const source = bitmap.width / bitmap.height;
    const sw = source > target ? bitmap.height * target : bitmap.width;
    const sh = source > target ? bitmap.height : bitmap.width / target;
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      throw new Error('Canvas nicht verfügbar');
    }
    ctx.drawImage(bitmap, (bitmap.width - sw) / 2, (bitmap.height - sh) / 2, sw, sh, 0, 0, width, height);
    bitmap.close();
    return new Promise((resolve, reject) =>
      canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('Bild konnte nicht umgewandelt werden'))), 'image/jpeg', quality),
    );
  }
}
```

- [ ] **Step 3: Failing Test für den Foto-Bereich**

`frontend/src/app/features/cv/cv-photo.spec.ts`:

```ts
import { TestBed } from '@angular/core/testing';
import { MatSnackBar } from '@angular/material/snack-bar';
import { of, throwError } from 'rxjs';
import { Api } from '../../core/api';
import { ImageResizer } from '../../core/image-resizer';
import { CvPhoto } from './cv-photo';

async function render(photo: Blob | null) {
  const api = {
    getCvPhoto: vi.fn(() => (photo ? of(photo) : throwError(() => ({ status: 404 })))),
    saveCvPhoto: vi.fn(() => of(undefined)),
    deleteCvPhoto: vi.fn(() => of(undefined)),
  };
  const resizer = { toPortraitJpeg: vi.fn(async () => new Blob(['jpeg'], { type: 'image/jpeg' })) };
  URL.createObjectURL = vi.fn(() => 'blob:foto');
  URL.revokeObjectURL = vi.fn();
  TestBed.configureTestingModule({
    imports: [CvPhoto],
    providers: [
      { provide: Api, useValue: api },
      { provide: ImageResizer, useValue: resizer },
    ],
  });
  const snackBar = TestBed.inject(MatSnackBar);
  vi.spyOn(snackBar, 'open');
  const fixture = TestBed.createComponent(CvPhoto);
  const settle = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  await settle();
  return { api, resizer, snackBar, settle, el: fixture.nativeElement as HTMLElement };
}

function choose(el: HTMLElement, file: File) {
  const input = el.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, 'files', { value: [file] });
  input.dispatchEvent(new Event('change'));
}

describe('CvPhoto', () => {
  afterEach(() => vi.restoreAllMocks());

  it('zeigt das vorhandene Foto', async () => {
    const { el } = await render(new Blob(['x'], { type: 'image/jpeg' }));
    expect(el.querySelector<HTMLImageElement>('img')!.src).toBe('blob:foto');
    expect(el.querySelector('button.remove')).not.toBeNull();
  });

  it('zeigt einen Platzhalter ohne Foto', async () => {
    const { el } = await render(null);
    expect(el.querySelector('img')).toBeNull();
    expect(el.textContent).toContain('Noch kein Foto');
  });

  it('verkleinert und lädt ein gewähltes Bild hoch', async () => {
    const { el, api, resizer, settle } = await render(null);
    choose(el, new File(['raw'], 'foto.png', { type: 'image/png' }));
    await settle();
    await settle();
    expect(resizer.toPortraitJpeg).toHaveBeenCalled();
    expect(api.saveCvPhoto).toHaveBeenCalledTimes(1);
    expect(el.querySelector<HTMLImageElement>('img')!.src).toBe('blob:foto');
  });

  it('lehnt Nicht-Bilder ab', async () => {
    const { el, api, snackBar, settle } = await render(null);
    choose(el, new File(['%PDF'], 'lebenslauf.pdf', { type: 'application/pdf' }));
    await settle();
    expect(api.saveCvPhoto).not.toHaveBeenCalled();
    expect(snackBar.open).toHaveBeenCalledWith('Bitte ein Bild (JPG oder PNG) wählen', 'OK', { duration: 4000 });
  });

  it('entfernt das Foto', async () => {
    const { el, api, settle } = await render(new Blob(['x'], { type: 'image/jpeg' }));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    el.querySelector<HTMLButtonElement>('button.remove')!.click();
    await settle();
    expect(api.deleteCvPhoto).toHaveBeenCalled();
    expect(el.querySelector('img')).toBeNull();
  });
});
```

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv-photo.spec.ts`
Expected: FAIL – `Cannot find module './cv-photo'`.

- [ ] **Step 4: Foto-Komponente**

`frontend/src/app/features/cv/cv-photo.ts`:

```ts
import { Component, DestroyRef, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatSnackBar } from '@angular/material/snack-bar';
import { finalize } from 'rxjs';
import { Api } from '../../core/api';
import { ImageResizer } from '../../core/image-resizer';

@Component({
  selector: 'app-cv-photo',
  imports: [MatButtonModule],
  templateUrl: './cv-photo.html',
  styleUrl: './cv-photo.scss',
})
export class CvPhoto {
  private readonly api = inject(Api);
  private readonly resizer = inject(ImageResizer);
  private readonly snackBar = inject(MatSnackBar);

  protected readonly url = signal<string | null>(null);
  protected readonly busy = signal(false);

  constructor() {
    inject(DestroyRef).onDestroy(() => this.setUrl(null));
    this.api.getCvPhoto().subscribe({
      next: (blob) => this.setUrl(URL.createObjectURL(blob)),
      error: () => undefined, // 404 = noch kein Foto; andere Fehler meldet der Interceptor.
    });
  }

  protected async choose(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) {
      return;
    }
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      this.snackBar.open('Bitte ein Bild (JPG oder PNG) wählen', 'OK', { duration: 4000 });
      return;
    }
    this.busy.set(true);
    let jpeg: Blob;
    try {
      jpeg = await this.resizer.toPortraitJpeg(file);
    } catch {
      this.busy.set(false);
      this.snackBar.open('Das Bild konnte nicht gelesen werden', 'OK', { duration: 4000 });
      return;
    }
    this.api
      .saveCvPhoto(jpeg)
      .pipe(finalize(() => this.busy.set(false)))
      .subscribe({
        next: () => this.setUrl(URL.createObjectURL(jpeg)),
        error: () => undefined,
      });
  }

  protected remove(): void {
    if (!window.confirm('Foto entfernen?')) {
      return;
    }
    this.api.deleteCvPhoto().subscribe({ next: () => this.setUrl(null), error: () => undefined });
  }

  private setUrl(next: string | null): void {
    const old = this.url();
    if (old) {
      URL.revokeObjectURL(old);
    }
    this.url.set(next);
  }
}
```

`frontend/src/app/features/cv/cv-photo.html`:

```html
<div class="photo-box">
  @if (url(); as src) {
    <img [src]="src" alt="Bewerbungsfoto" />
  } @else {
    <div class="placeholder muted">Noch kein Foto</div>
  }
  <div class="photo-actions">
    <input #fileInput type="file" accept="image/jpeg,image/png,image/webp" (change)="choose($event)" hidden />
    <button mat-stroked-button type="button" class="upload" (click)="fileInput.click()" [disabled]="busy()">
      {{ url() ? 'Foto ersetzen' : 'Foto hochladen' }}
    </button>
    @if (url()) {
      <button mat-button type="button" class="remove" (click)="remove()" [disabled]="busy()">Foto entfernen</button>
    }
    <span class="muted hint">Wird auf 3:4 zugeschnitten.</span>
  </div>
</div>
```

`frontend/src/app/features/cv/cv-photo.scss`:

```scss
.photo-box {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}
img,
.placeholder {
  aspect-ratio: 3 / 4;
  border-radius: 8px;
  width: 120px;
}
img {
  object-fit: cover;
}
.placeholder {
  align-items: center;
  border: 2px dashed var(--mat-sys-outline-variant);
  display: flex;
  justify-content: center;
  text-align: center;
}
.photo-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.hint {
  font-size: 0.85em;
}
```

Run: `cd frontend && npx ng test --watch=false --include src/app/features/cv/cv-photo.spec.ts`
Expected: 5 Tests PASS.

- [ ] **Step 5: In die Lebenslauf-Seite einbauen, „PDF ansehen“**

In `frontend/src/app/features/cv/cv.ts`:
- `import { CvPhoto } from './cv-photo';` und `CvPhoto` in `imports` der Komponente ergänzen.

In `frontend/src/app/features/cv/cv.html`:
- direkt nach `<h1>Lebenslauf</h1>` ergänzen:

```html
<section class="photo-section">
  <h2>Foto</h2>
  <app-cv-photo />
</section>
```

- den Block `<div class="actions">…</div>` ersetzen durch:

```html
    <div class="actions">
      <button mat-flat-button type="submit" [disabled]="saving()">Speichern</button>
      @if (updatedAt()) {
        <a mat-stroked-button class="pdf" href="/api/v1/cv/pdf" target="_blank" rel="noopener" [class.disabled]="f.dirty" [attr.aria-disabled]="f.dirty">PDF ansehen</a>
      }
      @if (f.dirty) {
        <span class="muted">Ungespeicherte Änderungen – vor „PDF ansehen“ speichern.</span>
      } @else if (updatedAt(); as at) {
        <span class="muted">Zuletzt gespeichert: {{ at | date: 'dd.MM.yyyy' }}</span>
      }
    </div>
```

In `frontend/src/app/features/cv/cv.scss` ergänzen:

```scss
.pdf.disabled {
  opacity: 0.5;
  pointer-events: none;
}
```

In `frontend/src/app/features/cv/cv.spec.ts`:
- im `render`-Mock von `Api` ergänzen: `getCvPhoto: vi.fn(() => throwError(() => ({ status: 404 })))` (Import `throwError` aus `rxjs`),
- zwei Tests ergänzen:

```ts
  it('verlinkt das PDF, sobald ein Lebenslauf gespeichert ist', async () => {
    const { el } = await render();
    const link = el.querySelector<HTMLAnchorElement>('a.pdf')!;
    expect(link.getAttribute('href')).toBe('/api/v1/cv/pdf');
    expect(link.getAttribute('target')).toBe('_blank');
  });

  it('sperrt „PDF ansehen“ bei ungespeicherten Änderungen', async () => {
    const { el, settle } = await render();
    const input = el.querySelector<HTMLInputElement>('input[name="person-name"]')!;
    input.value = 'Erika Neu';
    input.dispatchEvent(new Event('input'));
    await settle();
    expect(el.querySelector('a.pdf')!.getAttribute('aria-disabled')).toBe('true');
    expect(el.textContent).toContain('Ungespeicherte Änderungen');
  });
```

- [ ] **Step 6: Alle Tests und Build**

Run: `cd frontend && npx ng test --watch=false && npx ng build`
Expected: alle Tests PASS, Build ohne Fehler.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app
git commit -m "feat(cv): Foto hochladen und PDF ansehen"
```

---

### Task 7: Ausrollen und Sichtprüfung (Haupt-Agent)

- [ ] **Step 1:** Branch mergen (nach Freigabe durch den User), `git push`, CI abwarten (`"/c/Program Files/GitHub CLI/gh.exe" run watch`).
- [ ] **Step 2:** Dem User ankündigen, dass auf dem Pi ein neuer Container (Gotenberg, ca. 1,5 GB Image) geladen und die App neu gestartet wird. Dann: `ssh marsc@pi.local 'cd ~/bewerbungsmanager && git pull -q && docker compose up -d --build && docker compose ps --format "{{.Service}}: {{.Status}}"'` (Pull kann einige Minuten dauern; im Hintergrund ausführen).
- [ ] **Step 3:** `curl -s -o "$TEMP/cv.pdf" -w "%{http_code} %{time_total}s\n" http://pi.local:4200/api/v1/cv/pdf` – Expected `200`; PDF mit dem Read-Tool ansehen und Layout prüfen (Seitenleiste, Foto-Platzhalter ohne Foto, Umbruch).
- [ ] **Step 4:** User bitten, auf `http://pi.local:4200/lebenslauf` das Foto hochzuladen und „PDF ansehen“ zu öffnen; Design-Wünsche aufnehmen.
