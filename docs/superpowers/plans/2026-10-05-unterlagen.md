# Unterlagen (Plan 3b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Für eine freigegebene Stelle („Unterlagen erstellen“) schreibt die Routine R2 Anschreiben, Profil-Satz, Schwerpunkte und Mailtext; der Pi rendert Anschreiben + Lebenslauf als ein PDF und legt – wenn eine Bewerbungsadresse bekannt ist – selbst einen Gmail-Entwurf mit PDF-Anhang per IMAP an. Gesendet wird nie automatisch.

**Architecture:**
- Migration `00006` ergänzt `applications` um `documents_state`/`documents_error`/`gmail_draft_at` und legt `application_documents` an (eine aktuelle Version je Bewerbung).
- `internal/documents` bekommt `RenderApplication`: ein HTML-Dokument mit Anschreiben-Seite (gleiche Seitenleiste wie der Lebenslauf) und danach dem Lebenslauf; Profil-Satz ersetzt das Profil, Schwerpunkte ordnen die Kenntnisse (nur vorhandene, nie neue), Sprache `de`/`en` steuert Überschriften und Datumsformat.
- Neues Paket `internal/mail` baut die MIME-Nachricht und legt sie per IMAP (`github.com/emersion/go-imap/v2`) im Ordner mit Attribut `\Drafts` ab (Flag `\Draft`). Kein SMTP, kein Senden.
- Service-Zustände: `keine` → (Oberfläche) `angefordert` → (Agent liefert Text, PDF gerendert) `erstellt` → (Entwurf angelegt) `entwurf_angelegt`; ohne Bewerbungsadresse → `portal`; Renderfehler → `fehler`; Gotenberg nicht erreichbar → 503, Zustand bleibt `angefordert`.

**Tech Stack:** Go 1.27, PostgreSQL 17, goose, sqlc, oapi-codegen (strict), kin-openapi, Gotenberg 8, go-imap v2; Angular 22, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-03-agenten-automatisierung-design.md` – „Datenmodell → applications/application_documents“, „Agent-API“, „PDF-Erzeugung“, „Oberfläche → Detail“, „Routinen → R2“, „Fehlerbehandlung“. **Abweichung (vom User am 2026-10-05 entschieden):** Den Gmail-Entwurf legt der Pi per IMAP an (Gmail-Konnektor kann große Anhänge nicht praktikabel übergeben). `PATCH …/gmail` und `GET …/documents/pdf` für den Agenten entfallen; `gmail_thread_id`/`processed_mails` folgen in Plan 3c.

**Umgebung (Windows):** vor Go-/task-Befehlen `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`; Docker Desktop läuft (nicht selbst starten); Commits ohne Attributionszeilen; Branch `feature/unterlagen` (legt der Controller an).

---

## Dateien

| Datei | Aktion | Verantwortung |
|---|---|---|
| `backend/migrations/00006_documents.sql` | neu | Spalten + Tabelle |
| `backend/internal/store/queries/documents.sql` | neu | Queries Unterlagen |
| `backend/internal/store/queries/applications.sql` | ändern | neue Spalten in `GetApplication`, Agent-Liste |
| `backend/internal/testdb/testdb.go` | ändern | `Reset` leert `application_documents` |
| `backend/internal/documents/application.go`, `templates/application.html.tmpl` (+ Tests) | neu | Anschreiben + CV als ein HTML |
| `backend/internal/documents/templates/cv.html.tmpl`, `render.go` | ändern | Blöcke wiederverwendbar, Sprache |
| `backend/internal/mail/draft.go`, `imap.go` (+ Tests) | neu | MIME bauen, Entwurf per IMAP ablegen |
| `backend/internal/service/documents.go` (+ Test) | neu | Zustände, Rendern, Entwurf |
| `backend/internal/service/service.go` | ändern | Option `WithDrafter` |
| `backend/internal/config/config.go` (+ Test) | ändern | `GMAIL_ADDRESS`, `GMAIL_APP_PASSWORD` |
| `backend/cmd/server/main.go` | ändern | Drafter verdrahten |
| `backend/internal/httpapi/documents.go`, `agent_documents.go` (+ Tests), `convert.go` | neu/ändern | Handler |
| `api/openapi.yaml` | ändern | Schemas, Pfade |
| `docker-compose.yml`, `.env.example`, `README.md` | ändern | Gmail-Konfiguration |
| `frontend/src/app/features/applications/application-documents.*` | neu | Bereich „Unterlagen“ |
| `frontend/src/app/features/applications/application-detail.*`, `core/api.ts` | ändern | Einbindung, Fassade |

---

### Task 1: Datenmodell und Zustände

**Files:** Create `backend/migrations/00006_documents.sql`, `backend/internal/store/queries/documents.sql`; Modify `backend/internal/store/queries/applications.sql`, `backend/internal/testdb/testdb.go`; Create `backend/internal/service/documents.go`, `backend/internal/service/documents_test.go`; Modify `backend/internal/service/applications.go`.

- [ ] **Step 1: Migration**

```sql
-- +goose Up
ALTER TABLE applications
    ADD COLUMN documents_state text NOT NULL DEFAULT 'keine' CHECK (documents_state IN
        ('keine', 'angefordert', 'erstellt', 'entwurf_angelegt', 'portal', 'fehler')),
    ADD COLUMN documents_error text,
    ADD COLUMN gmail_draft_at  timestamptz;

CREATE INDEX applications_documents_state ON applications (documents_state) WHERE documents_state = 'angefordert';

