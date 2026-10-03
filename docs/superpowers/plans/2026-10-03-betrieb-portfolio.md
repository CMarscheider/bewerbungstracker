# Bewerbungs-Tracker Betrieb & Portfolio – Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Das fertige Projekt mit einem Befehl startbar und vorzeigbar machen. Dazu gehören:
- `docker compose up` startet den kompletten Stack.
- Realistische Demo-Daten lassen sich laden.
- Screenshots entstehen reproduzierbar.
- Eine GitHub-Actions-CI läuft.
- README und ADRs erklären das Projekt Recruitern und Entwicklern.

**Architecture:** Compose startet drei Dienste: `db` (Postgres 17), `backend` (Go-Image) und `frontend` (nginx-Image, leitet `/api` an `backend:8080` weiter, dynamische DNS-Auflösung). Nach außen ist nur das Frontend erreichbar. Die Demo-Daten erzeugt ein Go-Programm `cmd/seed`. Es spricht die echte REST-API an, damit alle Fachregeln greifen. Alle Daten sind relativ zu „heute“, Dashboard und Statistik bleiben also aktuell. Ein Unit-Test prüft jedes Szenario gegen den Domain-Zustandsautomaten. Die Screenshots erstellt ein Playwright-Skript. Die CI prüft Lint, Tests, Codegenerierung ohne Abweichung und beide Image-Builds.

**Tech Stack:** Docker Compose, Go (Seed), Playwright (Node 24), GitHub Actions, golangci-lint v2, Mermaid im README.

**Spec:** `docs/superpowers/specs/2026-10-02-bewerbungs-tracker-design.md` (Abschnitte „Betrieb und Werkzeuge“, „Portfolio“).

**Fakten aus Plan 1/2, auf die sich dieser Plan verlässt:**
- Backend-Image: `backend/Dockerfile`, lauscht auf 8080 und migriert beim Start. Die Konfiguration kommt aus `DATABASE_URL` und `PORT`.
- Frontend-Image: `frontend/Dockerfile`, nginx auf Port 80. `nginx.conf` leitet `/api/` an `http://backend:8080` weiter (Resolver `127.0.0.11`), der Compose-Dienst **muss** also `backend` heißen.
- API-Doku liegt unter `/api/docs`, das Spec-JSON unter `/api/openapi.json`. Fehler kommen als Problem-JSON.
- Domainregeln (`backend/internal/domain`):
  - Erstes Ereignis ist `Vorgemerkt` oder `Beworben`.
  - Nur Termine (`ScreeningGespraech`, `Interview`, `Kennenlerntag`) dürfen in der Zukunft liegen.
  - Fristen gibt es nur bei `Vorgemerkt`, `ChallengeErhalten` und `AngebotErhalten`.
  - Ein Datum darf nicht vor dem vorherigen Ereignis liegen.
- Auf dem Entwicklungsrechner ist **Port 8080 belegt**. Compose stellt nach außen nur das Frontend bereit, Standard ist **Port 4200**, überschreibbar mit `APP_PORT`.
- Der Rechner nutzt Windows mit Git Bash. Vor Befehlen jeweils `export PATH="/c/Program Files/nodejs:/c/Program Files/Go/bin:$HOME/go/bin:$PATH"` setzen.

---

## Dateistruktur

```
docker-compose.yml                 db + backend + frontend
.env.example                       POSTGRES_PASSWORD, APP_PORT
Taskfile.yml                       + up, down, logs, seed, screenshots, lint
backend/.golangci.yml              Linter-Konfiguration
backend/cmd/seed/
  main.go                          HTTP-Client, legt Demo-Daten über die API an
  scenarios.go                     ~25 Szenarien mit relativen Daten
  scenarios_test.go                prüft jedes Szenario gegen domain.CanApply + Abdeckung
tools/screenshots/
  package.json, shoot.mjs          Playwright-Screenshots aller Seiten
docs/screenshots/*.png             erzeugte Bilder (eingecheckt)
docs/decisions/0001-…0004-*.md     ADRs
.github/workflows/ci.yml           CI
README.md                          Portfolio-Startseite
```

---

### Task 1: Docker Compose

**Files:** Create: `docker-compose.yml`, `.env.example`. Modify: `Taskfile.yml`, `.gitignore`.

- [x] **Step 1: Compose-Datei**

`docker-compose.yml`:

```yaml
name: bewerbungsmanager

services:
  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: bewerbungen
      POSTGRES_USER: bewerbungen
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-bewerbungen}
      TZ: Europe/Berlin
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U bewerbungen -d bewerbungen"]
      interval: 5s
      timeout: 3s
      retries: 20
    restart: unless-stopped

  backend:
    build: ./backend
    environment:
      DATABASE_URL: postgres://bewerbungen:${POSTGRES_PASSWORD:-bewerbungen}@db:5432/bewerbungen?sslmode=disable
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped

  frontend:
    build: ./frontend
    ports:
      - "${APP_PORT:-4200}:80"
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  pgdata:
```

`.env.example`:

```
# Kopieren nach .env und bei Bedarf anpassen.
POSTGRES_PASSWORD=bewerbungen
APP_PORT=4200
```

`.gitignore` hat `.env` bereits. Prüfen und ggf. ergänzen.

- [x] **Step 2: Taskfile**

In `Taskfile.yml` ergänzen:

```yaml
  up:
    desc: Startet den kompletten Stack (http://localhost:4200)
    cmds:
      - docker compose up -d --build

  down:
    desc: Stoppt den Stack (Daten bleiben erhalten)
    cmds:
      - docker compose down

  logs:
    desc: Zeigt die Logs aller Dienste
    cmds:
      - docker compose logs -f
```

- [x] **Step 3: Smoke-Test in eigenem Projekt**

Läuft mit eigenem Projektnamen und Port 14200, damit kein späterer Echtbetrieb berührt wird.

```bash
APP_PORT=14200 docker compose -p bm-smoke up -d --build
for i in $(seq 1 60); do curl -sf localhost:14200/api/v1/companies >/dev/null && break; sleep 2; done
curl -s localhost:14200/api/v1/companies
curl -s -o /dev/null -w "%{http_code}\n" localhost:14200/
curl -s -o /dev/null -w "%{http_code}\n" localhost:14200/api/docs
docker compose -p bm-smoke ps
```

Expected: `[]`, `200`, `200`, alle drei Dienste `running`, `db` `healthy`.

Danach aufräumen: `docker compose -p bm-smoke down -v`.

- [x] **Step 4: Commit**

```bash
git add docker-compose.yml .env.example Taskfile.yml .gitignore
git commit -m "feat: Docker Compose für den kompletten Stack"
```

---

### Task 2: Demo-Daten (Seed)

**Files:** Create: `backend/cmd/seed/scenarios.go`, `backend/cmd/seed/scenarios_test.go`, `backend/cmd/seed/main.go`. Modify: `Taskfile.yml`.

- [x] **Step 1: Failing Test**

`backend/cmd/seed/scenarios_test.go`:

```go
package main

import (
	"testing"
	"time"

	"bewerbungsmanager/internal/domain"
)

var testToday = domain.DateOf(time.Date(2026, 10, 3, 12, 0, 0, 0, time.Local))

// Jedes Szenario muss die echten Domainregeln erfüllen, sonst lehnt die API es ab.
func TestScenariosAreValid(t *testing.T) {
	for _, s := range scenarios() {
		var history []domain.Event
		for i, st := range s.Steps {
			next := st.event(testToday)
			if err := domain.CanApply(history, next, testToday); err != nil {
				t.Errorf("%s / %s, Schritt %d (%s): %v", s.Company, s.Position, i, st.Type, err)
				break
			}
			// Der Seed erfasst alles heute: Erfassungsdatum = heute.
			history = append(history, domain.Event{Type: next.Type, OccurredOn: next.OccurredOn, RecordedOn: testToday})
		}
	}
}

// Die Demo-Daten sollen jede Ansicht sinnvoll füllen.
func TestScenariosCoverAllViews(t *testing.T) {
	all := scenarios()
	if len(all) < 25 {
		t.Errorf("nur %d Szenarien, erwartet mindestens 25", len(all))
	}

	has := map[string]bool{}
	sources := map[string]bool{}
	perCompany := map[string]int{}
	for _, s := range all {
		sources[s.Source] = true
		perCompany[s.Company]++
		last := s.Steps[len(s.Steps)-1]
		has[string(last.Type)] = true
		if last.DueDay != nil && *last.DueDay < 0 {
			has["überfällige Frist"] = true
		}
		if last.DueDay != nil && *last.DueDay >= 0 && *last.DueDay <= 7 {
			has["Frist in 7 Tagen"] = true
		}
		if last.Type.AllowsFutureDate() && last.Day >= 0 && last.Day <= 14 {
			has["Termin in 14 Tagen"] = true
		}
	}
	for _, want := range []string{
		"überfällige Frist", "Frist in 7 Tagen", "Termin in 14 Tagen",
		"Vorgemerkt", "Beworben", "AngebotErhalten", "AngebotAngenommen", "AngebotAbgelehnt",
		"Absage", "Zurueckgezogen", "KeineRueckmeldung",
	} {
		if !has[want] {
			t.Errorf("Demo-Daten decken %q nicht ab", want)
		}
	}
	if len(sources) < 4 {
		t.Errorf("nur %d Quellen, erwartet mindestens 4", len(sources))
	}
	multi := false
	for _, n := range perCompany {
		if n > 1 {
			multi = true
		}
	}
	if !multi {
		t.Error("mindestens eine Firma soll mehrere Bewerbungen haben")
	}
}
```

- [x] **Step 2: Test laufen lassen, er muss fehlschlagen**

Run: `cd backend && go test ./cmd/seed/ ; cd ..` → FAIL (`undefined: scenarios`).

- [x] **Step 3: Szenarien**

`backend/cmd/seed/scenarios.go`:

```go
package main

import (
	"time"

	"bewerbungsmanager/internal/domain"
)

// step ist ein Ereignis relativ zu heute: Day -3 = vor drei Tagen, +2 = übermorgen.
type step struct {
	Type   domain.EventType
	Day    int
	DueDay *int
	Note   string
}

type scenario struct {
	Company  string
	Position string
	Location string
	Source   string
	Steps    []step
}

func due(day int) *int { return &day }

func (s step) event(today time.Time) domain.NewEvent {
	e := domain.NewEvent{Type: s.Type, OccurredOn: today.AddDate(0, 0, s.Day)}
	if s.DueDay != nil {
		d := today.AddDate(0, 0, *s.DueDay)
		e.DueOn = &d
	}
	if s.Note != "" {
		note := s.Note
		e.Note = &note
	}
	return e
}

// scenarios liefert fiktive Bewerbungen. Firmen und Personen sind frei erfunden.
func scenarios() []scenario {
	const (
		b  = domain.Beworben
		v  = domain.Vorgemerkt
		sc = domain.ScreeningGespraech
		ce = domain.ChallengeErhalten
		ca = domain.ChallengeAbgegeben
		iv = domain.Interview
		kt = domain.Kennenlerntag
		ae = domain.AngebotErhalten
		an = domain.AngebotAngenommen
		al = domain.AngebotAbgelehnt
		ab = domain.Absage
		zu = domain.Zurueckgezogen
		kr = domain.KeineRueckmeldung
	)
	return []scenario{
		{"Acme GmbH", "Senior Go-Entwickler", "Berlin", "LinkedIn", []step{
			{Type: b, Day: -40}, {Type: sc, Day: -33, Note: "Telefonat mit HR, 30 Minuten"},
			{Type: iv, Day: -25, Note: "Fachgespräch mit Teamlead"}, {Type: iv, Day: -18, Note: "Systemdesign mit CTO"},
			{Type: kt, Day: -10}, {Type: ae, Day: -3, DueDay: due(4), Note: "Angebot schriftlich erhalten"},
		}},
		{"Acme GmbH", "Platform Engineer", "Berlin", "LinkedIn", []step{
			{Type: b, Day: -70}, {Type: ab, Day: -60, Note: "Stelle intern besetzt"},
		}},
		{"Nordlicht Software", "Backend Engineer", "Hamburg", "StepStone", []step{
			{Type: b, Day: -30}, {Type: ce, Day: -24, DueDay: due(-20), Note: "REST-API in Go mit Tests"},
			{Type: ca, Day: -21}, {Type: iv, Day: 3, Note: "Code-Review der Challenge"},
		}},
		{"Datenwerk Solutions", "Go Developer", "Remote", "LinkedIn", []step{
			{Type: b, Day: -20}, {Type: ce, Day: -2, DueDay: due(5), Note: "Take-Home: CLI-Tool"},
		}},
		{"Bergblick IT", "Softwareentwickler Java/Go", "München", "Empfehlung", []step{
			{Type: b, Day: -50}, {Type: iv, Day: -40}, {Type: ae, Day: -30, DueDay: due(-25)},
			{Type: an, Day: -26, Note: "Start zum Quartalsbeginn"},
		}},
		{"Hanse Logistik", "Plattform-Entwickler", "Bremen", "Firmenwebsite", []step{
			{Type: b, Day: -35}, {Type: ab, Day: -28},
		}},
		{"Pixelhaus Studio", "Fullstack-Entwickler Angular/Go", "Köln", "LinkedIn", []step{
			{Type: b, Day: -45}, {Type: sc, Day: -38}, {Type: ab, Day: -36, Note: "Mehr Frontend-Erfahrung gesucht"},
		}},
		{"Cloudkraft", "DevOps Engineer", "Remote", "StepStone", []step{
			{Type: b, Day: -60}, {Type: kr, Day: -20},
		}},
		{"Finanzblick Bank", "Backend Developer", "Frankfurt am Main", "Indeed", []step{
			{Type: v, Day: -5, DueDay: due(-1), Note: "Anschreiben noch anpassen"},
		}},
		{"Solarwerk Energie", "Go-Entwickler", "Freiburg", "Firmenwebsite", []step{
			{Type: v, Day: -3, DueDay: due(10)},
		}},
		{"Medicus Health", "Software Engineer", "Leipzig", "LinkedIn", []step{
			{Type: b, Day: -14}, {Type: sc, Day: 2, Note: "Videocall 10:00 Uhr"},
		}},
		{"Logiq Systems", "Backend-Entwickler", "Stuttgart", "StepStone", []step{
			{Type: b, Day: -25}, {Type: iv, Day: -15}, {Type: ab, Day: -8},
		}},
		{"Byteschmiede", "Junior Go-Entwickler", "Dresden", "Empfehlung", []step{
			{Type: b, Day: -12}, {Type: kt, Day: 6, Note: "Probetag vor Ort"},
		}},
		{"Mittelland Versicherung", "Anwendungsentwickler", "Nürnberg", "Indeed", []step{
			{Type: b, Day: -40}, {Type: ce, Day: -32, DueDay: due(-25)}, {Type: ca, Day: -26}, {Type: ab, Day: -18},
		}},
		{"Stadtwerke Nordheide", "IT-Entwickler", "Kiel", "Firmenwebsite", []step{
			{Type: b, Day: -9},
		}},
		{"Quantum Retail", "Go Backend Engineer", "Berlin", "LinkedIn", []step{
			{Type: b, Day: -7},
		}},
		{"Greenfield Mobility", "Software Developer", "München", "StepStone", []step{
			{Type: b, Day: -21}, {Type: sc, Day: -16}, {Type: kr, Day: -2},
		}},
		{"Kanzlei Digital", "Legal-Tech-Entwickler", "Hamburg", "Empfehlung", []step{
			{Type: v, Day: -10}, {Type: zu, Day: -8, Note: "Passt fachlich nicht"},
		}},
		{"Orbit Games", "Backend Programmer", "Remote", "LinkedIn", []step{
			{Type: b, Day: -28}, {Type: iv, Day: -20}, {Type: ae, Day: -12, DueDay: due(-5)},
			{Type: al, Day: -6, Note: "Gehalt unter Erwartung"},
		}},
		{"Rheinwerk Consulting", "Go Consultant", "Düsseldorf", "Indeed", []step{
			{Type: b, Day: -16}, {Type: ab, Day: -15},
		}},
		{"Tierisch Gut", "Webentwickler", "Hannover", "Firmenwebsite", []step{
			{Type: b, Day: -4},
		}},
		{"Wolkenlos SaaS", "Senior Backend Engineer", "Remote", "LinkedIn", []step{
			{Type: b, Day: -33}, {Type: sc, Day: -27}, {Type: ce, Day: -22, DueDay: due(-15)},
			{Type: ca, Day: -16}, {Type: iv, Day: -9}, {Type: kr, Day: -1},
		}},
		{"Fahrrad Digital", "Software Engineer", "Münster", "StepStone", []step{
			{Type: b, Day: -19}, {Type: iv, Day: 9},
		}},
		{"Konsum IT", "Entwickler E-Commerce", "Bielefeld", "Indeed", []step{
			{Type: b, Day: -55}, {Type: ab, Day: -50},
		}},
		{"Eisvogel Labs", "Go-Entwickler", "Potsdam", "Empfehlung", []step{
			{Type: b, Day: -11}, {Type: ce, Day: -6, DueDay: due(1)},
		}},
		{"Hafen Data", "Data Engineer (Go)", "Hamburg", "LinkedIn", []step{
			{Type: b, Day: -26}, {Type: zu, Day: -10, Note: "Anderes Angebot angenommen"},
		}},
	}
}
```