-- Je Bewerbung genau eine aktuelle Fassung der Unterlagen.
CREATE TABLE application_documents (
    application_id uuid PRIMARY KEY REFERENCES applications (id) ON DELETE CASCADE,
    version        int  NOT NULL CHECK (version >= 1),
    language       text NOT NULL CHECK (language IN ('de', 'en')),
    cover_letter   text NOT NULL,
    profile_line   text,
    highlights     jsonb NOT NULL DEFAULT '[]',
    mail_subject   text NOT NULL,
    mail_body      text NOT NULL,
    pdf            bytea,
    file_name      text,
    rendered_at    timestamptz,
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE application_documents;
DROP INDEX applications_documents_state;
ALTER TABLE applications DROP COLUMN gmail_draft_at, DROP COLUMN documents_error, DROP COLUMN documents_state;
```

- [ ] **Step 2: Queries**

`backend/internal/store/queries/documents.sql`:

```sql
-- name: SetDocumentsState :exec
UPDATE applications SET documents_state = $2, documents_error = $3, updated_at = now() WHERE id = $1;

-- name: SetDraftCreated :exec
UPDATE applications SET documents_state = 'entwurf_angelegt', documents_error = NULL, gmail_draft_at = now(), updated_at = now()
WHERE id = $1;

-- name: GetDocuments :one
SELECT * FROM application_documents WHERE application_id = $1;

-- name: UpsertDocuments :one
INSERT INTO application_documents (application_id, version, language, cover_letter, profile_line, highlights,
                                   mail_subject, mail_body, pdf, file_name, rendered_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
ON CONFLICT (application_id) DO UPDATE
SET version = EXCLUDED.version, language = EXCLUDED.language, cover_letter = EXCLUDED.cover_letter,
    profile_line = EXCLUDED.profile_line, highlights = EXCLUDED.highlights, mail_subject = EXCLUDED.mail_subject,
    mail_body = EXCLUDED.mail_body, pdf = EXCLUDED.pdf, file_name = EXCLUDED.file_name,
    rendered_at = EXCLUDED.rendered_at, updated_at = now()
RETURNING *;

-- name: ListAgentApplicationsByDocumentsState :many
SELECT a.id, c.name AS company_name, c.website AS company_website, a.position_title, a.job_url, a.location,
       a.contact_email, a.posting_text, a.fit_reason, a.documents_state,
       COALESCE(d.version, 0)::int AS documents_version
FROM applications a
JOIN companies c ON c.id = a.company_id
LEFT JOIN application_documents d ON d.application_id = a.id
WHERE a.documents_state = $1
ORDER BY a.updated_at, a.id;
```

`GetApplication` liefert zusätzlich `a.documents_state, a.documents_error, a.gmail_draft_at`. `testdb.Reset` leert zusätzlich `application_documents` (vor `applications` bzw. in derselben `TRUNCATE`-Liste). Dann `task generate:sqlc`.

- [ ] **Step 3: Failing Tests (Service-Zustände)**

`backend/internal/service/documents_test.go` – nutze vorhandene Helfer (`newService`, `ctx`, `ptr`) und für eine Bewerbung `svc.CreateAgentJob(ctx, sampleJob())` aus `agent_jobs_test.go`:

```go
func TestRequestDocumentsSetsRequested(t *testing.T) {
	svc := newService(t)
	a, _ := svc.CreateAgentJob(ctx, sampleJob())
	if a.DocumentsState != service.DocsNone {
		t.Fatalf("Start: %s", a.DocumentsState)
	}
	got, err := svc.RequestDocuments(ctx, a.ID)
	if err != nil || got.DocumentsState != service.DocsRequested {
		t.Fatalf("angefordert: %v %s", err, got.DocumentsState)
	}
	if _, err := svc.RequestDocuments(ctx, a.ID); err != nil { // idempotent
		t.Fatal(err)
	}
	list, err := svc.ListAgentApplications(ctx, service.DocsRequested)
	if err != nil || len(list) != 1 || list[0].ID != a.ID || list[0].PostingText == nil {
		t.Fatalf("Agent-Liste: %v %+v", err, list)
	}
}

func TestRequestDocumentsUnknownApplication(t *testing.T) {
	svc := newService(t)
	var nf *service.NotFoundError
	if _, err := svc.RequestDocuments(ctx, uuid.New()); !errors.As(err, &nf) {
		t.Fatalf("erwartet NotFound, bekommen %v", err)
	}
}
```

- [ ] **Step 4: Implementierung**

`backend/internal/service/documents.go` (Teil 1; Teil 2 folgt in Task 4):

```go
package service

// Zustände der Bewerbungsunterlagen (Spalte applications.documents_state).
const (
	DocsNone      = "keine"
	DocsRequested = "angefordert"
	DocsCreated   = "erstellt"
	DocsDrafted   = "entwurf_angelegt"
	DocsPortal    = "portal"
	DocsFailed    = "fehler"
)

// AgentApplication ist eine Stelle, wie sie der Agent für die Unterlagen braucht.
type AgentApplication struct {
	ID             uuid.UUID
	CompanyName    string
	CompanyWebsite *string
	PositionTitle  string
	JobURL         *string
	Location       *string
	ContactEmail   *string
	PostingText    *string
	FitReason      *string
	DocumentsState string
	DocumentsVersion int // 0 = noch keine Unterlagen; der Agent liefert Version DocumentsVersion+1
}

// RequestDocuments gibt eine Stelle für die Unterlagen frei (auch erneut, z. B. nach einem Fehler).
func (s *Service) RequestDocuments(ctx context.Context, id uuid.UUID) (Application, error) { … }

// ListAgentApplications liefert Stellen in einem Unterlagen-Zustand, älteste Änderung zuerst.
func (s *Service) ListAgentApplications(ctx context.Context, state string) ([]AgentApplication, error) { … }
```

`RequestDocuments`: in einer Transaktion `LockApplication` (NotFound „Bewerbung“), dann `SetDocumentsState(id, DocsRequested, nil)`; danach `GetApplication`. Unbekannter `state` in `ListAgentApplications` → `ValidationError{Field: "documents_state"}`. `Application` bekommt `DocumentsState string`, `DocumentsError *string`, `GmailDraftAt *time.Time` (gefüllt in `GetApplication`).

- [ ] **Step 5: Tests + Lint, Commit** – `go test ./...`, `golangci-lint run ./...`; Commit `feat(docs): Datenmodell und Freigabe der Unterlagen`.

---

### Task 2: PDF aus Anschreiben und Lebenslauf

**Files:** Create `backend/internal/documents/application.go`, `backend/internal/documents/templates/application.html.tmpl`, `backend/internal/documents/application_test.go`; Modify `backend/internal/documents/templates/cv.html.tmpl`, `backend/internal/documents/render.go`, `backend/internal/documents/gotenberg_test.go`.

- [ ] **Step 1: Vorlage zerlegen, ohne das CV-PDF zu verändern**

`cv.html.tmpl` in benannte Blöcke aufteilen: `{{define "styles"}}` (der komplette `<style>`-Inhalt), `{{define "aside"}}` (Seitenleiste: Foto, Kontakt, Kenntnisse, Sprachen), `{{define "cv-main"}}` (Hauptspalte). Die Datei selbst rendert weiterhin das vollständige Dokument aus diesen Blöcken. Überschriften und „heute“ kommen aus einem Feld `.L` (Labels, siehe unten) statt fest im Text.

Test zuerst: `TestRenderCVUnchanged` rendert `sample(t)` mit der alten Vorlage (vor dem Umbau einmal als `testdata/cv_golden.html` speichern) und vergleicht byte-genau mit dem neuen Ergebnis. Nach dem Umbau muss er grün sein; danach darf die Golden-Datei bleiben (schützt vor ungewollten Layout-Änderungen).

Labels in `render.go`:

```go
// Labels sind die festen Texte der Vorlagen je Sprache.
type Labels struct {
	Contact, Skills, Languages, Profile, Projects, Experience, Education, Present string
	Closing, SubjectPrefix string
	Months [12]string
}

var labels = map[string]Labels{
	"de": {Contact: "Kontakt", Skills: "Kenntnisse", Languages: "Sprachen", Profile: "Profil", Projects: "Projekte",
		Experience: "Berufserfahrung", Education: "Ausbildung", Present: "heute",
		Closing: "Mit freundlichen Grüßen", SubjectPrefix: "Bewerbung als",
		Months: [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}},
	"en": {Contact: "Contact", Skills: "Skills", Languages: "Languages", Profile: "Profile", Projects: "Projects",
		Experience: "Experience", Education: "Education", Present: "present",
		Closing: "Kind regards", SubjectPrefix: "Application for",
		Months: [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}},
}
```

Die bisherigen deutschen Überschriften in `cv.html.tmpl` exakt so übernehmen, wie sie heute im Template stehen (falls sie von obiger Liste abweichen, die Template-Texte als `de`-Werte nehmen, damit der Golden-Test grün bleibt). `period` bekommt das Wort für „heute“ als Parameter (bzw. eine Methode am View), `RenderCV` nutzt `labels["de"]`.

- [ ] **Step 2: Failing Tests für `RenderApplication`**

`backend/internal/documents/application_test.go`:

```go
func sampleLetter() documents.Letter {
	return documents.Letter{
		Language: "de", CompanyName: "Acme GmbH", PositionTitle: "Junior Frontend-Entwickler",
		CoverLetter: "Sehr geehrte Damen und Herren,\n\nerster Absatz.\n\nzweiter Absatz.",
		ProfileLine: "Junior-Frontend-Entwickler mit Angular-Projekten.",
		Highlights:  []string{"TypeScript", "Erfunden"},
		Date:        time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
	}
}

func TestRenderApplicationContainsLetterAndCV(t *testing.T) {
	html := string(must(documents.RenderApplication(sample(t), nil, sampleLetter())))
	for _, want := range []string{
		"Bewerbung als Junior Frontend-Entwickler", "Acme GmbH", "5. Oktober 2026",
		"<p>Sehr geehrte Damen und Herren,</p>", "<p>erster Absatz.</p>", "Mit freundlichen Grüßen",
		"Junior-Frontend-Entwickler mit Angular-Projekten.", // ersetzt das Profil
		"class=\"letter\"", "class=\"page cv\"",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("fehlt: %q", want)
		}
	}
	if strings.Contains(html, "Erfunden") {
		t.Error("Schwerpunkt, der nicht im Lebenslauf steht, darf nicht erscheinen")
	}
}

func TestRenderApplicationOrdersSkillsByHighlights(t *testing.T) {
	cv := sample(t) // Kenntnisse z. B. Frontend: Angular, TypeScript …
	l := sampleLetter()
	l.Highlights = []string{"typescript"}
	html := string(must(documents.RenderApplication(cv, nil, l)))
	if strings.Index(html, ">TypeScript<") > strings.Index(html, ">Angular<") {
		t.Error("TypeScript muss vor Angular stehen")
	}
}

func TestRenderApplicationEnglish(t *testing.T) {
	l := sampleLetter()
	l.Language = "en"
	html := string(must(documents.RenderApplication(sample(t), nil, l)))
	for _, want := range []string{"Application for", "5 October 2026", "Kind regards", "Experience"} {
		if !strings.Contains(html, want) {
			t.Errorf("fehlt: %q", want)
		}
	}
}

func TestRenderApplicationEscapes(t *testing.T) {
	l := sampleLetter()
	l.CoverLetter = "<script>alert(1)</script>"
	if strings.Contains(string(must(documents.RenderApplication(sample(t), nil, l))), "<script>alert") {
		t.Error("Anschreiben muss escaped werden")
	}
}

func TestApplicationFileName(t *testing.T) {
	if got := documents.ApplicationFileName(sample(t), "Müller & Söhne GmbH"); got != "Bewerbung_Mustermann_Mueller_Soehne_GmbH.pdf" {
		t.Fatalf("got %q", got)
	}
}
```

(Erwartete Namen an `sample(t)` anpassen – der Nachname ist das letzte Wort von `person.name`. Gibt es in `sample` keine Kenntnis „TypeScript“/„Angular“, die Testdaten dort ergänzen.)

In `gotenberg_test.go` ergänzen: `TestGotenbergRendersApplication` rendert `RenderApplication` gegen den Gotenberg-Testcontainer und prüft, dass das PDF mindestens 2 Seiten hat (Zählung `/Type /Page` ohne `/Pages`, wie im bestehenden CV-Test, falls vorhanden – sonst so).

- [ ] **Step 3: Implementierung**

`backend/internal/documents/application.go`:

```go
// Letter ist der stellenbezogene Teil der Bewerbung.
type Letter struct {
	Language      string // "de" oder "en"
	CompanyName   string
	PositionTitle string
	CoverLetter   string   // Anrede und Absätze, getrennt durch Leerzeilen; Gruß und Name ergänzt die Vorlage
	ProfileLine   string   // ersetzt das Profil im Lebenslauf, wenn gesetzt
	Highlights    []string // ordnen die Kenntnisse; nur vorhandene Einträge zählen
	Date          time.Time
}

// RenderApplication erzeugt ein HTML-Dokument: Seite 1 Anschreiben, danach der Lebenslauf.
func RenderApplication(cv CV, photo []byte, l Letter) ([]byte, error)

// ApplicationFileName liefert z. B. "Bewerbung_Marscheider_Acme_GmbH.pdf" (nur ASCII).
func ApplicationFileName(cv CV, company string) string
```

- Absätze: `CoverLetter` an `\n\s*\n` trennen, jeden Absatz trimmen, leere weglassen, einzelne Zeilenumbrüche innerhalb eines Absatzes als `<br>` (Template: Zeilen einzeln ausgeben).
- Datum: `de` → `5. Oktober 2026`, `en` → `5 October 2026`; davor der Ort aus `cv.Person.Location`, falls gesetzt (`Espelkamp, 5. Oktober 2026`).
- Schwerpunkte: Kopie von `cv.Skills`; innerhalb jeder Gruppe Einträge, die (ohne Groß-/Kleinschreibung, getrimmt) in `Highlights` stehen, in der Reihenfolge der Highlights nach vorn; Gruppen mit mindestens einem Treffer nach vorn (stabile Reihenfolge sonst). Unbekannte Highlights ignorieren.
- `ProfileLine` ≠ "" ersetzt `Summary`.
- Vorlage `application.html.tmpl`: nutzt `{{template "styles"}}`, dann eine Anschreiben-Seite `<div class="page letter">` mit `{{template "aside" .}}` und einer Hauptspalte (Name + Überschrift wie im CV-Kopf, Empfänger `CompanyName`, Ort/Datum rechtsbündig, Betreff fett `SubjectPrefix + " " + PositionTitle`, Absätze, Gruß `Closing`, Name), danach `<div class="page cv" style="break-before: page">` mit Seitenleiste und `{{template "cv-main" .}}`. Die Anschreiben-Seite darf nicht umbrechen: `.letter { height: 297mm; overflow: hidden; }` – zu lange Texte werden abgeschnitten, deshalb begrenzt die API `cover_letter` auf 3000 Zeichen.
- Beide Vorlagen über dieselbe `template.Template`-Menge parsen (`ParseFS(templateFS, "templates/*.tmpl")`), `RenderCV` führt `cv.html.tmpl` aus, `RenderApplication` `application.html.tmpl`.

- [ ] **Step 4: Tests + Lint** (`go test ./internal/documents/...` inkl. Gotenberg-Container, dann `./...`), **Commit** `feat(docs): Anschreiben und Lebenslauf als ein PDF`.

---

### Task 3: Gmail-Entwurf per IMAP

**Files:** Create `backend/internal/mail/draft.go`, `backend/internal/mail/draft_test.go`, `backend/internal/mail/imap.go`, `backend/internal/mail/imap_test.go`; Modify `backend/go.mod`.

- [ ] **Step 1: Failing Tests**

`draft_test.go`: `BuildDraft(Draft{From: "Christian Marscheider <c@example.com>", To: "jobs@acme.example", Subject: "Bewerbung als Junior Frontend-Entwickler (m/w/d) – Christian Marscheider", Body: "Sehr geehrte …\n\nViele Grüße", FileName: "Bewerbung_Marscheider_Acme.pdf", PDF: []byte("%PDF-1.7 test"), Date: fixed})` ergibt eine Nachricht, die `net/mail.ReadMessage` lesen kann; Prüfungen:
- `Subject` ist RFC-2047-kodiert und dekodiert (`mime.WordDecoder`) wieder zum Original (Umlaute, Gedankenstrich).
- `To`, `From`, `Date`, `MIME-Version: 1.0`, `Message-Id` (Format `<…@…>`) vorhanden.
- `multipart/mixed` mit genau zwei Teilen: `text/plain; charset=utf-8` (Inhalt nach Dekodierung = Body) und `application/pdf` mit `Content-Disposition: attachment; filename="Bewerbung_Marscheider_Acme.pdf"`, `Content-Transfer-Encoding: base64`, dekodiert = PDF-Bytes.
- `To` mit Zeilenumbruch (`"a@b.de\r\nBcc: x@y.de"`) → Fehler (Header-Injection); dasselbe für `Subject` mit `\r` oder `\n`.

`imap_test.go` startet einen In-Memory-IMAP-Server aus `github.com/emersion/go-imap/v2/imapserver/imapmemserver` (Benutzer `user`/`pass`, Postfächer `INBOX` und `Entwürfe` mit Special-Use `\Drafts`), auf `127.0.0.1:0` ohne TLS. `NewIMAPDrafter(IMAPConfig{Addr: addr, Username: "user", Password: "pass", Insecure: true})` → `Save(ctx, msg)` legt genau eine Nachricht in `Entwürfe` ab, mit Flag `\Draft`. Falsches Passwort → Fehler, der `ErrAuth` erfüllt (`errors.Is`). Server nicht erreichbar → Fehler, der `ErrUnavailable` erfüllt.

(Unterstützt `imapmemserver` keine Special-Use-Attribute, stattdessen den Fallback testen: Postfach namens `[Gmail]/Drafts` wird gefunden.)

- [ ] **Step 2: Implementierung**

```go
// Package mail baut Bewerbungsentwürfe und legt sie per IMAP im Entwurfsordner ab. Es sendet nie.
package mail

type Draft struct {
	From, To, Subject, Body string
	FileName                string
	PDF                     []byte
	Date                    time.Time
}

// BuildDraft erzeugt eine RFC-5322-Nachricht (multipart/mixed: Text + PDF).
func BuildDraft(d Draft) ([]byte, error)

type IMAPConfig struct {
	Addr               string // z. B. "imap.gmail.com:993"
	Username, Password string
	Insecure           bool // nur für Tests: ohne TLS
}

var (
	ErrAuth        = errors.New("imap: anmeldung fehlgeschlagen")
	ErrUnavailable = errors.New("imap: server nicht erreichbar")
	ErrNoDrafts    = errors.New("imap: kein entwurfsordner gefunden")
)

type IMAPDrafter struct{ cfg IMAPConfig }

func NewIMAPDrafter(cfg IMAPConfig) *IMAPDrafter

// Save legt msg mit Flag \Draft im Ordner mit Special-Use \Drafts ab
// (Fallback: "[Gmail]/Drafts", "[Gmail]/Entwürfe", "Drafts", "Entwürfe").
func (d *IMAPDrafter) Save(ctx context.Context, msg []byte) error
```

- Header-Werte: `mime.QEncoding.Encode("utf-8", …)` für Subject und Anzeigename; Adressen mit `net/mail.ParseAddress` prüfen; jedes `\r`/`\n` in Header-Werten → Fehler.
- Text-Teil: `quoted-printable` (`mime/quotedprintable`), PDF-Teil: base64 mit Zeilen à 76 Zeichen.
- Message-ID: `<` + zufällige 16 Byte hex + `@` + Domain der From-Adresse + `>`.
- IMAP: `imapclient.DialTLS` (bzw. `DialInsecure` bei `Insecure`), Verbindungs-Timeout 15 s, `Login`, `List("", "*", &imap.ListOptions{ReturnSpecialUse: true})`, Ordner mit `imap.MailboxAttrDrafts` wählen, `Append(mailbox, size, &imap.AppendOptions{Flags: []imap.Flag{imap.FlagDraft}, Time: d.Date})`, `Logout`. Kontext-Abbruch beachten.

`go get github.com/emersion/go-imap/v2@latest` (Version im Commit festhalten).

- [ ] **Step 3: Tests + Lint, Commit** `feat(mail): Bewerbungsentwurf per IMAP ablegen`.

---

### Task 4: Service – Unterlagen speichern, rendern, Entwurf anlegen

**Files:** Modify `backend/internal/service/documents.go`, `backend/internal/service/documents_test.go`, `backend/internal/service/service.go`, `backend/internal/config/config.go` (+ Test), `backend/cmd/server/main.go`, `docker-compose.yml`, `.env.example`.

- [ ] **Step 1: Failing Tests**

In `documents_test.go` mit Fakes (kein echtes Gotenberg/IMAP): `fakePDF` (liefert `%PDF` oder Fehler `documents.ErrUnavailable`) über die vorhandene Option `WithPDFConverter`, `fakeDrafter` (zählt Aufrufe, speichert letzte Nachricht, kann Fehler liefern) über neue Option `WithDrafter(d Drafter, from string)`. Lebenslauf vorher mit dem vorhandenen Helfer `saveSampleCV` speichern.

Fälle:
1. **Agent liefert, mit Adresse:** `RequestDocuments` → `SaveAgentDocuments(ctx, id, DocumentsInput{Version: 1, Language: "de", CoverLetter: "…", MailSubject: "…", MailBody: "…", Highlights: []string{"TypeScript"}})` → Zustand `entwurf_angelegt`, `GmailDraftAt` gesetzt, Drafter 1× aufgerufen, Nachricht enthält `To: jobs@acme.example` und Dateiname `Bewerbung_…_Acme_GmbH.pdf`; `GetDocuments` liefert Version 1, `DocumentsPDF` liefert die PDF-Bytes und den Dateinamen.
2. **Idempotent:** derselbe Aufruf erneut (Version 1, gleicher Inhalt) → kein Fehler, Drafter bleibt bei 1 Aufruf, Zustand unverändert.
3. **Konflikte:** `SaveAgentDocuments` ohne vorherige Anforderung → `ConflictError`; Version 1 mit anderem Inhalt, nachdem Version 1 gespeichert ist → `ConflictError`.
4. **Ohne Adresse:** Stelle ohne `ContactEmail` → Zustand `portal`, Drafter nicht aufgerufen.
5. **Ohne Drafter (IMAP nicht eingerichtet), mit Adresse:** Zustand `erstellt`.
6. **Gotenberg nicht erreichbar:** `UnavailableError`, Zustand bleibt `angefordert`, `GetDocuments` → NotFound (nichts gespeichert).
7. **Anderer Renderfehler:** Zustand `fehler`, `DocumentsError` gesetzt, Fehler wird zurückgegeben.
8. **Entwurf scheitert:** Drafter liefert Fehler → Unterlagen bleiben gespeichert, Zustand `erstellt`, `DocumentsError` = „Gmail-Entwurf konnte nicht angelegt werden: …“; kein Fehler an den Aufrufer (PDF ist ja da).
9. **Oberfläche bearbeitet:** `UpdateDocuments(ctx, id, DocumentsInput{…ohne Version…})` nach Fall 1 → Version 2, neu gerendert, Zustand bleibt `entwurf_angelegt` (der alte Entwurf bleibt in Gmail; die Oberfläche weist darauf hin); ohne vorhandene Unterlagen → NotFound.
10. **Entwurf neu anlegen:** `CreateDraft(ctx, id)` → Drafter erneut aufgerufen, Zustand `entwurf_angelegt`; ohne Adresse → `ValidationError{Field: "contact_email"}`; ohne Drafter → `UnavailableError`.
11. **Validierung:** `Language` nicht `de`/`en`, leeres `CoverLetter`/`MailSubject`/`MailBody` → `ValidationError` mit passendem `Field`.
12. **Kein Lebenslauf gespeichert:** `ConflictError` („Erst den Lebenslauf speichern“).

- [ ] **Step 2: Implementierung**

```go
// Drafter legt eine fertige Nachricht als Entwurf ab (Produktion: mail.IMAPDrafter).
type Drafter interface {
	Save(ctx context.Context, msg []byte) error
}

func WithDrafter(d Drafter, from string) Option // from = "Name <adresse>" oder nur Adresse

type DocumentsInput struct {
	Version     int // nur Agent; 0 bei der Oberfläche (= aktuelle Version + 1)
	Language    string
	CoverLetter string
	ProfileLine *string
	Highlights  []string
	MailSubject string
	MailBody    string
}

type Documents struct {
	Version     int
	Language    string
	CoverLetter string
	ProfileLine *string
	Highlights  []string
	MailSubject string
	MailBody    string
	FileName    *string
	RenderedAt  *time.Time
	UpdatedAt   time.Time
}

func (s *Service) SaveAgentDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Documents, error)
func (s *Service) UpdateDocuments(ctx context.Context, id uuid.UUID, in DocumentsInput) (Documents, error)
func (s *Service) GetDocuments(ctx context.Context, id uuid.UUID) (Documents, error)
func (s *Service) DocumentsPDF(ctx context.Context, id uuid.UUID) ([]byte, string, error)
func (s *Service) CreateDraft(ctx context.Context, id uuid.UUID) (Application, error)
```

Ablauf `SaveAgentDocuments`:
1. Validieren (siehe Fall 11); `Highlights` höchstens 8, jeweils getrimmt, leere verwerfen.
2. Lesen (ohne Sperre): Bewerbung (+ Firma), vorhandene Unterlagen. Ist `Version` = gespeicherte Version und der Inhalt gleich → gespeicherte Unterlagen zurückgeben (idempotent). Ist der Zustand nicht `angefordert` → `ConflictError("Für diese Stelle sind keine Unterlagen angefordert")`; ist `Version` ≤ gespeicherte Version → `ConflictError("Version ist veraltet")`.
3. Rendern **außerhalb** der Transaktion (Lebenslauf + Foto laden wie `CVPDF`, `documents.RenderApplication`, `s.pdf.Convert` mit `pdfTimeout`). `ErrUnavailable` → `UnavailableError`, nichts speichern. Anderer Fehler → `SetDocumentsState(id, DocsFailed, &msg)` und Fehler zurück.
4. In einer Transaktion: `LockApplication`, Zustand erneut prüfen (noch `angefordert`), `UpsertDocuments`, Zustand `erstellt` (bzw. `portal` ohne Adresse).
5. Mit Adresse und Drafter: `mail.BuildDraft` (From aus `WithDrafter`, To = `contact_email`, Subject/Body aus Input, Dateiname, PDF) → `Save` mit eigenem 30-s-Timeout → `SetDraftCreated`; Fehler → `SetDocumentsState(id, DocsCreated, "Gmail-Entwurf konnte nicht angelegt werden: …")`, kein Fehler an den Aufrufer.

`UpdateDocuments`: wie oben, aber Version = gespeichert + 1, kein Zustandswechsel außer `fehler` → `erstellt`, kein automatischer Entwurf. `CreateDraft`: lädt Unterlagen + PDF und führt Schritt 5 aus (Fehler hier werden zurückgegeben: `UnavailableError` bei IMAP nicht erreichbar, `ConflictError` bei Anmeldefehler mit Hinweis „App-Passwort prüfen“).

Config: `GmailAddress`, `GmailAppPassword` aus `GMAIL_ADDRESS`, `GMAIL_APP_PASSWORD`; beide oder keine (sonst Fehler „GMAIL_ADDRESS und GMAIL_APP_PASSWORD gehören zusammen“); Leerzeichen im App-Passwort entfernen (Google zeigt es in 4er-Gruppen). `main.go`: wenn gesetzt, `service.WithDrafter(mail.NewIMAPDrafter(mail.IMAPConfig{Addr: "imap.gmail.com:993", …}), cvName+" <"+addr+">")` – den Anzeigenamen beim Bauen der Nachricht aus dem gespeicherten Lebenslauf nehmen (Service kennt ihn), daher `WithDrafter(d, address)` und der Service setzt `"Name <address>"`. Info-Log, wenn Gmail nicht eingerichtet ist. `docker-compose.yml`: `GMAIL_ADDRESS: ${GMAIL_ADDRESS:-}`, `GMAIL_APP_PASSWORD: ${GMAIL_APP_PASSWORD:-}`; `.env.example` dokumentiert beide (App-Passwort unter myaccount.google.com/apppasswords, erfordert Bestätigung in zwei Schritten; der Pi legt nur Entwürfe an, sendet nie).

- [ ] **Step 3: Tests + Lint, Commit** `feat(docs): Unterlagen rendern und Gmail-Entwurf anlegen`.

---

### Task 5: API

**Files:** Modify `api/openapi.yaml`, `backend/internal/httpapi/convert.go`, `README.md`; Create `backend/internal/httpapi/documents.go`, `backend/internal/httpapi/agent_documents.go`, `backend/internal/httpapi/documents_test.go`; Generate server + client.

- [ ] **Step 1: Spec**

Schemas:

```yaml
    DocumentsState:
      type: string
      enum: [keine, angefordert, erstellt, entwurf_angelegt, portal, fehler]
    DocumentsInput:
      type: object
      required: [language, cover_letter, mail_subject, mail_body]
      properties:
        language: { type: string, enum: [de, en] }
        cover_letter: { type: string, minLength: 1, maxLength: 3000, pattern: '\S' }
        profile_line: { type: string, maxLength: 400 }
        highlights:
          type: array
          maxItems: 8
          items: { type: string, minLength: 1, maxLength: 60, pattern: '\S' }
        mail_subject: { type: string, minLength: 1, maxLength: 200, pattern: '^[^\r\n]*\S[^\r\n]*$' }
        mail_body: { type: string, minLength: 1, maxLength: 3000, pattern: '\S' }
    AgentDocumentsInput:
      allOf:
        - $ref: "#/components/schemas/DocumentsInput"
        - type: object
          required: [version]
          properties:
            version: { type: integer, minimum: 1 }
    Documents:
      type: object
      required: [version, language, cover_letter, highlights, mail_subject, mail_body, updated_at]
      properties:
        version: { type: integer }
        language: { type: string, enum: [de, en] }
        cover_letter: { type: string }
        profile_line: { type: string }
        highlights: { type: array, items: { type: string } }
        mail_subject: { type: string }
        mail_body: { type: string }
        file_name: { type: string }
        rendered_at: { type: string, format: date-time }
        updated_at: { type: string, format: date-time }
    AgentApplication:
      type: object
      required: [id, company_name, position_title, documents_state]
      properties:
        id: { type: string, format: uuid }
        company_name: { type: string }
        company_website: { type: string }
        position_title: { type: string }
        job_url: { type: string }
        location: { type: string }
        contact_email: { type: string }
        posting_text: { type: string }
        fit_reason: { type: string }
        documents_state: { $ref: "#/components/schemas/DocumentsState" }
        documents_version: { type: integer, description: "0 = noch keine Unterlagen; zu liefern ist documents_version + 1" }