- [x] **Step 4: Tests laufen lassen**

Run: `cd backend && go test ./cmd/seed/ -v ; cd ..` → beide Tests PASS.
Schlägt `TestScenariosAreValid` fehl, nur das betroffene Szenario anpassen. Die Testlogik bleibt unverändert.

- [x] **Step 5: HTTP-Seeder**

`backend/cmd/seed/main.go`:

```go
// Command seed legt Demo-Daten über die REST-API an, damit alle Fachregeln greifen.
//
//	go run ./cmd/seed -url http://localhost:4200
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"bewerbungsmanager/internal/domain"
)

func main() {
	baseURL := flag.String("url", "http://localhost:4200", "Basis-URL der App (Frontend-Proxy oder Backend)")
	flag.Parse()

	if err := run(*baseURL, domain.DateOf(time.Now())); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(baseURL string, today time.Time) error {
	api := &apiClient{base: baseURL, http: &http.Client{Timeout: 10 * time.Second}}

	var existing []struct{ ID string }
	if err := api.do(http.MethodGet, "/api/v1/companies", nil, &existing); err != nil {
		return fmt.Errorf("API nicht erreichbar (läuft der Stack? `task up`): %w", err)
	}
	if len(existing) > 0 {
		return errors.New("es gibt bereits Daten; Demo-Daten nur in eine leere Datenbank laden (`docker compose down -v` setzt zurück)")
	}

	companyIDs := map[string]string{}
	events := 0
	for _, s := range scenarios() {
		companyID, ok := companyIDs[s.Company]
		if !ok {
			var created struct{ ID string `json:"id"` }
			if err := api.do(http.MethodPost, "/api/v1/companies", map[string]any{"name": s.Company}, &created); err != nil {
				return fmt.Errorf("Firma %s: %w", s.Company, err)
			}
			companyID = created.ID
			companyIDs[s.Company] = companyID
		}

		var app struct{ ID string `json:"id"` }
		body := map[string]any{
			"company_id":     companyID,
			"position_title": s.Position,
			"location":       s.Location,
			"source":         s.Source,
			"first_event":    eventPayload(s.Steps[0], today),
		}
		if err := api.do(http.MethodPost, "/api/v1/applications", body, &app); err != nil {
			return fmt.Errorf("Bewerbung %s / %s: %w", s.Company, s.Position, err)
		}
		events++

		for _, st := range s.Steps[1:] {
			if err := api.do(http.MethodPost, "/api/v1/applications/"+app.ID+"/events", eventPayload(st, today), nil); err != nil {
				return fmt.Errorf("Ereignis %s bei %s / %s: %w", st.Type, s.Company, s.Position, err)
			}
			events++
		}
	}

	fmt.Printf("Demo-Daten angelegt: %d Firmen, %d Bewerbungen, %d Ereignisse.\n", len(companyIDs), len(scenarios()), events)
	return nil
}

func eventPayload(s step, today time.Time) map[string]any {
	e := s.event(today)
	p := map[string]any{"type": string(e.Type), "occurred_on": e.OccurredOn.Format(time.DateOnly)}
	if e.DueOn != nil {
		p["due_on"] = e.DueOn.Format(time.DateOnly)
	}
	if e.Note != nil {
		p["note"] = *e.Note
	}
	return p
}

type apiClient struct {
	base string
	http *http.Client
}

// do sendet body als JSON und dekodiert die Antwort nach out (falls nicht nil).
func (c *apiClient) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, res.StatusCode, bytes.TrimSpace(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
```

- [x] **Step 6: Taskfile**

```yaml
  seed:
    desc: Lädt Demo-Daten in den laufenden Stack (nur bei leerer Datenbank)
    dir: backend
    cmds:
      - go run ./cmd/seed -url {{.URL | default "http://localhost:4200"}}
```

- [x] **Step 7: Gegen den echten Stack prüfen**

```bash
APP_PORT=14200 docker compose -p bm-smoke up -d --build
for i in $(seq 1 60); do curl -sf localhost:14200/api/v1/companies >/dev/null && break; sleep 2; done
task seed URL=http://localhost:14200
curl -s localhost:14200/api/v1/stats/funnel
curl -s "localhost:14200/api/v1/deadlines" | head -c 400; echo
curl -s "localhost:14200/api/v1/appointments" | head -c 400; echo
task seed URL=http://localhost:14200   # zweiter Lauf muss mit Hinweis abbrechen
docker compose -p bm-smoke down -v
```