```

`AgentApplication.required` enthält zusätzlich `documents_version`.

(Prüfen, ob kin-openapi `allOf` mit `required` sauber validiert; sonst `AgentDocumentsInput` als eigenständiges Objekt mit allen Feldern + `version` ausschreiben.)

`Application` bekommt `documents_state` (required), `documents_error`, `gmail_draft_at` (date-time).

Pfade (Oberfläche, ohne Token):
- `POST /api/v1/applications/{id}/documents/request` (`requestDocuments`) → 200 `Application`
- `GET /api/v1/applications/{id}/documents` (`getDocuments`) → 200 `Documents` / 404
- `PUT /api/v1/applications/{id}/documents` (`updateDocuments`, Body `DocumentsInput`) → 200 `Documents`
- `GET /api/v1/applications/{id}/documents/pdf` (`getDocumentsPdf`) → `application/pdf` mit `Content-Disposition: inline; filename="…"` und `Cache-Control: no-store` (wie `/cv/pdf`)
- `POST /api/v1/applications/{id}/documents/draft` (`createDraft`) → 200 `Application`

Pfade (Agent, `security: [{ agentToken: [] }]`, Tag `Agent`):
- `GET /api/agent/applications?documents_state=` (`agentListApplications`, Parameter Pflicht, Schema `DocumentsState`) → 200 `[AgentApplication]`
- `PUT /api/agent/applications/{id}/documents` (`agentPutDocuments`, Body `AgentDocumentsInput`) → 200 `Documents`

`task generate:api`, `task generate:client`.

- [ ] **Step 2: Failing HTTP-Tests** (`documents_test.go`): Der Test-Server bekommt Fake-PDF und Fake-Drafter (Router-/Service-Optionen wie in vorhandenen PDF-Tests nachsehen). Ablauf: Stelle per Agent anlegen → `POST …/documents/request` → `GET /api/agent/applications?documents_state=angefordert` enthält sie (mit `posting_text`) → `PUT /api/agent/applications/{id}/documents` (Version 1) → 200, danach `GET /api/v1/applications/{id}` zeigt `entwurf_angelegt`; `GET …/documents/pdf` → 200 `application/pdf`, `Content-Disposition` mit `Bewerbung_`; zweites identisches PUT → 200; `PUT` mit `mail_subject` mit `\n` → 400; `cover_letter` 3001 Zeichen → 400; ohne Token → 401; `GET …/documents` einer Stelle ohne Unterlagen → 404.

- [ ] **Step 3: Handler** – dünn, nach Muster `cv_review.go`/`cv_pdf.go`; `documentsDTO`, `agentApplicationDTO` in `convert.go`; `applicationDTO` um die neuen Felder ergänzen.

- [ ] **Step 4: README** – Agent-API-Liste ergänzen (`GET /api/agent/applications?documents_state=angefordert`, `PUT /api/agent/applications/{id}/documents` mit `version`, idempotent, 409/503-Bedeutung); neuer Absatz „Gmail-Entwürfe“: `GMAIL_ADDRESS`/`GMAIL_APP_PASSWORD` in `.env`, der Pi legt Entwürfe per IMAP an und sendet nie.

- [ ] **Step 5: Tests + Lint + `npx ng build`, Commit** `feat(docs): API für Unterlagen und Entwürfe`.

---

### Task 6: Oberfläche – Bereich „Unterlagen“

**Files:** Create `frontend/src/app/features/applications/application-documents.{ts,html,scss,spec.ts}`; Modify `frontend/src/app/core/api.ts`, `application-detail.{ts,html,spec.ts}`.

- [ ] **Step 1: Failing Tests** (`application-documents.spec.ts`, Api gemockt; Eingaben `application` (mit `documents_state`, `contact_email`, `job_url`) und Ausgabe `changed` (neu geladene Bewerbung)):
- `keine`: Knopf „Unterlagen erstellen“ ruft `requestDocuments(id)` und meldet `changed`.
- `angefordert`: Text „Claude erstellt die Unterlagen beim nächsten Lauf.“ und Knopf „Status aktualisieren“ (lädt die Bewerbung neu → `changed`).
- `erstellt`/`entwurf_angelegt`/`portal`: lädt `getDocuments(id)`; zeigt Link „PDF ansehen“ (`/api/v1/applications/{id}/documents/pdf`, `target=_blank`), Formular mit `cover_letter` (Textarea), `profile_line`, `mail_subject`, `mail_body` (Textarea), Sprache; „Speichern und neu rendern“ ruft `updateDocuments(id, …)` (Highlights unverändert mitsenden).
- `entwurf_angelegt`: Hinweis „Entwurf liegt in Gmail“ mit Link `https://mail.google.com/mail/u/0/#drafts` (`target=_blank`, `rel=noopener`) und Datum `gmail_draft_at`; nach dem Speichern zusätzlich „Der Gmail-Entwurf enthält noch die alte Fassung“ mit Knopf „Gmail-Entwurf neu anlegen“ → `createDraft(id)`.
- `erstellt` mit `contact_email`: Knopf „Gmail-Entwurf anlegen“ → `createDraft(id)`; `portal`: Hinweis „Keine Bewerbungsadresse – über das Portal bewerben“ mit `job_url`-Link.
- `fehler`: `documents_error` in einer Fehlermeldung (`role="alert"`) und Knopf „Erneut anfordern“ → `requestDocuments`.
- `documents_error` bei `erstellt` (Entwurf gescheitert) wird ebenfalls angezeigt.
- Erneut anfordern bei vorhandenen Unterlagen fragt nach (`window.confirm`, im Test gemockt): „Claude schreibt die Unterlagen neu. Fortfahren?“.

- [ ] **Step 2: Implementierung** – Komponente nach Muster `cv-review.ts` (Signals, `takeUntilDestroyed`, `busy`-Guard, Snackbar-Meldungen, `role="status"`-Bereich für Zustandstexte). Fassade in `core/api.ts`: `requestDocuments`, `getDocuments` (mit `SILENT_NOT_FOUND`), `updateDocuments`, `createDraft`. Im Detail unter dem Bereich „Passung“ einbinden; `changed` aktualisiert die angezeigte Bewerbung.

- [ ] **Step 3: Tests + Build, Commit** `feat(docs): Bereich Unterlagen in der Bewerbung`.

---

### Task 7: Abnahme (Controller, mit dem User)

- [ ] Merge nach Rückfrage, Deploy auf den Pi.
- [ ] User richtet Bestätigung in zwei Schritten und ein App-Passwort ein (myaccount.google.com/apppasswords) und gibt es **selbst** auf dem Pi in `~/bewerbungsmanager/.env` ein (`GMAIL_ADDRESS=…`, `GMAIL_APP_PASSWORD=…`) – das Passwort erscheint nie im Chat. Dann `docker compose up -d`.
- [ ] Probe: Test-Stelle mit `contact_email` = eigene Adresse anlegen, „Unterlagen erstellen“, Haupt-Agent liefert als R2 Version 1 → Entwurf erscheint in Gmail mit PDF; PDF und Mail mit dem User durchsehen; Test-Stelle und Entwurf löschen.
- [ ] Routine R2 erweitern (Prompt: Stellen mit `angefordert` bearbeiten, Regeln aus der Spec, KI nur bei KI-Anzeige, Anschreiben max. 3000 Zeichen, Mailtext 3–5 Sätze, `version` = `documents_version` + 1, bei Wiederholung nach Netzfehler dieselbe) und den Rhythmus mit dem User klären (Spec: stündlich 8–20 Uhr; derzeit 3× täglich).