Expected:
- Der erste Lauf meldet „Demo-Daten angelegt: 25 Firmen, 26 Bewerbungen, …“.
- Der Funnel hat bei „Beworben“ 23 erreichte Bewerbungen (26 minus 3 nur vorgemerkte). Es gibt überfällige und anstehende Fristen sowie Termine.
- Der zweite Lauf bricht mit dem Hinweis auf vorhandene Daten ab, mit Exit-Code ≠ 0.

- [x] **Step 8: Commit**

```bash
git add backend/cmd/seed Taskfile.yml
git commit -m "feat: Demo-Daten über die API mit relativen Datumswerten"
```

---

### Task 3: Screenshots per Playwright

**Files:** Create: `tools/screenshots/package.json`, `tools/screenshots/shoot.mjs`, `docs/screenshots/*.png`. Modify: `Taskfile.yml`, `.gitignore`.

- [x] **Step 1: Werkzeug**

`tools/screenshots/package.json`:

```json
{
  "name": "bewerbungsmanager-screenshots",
  "private": true,
  "type": "module",
  "scripts": {
    "shoot": "node shoot.mjs"
  },
  "devDependencies": {
    "playwright": "^1.50.0"
  }
}
```

`tools/screenshots/shoot.mjs`:

```js
// Erstellt README-Screenshots aus dem laufenden Stack mit Demo-Daten.
//   BASE_URL=http://localhost:4200 npm run shoot
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const base = process.env.BASE_URL ?? 'http://localhost:4200';
const outDir = fileURLToPath(new URL('../../docs/screenshots/', import.meta.url));
await mkdir(outDir, { recursive: true });

const offers = await (await fetch(`${base}/api/v1/applications?status=AngebotErhalten`)).json();
if (!Array.isArray(offers) || offers.length === 0) {
  throw new Error('Keine Demo-Daten gefunden – vorher `task seed` ausführen.');
}

const pages = [
  { name: 'dashboard', path: '/' },
  { name: 'bewerbungen', path: '/bewerbungen' },
  { name: 'detail', path: `/bewerbungen/${offers[0].id}` },
  { name: 'statistik', path: '/statistik' },
  { name: 'neu', path: '/bewerbungen/neu' },
];
const viewports = [
  { suffix: 'desktop', viewport: { width: 1280, height: 860 }, scale: 1, only: null },
  { suffix: 'mobile', viewport: { width: 390, height: 844 }, scale: 2, only: ['dashboard', 'detail'] },
];

const browser = await chromium.launch();
try {
  for (const vp of viewports) {
    const context = await browser.newContext({
      viewport: vp.viewport,
      deviceScaleFactor: vp.scale,
      locale: 'de-DE',
      timezoneId: 'Europe/Berlin',
      colorScheme: 'light',
    });
    const page = await context.newPage();
    for (const p of pages) {
      if (vp.only && !vp.only.includes(p.name)) continue;
      await page.goto(base + p.path, { waitUntil: 'networkidle' });
      await page.waitForTimeout(400); // Animationen von Material abklingen lassen
      const file = `${outDir}${p.name}-${vp.suffix}.png`;
      await page.screenshot({ path: file, fullPage: true });
      console.log('gespeichert:', file);
    }
    await context.close();
  }
} finally {
  await browser.close();
}
```

In `.gitignore` ergänzen: `tools/screenshots/node_modules/`.

- [x] **Step 2: Taskfile**

```yaml
  screenshots:
    desc: Erstellt README-Screenshots (Stack muss laufen und Demo-Daten enthalten)
    dir: tools/screenshots
    cmds:
      - npm install --no-audit --no-fund
      - npx playwright install chromium
      - BASE_URL={{.URL | default "http://localhost:4200"}} npm run shoot
```

- [x] **Step 3: Screenshots erzeugen**

```bash
APP_PORT=14200 docker compose -p bm-smoke up -d --build
for i in $(seq 1 60); do curl -sf localhost:14200/api/v1/companies >/dev/null && break; sleep 2; done
task seed URL=http://localhost:14200
task screenshots URL=http://localhost:14200
docker compose -p bm-smoke down -v
ls -la docs/screenshots
```

Expected: sieben PNGs:
- `dashboard-desktop.png`, `bewerbungen-desktop.png`, `detail-desktop.png`, `statistik-desktop.png`, `neu-desktop.png`
- `dashboard-mobile.png`, `detail-mobile.png`

Zur Kontrolle jedes Bild mit dem Read-Tool ansehen. Die Seiten müssen gefüllt sein, also nicht „Lade …“ und kein Fehlertext. Ist eine Seite leer, `waitForTimeout` erhöhen oder auf ein Element warten, z. B. `page.waitForSelector('h1')`.

- [x] **Step 4: Commit**

```bash
git add tools/screenshots/package.json tools/screenshots/package-lock.json tools/screenshots/shoot.mjs docs/screenshots Taskfile.yml .gitignore
git commit -m "docs: reproduzierbare Screenshots per Playwright"
```

---

### Task 4: Linter und CI

**Files:** Create: `backend/.golangci.yml`, `.github/workflows/ci.yml`. Modify: `Taskfile.yml`, ggf. Backend-Code für Linter-Funde.

- [x] **Step 1: Linter-Konfiguration**

`backend/.golangci.yml`:

```yaml
version: "2"

linters:
  default: standard
  enable:
    - bodyclose
    - errorlint
    - misspell
    - unconvert

formatters:
  enable:
    - gofmt
```

Generierter Code (Header „Code generated … DO NOT EDIT“) wird von golangci-lint automatisch übersprungen.

- [x] **Step 2: Lokal ausführen und Funde beheben**

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
cd backend && golangci-lint run ./... ; cd ..
```

Jeden Fund im handgeschriebenen Code beheben, ohne Verhalten zu ändern. `misspell` ist auf Englisch eingestellt. Meldet er deutsche Wörter in Kommentaren oder Strings, diese per `misspell`-Option `ignore-rules` ausnehmen, statt Texte zu ändern. Danach `go test ./...` im Backend → grün.

In `Taskfile.yml` ergänzen:

```yaml
  lint:
    desc: Linter für das Backend
    dir: backend
    cmds:
      - golangci-lint run ./...
```

- [x] **Step 3: CI-Workflow**

`.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [master]
  pull_request:

permissions:
  contents: read

jobs:
  backend:
    name: Backend (Lint + Tests)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v8
        with:
          version: latest
          working-directory: backend
      - name: Tests (mit Postgres per testcontainers)
        working-directory: backend
        run: go test ./...

  frontend:
    name: Frontend (Tests + Build)
    runs-on: ubuntu-latest
    env:
      NG_CLI_ANALYTICS: "false"
      CI: "true"
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: npm ci
      - run: npx ng test --watch=false
      - run: npx ng build

  codegen:
    name: Generierter Code aktuell
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - run: npm ci
        working-directory: frontend
      - run: go install github.com/go-task/task/v3/cmd/task@latest
      - run: task generate
      - name: Keine Abweichung zum eingecheckten Code
        run: |
          git status --porcelain
          test -z "$(git status --porcelain)"

  images:
    name: Docker-Images bauen
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker build -t bewerbungsmanager-backend ./backend
      - run: docker build -t bewerbungsmanager-frontend ./frontend
```

- [x] **Step 4: Workflow lokal plausibilisieren**

Den Workflow kann man ohne GitHub nicht ausführen. Lokal laufen deshalb die gleichen Befehle:

```bash
task lint
task test
task test:frontend
task generate && test -z "$(git status --porcelain)" && echo "codegen ok"
docker build -t bewerbungsmanager-backend ./backend && docker build -t bewerbungsmanager-frontend ./frontend
```

Expected: alles grün bzw. `codegen ok`. Die YAML-Syntax zusätzlich prüfen mit `npx -y yaml-lint .github/workflows/ci.yml` (oder `npx -y js-yaml .github/workflows/ci.yml > /dev/null`).

- [x] **Step 5: Commit**

```bash
git add backend .github Taskfile.yml
git commit -m "ci: Lint, Tests, Codegen-Prüfung und Image-Builds per GitHub Actions"
```

---

### Task 5: Architecture Decision Records

**Files:** Create: `docs/decisions/0001-ereignisse-als-wahrheit.md`, `0002-spec-first-openapi.md`, `0003-sqlc-statt-orm.md`, `0004-integrationstests-mit-echtem-postgres.md`.

- [ ] **Step 1: ADRs schreiben**

`docs/decisions/0001-ereignisse-als-wahrheit.md`:

```markdown
# 0001 – Ereignisse als Wahrheit statt Statusfeld

Status: angenommen · 2026-10-02

## Kontext

Eine Bewerbung durchläuft viele Schritte (Beworben, Screening, Challenge, Interviews, Angebot …).
Für Übersicht und Statistik zählt nicht nur der aktuelle Stand, sondern der Weg dorthin:
Wie lange dauert die erste Antwort? In welcher Phase kommen Absagen?

## Entscheidung

Jeder Statuswechsel wird als Ereignis in `application_events` gespeichert. Die Tabelle ist
append-only; einzige Ausnahme ist „Rückgängig“, das das jeweils letzte Ereignis löscht.
`applications.current_status` ist nur ein Cache, der in derselben Transaktion wie das Ereignis
geschrieben wird. Erlaubte Übergänge prüft ein reiner Zustandsautomat im Paket `domain`
(ohne Datenbank, vollständig tabellengetrieben getestet).

Bewusst **kein** volles Event Sourcing mit Projektionen und Versionierung: Für eine
Einzelnutzer-App mit einigen hundert Datensätzen wäre das Aufwand ohne Nutzen.

## Konsequenzen

- Statistik (Funnel, Antwortzeiten, Absagen je Phase) entsteht direkt aus den Ereignissen.
- Regeln liegen an genau einer Stelle; das Frontend fragt erlaubte Ereignisse beim Server ab.
- Parallele Schreibzugriffe werden per `SELECT … FOR UPDATE` auf die Bewerbung serialisiert.
- Korrekturen sind nur über „Rückgängig“ möglich, nicht durch Bearbeiten alter Ereignisse.
```

`docs/decisions/0002-spec-first-openapi.md`:

```markdown
# 0002 – Spec-first mit OpenAPI

Status: angenommen · 2026-10-02

## Kontext

Backend (Go) und Frontend (Angular) müssen dieselbe API sprechen. Handgeschriebene DTOs auf
beiden Seiten laufen erfahrungsgemäß auseinander.

## Entscheidung

`api/openapi.yaml` ist der Vertrag. Daraus werden erzeugt:

- der Go-Server (oapi-codegen, „strict server“): Handler können nur die im Vertrag definierten
  Antworten liefern;
- der Angular-Client (ng-openapi-gen), auf den Komponenten über eine dünne Fassade
  (`core/api.ts`) zugreifen – die Fassade ist die einzige Stelle, die beim Generator-Update
  angepasst werden muss, und lässt sich in Tests einfach ersetzen.

Requests werden zusätzlich zur Laufzeit gegen die Spec validiert (kin-openapi). Fehler sind
einheitlich `application/problem+json` (RFC 9457).

## Konsequenzen

- Eine API-Änderung beginnt in der YAML-Datei; `task generate` erzeugt beide Seiten neu.
- Die CI prüft, dass der eingecheckte generierte Code aktuell ist.
- OpenAPI 3.0.3 statt 3.1, weil oapi-codegen 3.1 nicht zuverlässig unterstützt.
```

`docs/decisions/0003-sqlc-statt-orm.md`:

```markdown
# 0003 – sqlc statt ORM

Status: angenommen · 2026-10-02

## Kontext

Die Abfragen sind überschaubar, aber nicht trivial (View für das letzte Ereignis je
Bewerbung, Filter mit optionalen Parametern, Sperren mit `FOR UPDATE`).

## Entscheidung

SQL wird von Hand geschrieben (`internal/store/queries/*.sql`); sqlc erzeugt daraus
typsichere Go-Funktionen. Migrationen verwaltet goose, eingebettet ins Binary und beim Start
ausgeführt.

## Konsequenzen

- Jede Abfrage ist als SQL lesbar und reviewbar; keine versteckten N+1-Probleme.
- Schemaänderungen und Queries werden zur Generierungszeit gegeneinander geprüft.
- sqlc läuft per Docker-Image, damit keine CGO-Toolchain unter Windows nötig ist.
```

`docs/decisions/0004-integrationstests-mit-echtem-postgres.md`:

```markdown
# 0004 – Integrationstests mit echtem Postgres

Status: angenommen · 2026-10-03

## Kontext

Die interessanten Fehler liegen an der Grenze zur Datenbank: Transaktionen, Sperren,
die View `latest_events`, Sortierung, Constraints. Mocks für SQL beweisen dort nichts.

## Entscheidung

Service- und HTTP-Tests laufen gegen ein echtes Postgres 17, gestartet per testcontainers-go
(ein Container pro Testpaket, Tabellen pro Test geleert). Reine Fachlogik (`domain`, `stats`)
wird ohne Datenbank getestet.

## Konsequenzen

- Tests brauchen Docker – lokal wie in der CI (GitHub-Runner bringen es mit).
- Unter Windows erkennt testcontainers Docker per `os.Stat` auf die Named Pipe; bei parallel
  laufenden Testpaketen scheitert das mit „All pipe instances are busy“. `internal/testdb`
  setzt deshalb unter Windows `DOCKER_HOST` explizit.
```

- [ ] **Step 2: Commit**

```bash
git add docs/decisions
git commit -m "docs: Architekturentscheidungen als ADRs"
```

---

### Task 6: README

**Files:** Create: `README.md`.

- [ ] **Step 1: README schreiben**

`README.md` ist für zwei Leser gedacht. Recruiter verstehen in 30 Sekunden, was das Projekt ist und kann. Entwickler sehen Architektur, Entscheidungen und wie man es startet. Inhalt:

````markdown
# Bewerbungs-Tracker

Verwaltet die eigenen Bewerbungen – Firma, Stelle, Status und Fristen. Jeder Statuswechsel
(beworben, Challenge erhalten, Interview, Angebot …) wird als Ereignis gespeichert; daraus
entstehen Dashboard, Verlauf und Statistik.

**Go · PostgreSQL · REST/OpenAPI · Angular 22 · Docker**

![Dashboard](docs/screenshots/dashboard-desktop.png)

## Funktionen

- **Dashboard:** überfällige Fristen, Fristen der nächsten 7 Tage, anstehende Termine, Kennzahlen
- **Bewerbungen:** Liste mit Filtern und Suche, Anlegen mit Firmen-Autocomplete
- **Verlauf:** Timeline je Bewerbung; nur fachlich erlaubte nächste Schritte werden angeboten,
  Fristen nur dort, wo sie Sinn ergeben (Bewerbungsschluss, Challenge-Abgabe, Antwort auf Angebot)
- **Statistik:** Funnel, Tage bis zur ersten Antwort, Absagen je Phase, Erfolg je Quelle
- **API-Doku:** Swagger UI unter `/api/docs`

| Bewerbung im Detail | Statistik |
|---|---|
| ![Detail](docs/screenshots/detail-desktop.png) | ![Statistik](docs/screenshots/statistik-desktop.png) |

<p>
  <img src="docs/screenshots/dashboard-mobile.png" alt="Dashboard auf dem Handy" width="240">
  <img src="docs/screenshots/detail-mobile.png" alt="Detail auf dem Handy" width="240">
</p>

## Schnellstart

Voraussetzung: Docker.

```bash
docker compose up -d --build        # http://localhost:4200
go -C backend run ./cmd/seed        # optional: fiktive Demo-Daten (benötigt Go)
```

Port ändern: `APP_PORT=8081 docker compose up -d`. Zurücksetzen: `docker compose down -v`.

## Architektur

```mermaid
flowchart LR
  B[Browser] -->|HTTP| N[nginx<br/>Angular-App]
  N -->|/api| G[Go-Backend<br/>net/http · oapi-codegen]
  G --> D[(PostgreSQL 17)]
  S[api/openapi.yaml] -. generiert .-> G
  S -. generiert .-> N
```

- **Ereignisse als Wahrheit:** `application_events` ist append-only, der aktuelle Status ein
  Cache, der in derselben Transaktion geschrieben wird. Die Übergangsregeln stecken in einem
  reinen Zustandsautomaten (`backend/internal/domain`).
- **Spec-first:** Server und Angular-Client werden aus `api/openapi.yaml` erzeugt; die CI
  prüft, dass der generierte Code aktuell ist.
- **Schichten im Backend:** `httpapi` (generierter Strict-Server, Problem-JSON) → `service`
  (Transaktionen) → `domain` / `stats` (reine Logik); Datenzugriff über sqlc.
- **Frontend:** Standalone-Komponenten, Signals, zoneless; Komponenten sprechen nur mit einer
  dünnen Fassade über dem generierten Client.

Die Begründungen stehen in den [Architekturentscheidungen](docs/decisions/).

## Entwicklung

Werkzeuge: Go 1.27, Node 24, Docker, [Task](https://taskfile.dev).

```bash
task generate        # sqlc, Go-Server und Angular-Client aus Spec/SQL erzeugen
task test            # Backend-Tests (Domain, Service, HTTP – gegen echtes Postgres)
task test:frontend   # Frontend-Tests (Vitest)
task lint            # golangci-lint
task up / task down  # kompletten Stack starten/stoppen
task seed            # Demo-Daten laden
task screenshots     # README-Screenshots neu erzeugen
```

Backend und Frontend einzeln mit Live-Reload:

```bash
docker run -d --name bm-db -e POSTGRES_PASSWORD=dev -e POSTGRES_DB=bewerbungen -p 5432:5432 postgres:17-alpine
DATABASE_URL="postgres://postgres:dev@localhost:5432/bewerbungen?sslmode=disable" PORT=18080 go -C backend run ./cmd/server
npm --prefix frontend start          # http://localhost:4200, /api → localhost:18080
```

## Projektstruktur

```
api/openapi.yaml        API-Vertrag
backend/                Go: cmd/server, cmd/seed, internal/{domain,stats,service,store,httpapi,db}
frontend/               Angular: src/app/{core,shared,features}, generierter Client in src/app/api
docs/decisions/         Architekturentscheidungen (ADRs)
docs/superpowers/       Design-Spezifikation und Umsetzungspläne
```

## Tests

- **Domain & Statistik:** tabellengetriebene Tests ohne Datenbank, inkl. aller erlaubten und
  verbotenen Statusübergänge
- **Service & HTTP:** Integrationstests gegen PostgreSQL in Docker (testcontainers)
- **Frontend:** Komponententests mit Vitest – z. B. dass nur erlaubte Statuswechsel als Buttons
  erscheinen und das Fristfeld nur bei passenden Ereignissen
- **Demo-Daten:** jedes Szenario wird gegen den echten Zustandsautomaten geprüft

Alle Firmen und Personen in den Demo-Daten sind frei erfunden.
````

Den Port-Hinweis im Abschnitt „Backend und Frontend einzeln“ so lassen: `proxy.conf.json` zeigt auf 18080.

`npm --prefix frontend start` setzt voraus, dass `frontend/package.json` ein Skript `start` hat. Das legt `ng new` standardmäßig an. Prüfen und notfalls `npx ng serve` im README verwenden.

`go -C` gibt es seit Go 1.20.

- [ ] **Step 2: Prüfen**

- Alle Links und Bildpfade existieren: `docs/screenshots/*.png` und `docs/decisions/`.
- Alle Befehle im README stimmen mit Taskfile und Compose überein.
- Mermaid-Syntax mit einem Online-Renderer (z. B. mermaid.live) oder per Augenmaß prüfen. GitHub rendert `flowchart LR` nativ.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: README mit Screenshots, Architektur und Schnellstart"
```

---

### Task 7: Auf GitHub veröffentlichen (mit dem Nutzer)

Dieser Task braucht den Nutzer. Er legt auf github.com ein **leeres** Repository an, ohne README und ohne Lizenz, und gibt die URL weiter.

- [ ] **Step 1: Remote setzen und pushen** (erst nach ausdrücklichem Okay)

```bash
git remote add origin <URL>
git push -u origin master
```

- [ ] **Step 2: CI beobachten**

Bei einem öffentlichen Repository geht das ohne Login:
`curl -s "https://api.github.com/repos/<owner>/<repo>/actions/runs?per_page=1" | grep -E '"status"|"conclusion"|"html_url"' | head`

Läuft ein Job rot, das Log über den `html_url`-Link ansehen und den Fehler beheben. Der Nutzer kann dazu das Log weitergeben, oder bei öffentlichen Repos geht es über die API unter `/actions/runs/<id>/jobs`. Typische Kandidaten sind die Action-Versionen (`setup-go`, `golangci-lint-action`), die golangci-lint-Version und Rechte für Docker.

- [ ] **Step 3: CI-Badge**

Wenn die CI grün ist, ganz oben im README unter der Überschrift ergänzen:
`[![CI](https://github.com/<owner>/<repo>/actions/workflows/ci.yml/badge.svg)](https://github.com/<owner>/<repo>/actions/workflows/ci.yml)`. Dann committen und pushen.

---

## Abschluss-Check

- [ ] `task lint`, `task test`, `task test:frontend` grün
- [ ] `task generate && git status --porcelain` leer
- [ ] `docker compose up -d --build` startet sauber; nach `task seed` zeigen Dashboard und Statistik Daten
- [ ] README-Bilder sind aktuell und zeigen gefüllte Seiten
