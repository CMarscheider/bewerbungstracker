# Bewerbungs-Tracker – Design

Stand: 2026-10-02

## Ziel

Eine Web-App zum Verwalten der eigenen Bewerbungen: Firma, Stelle, Status und Fristen. Jeder Statuswechsel wird als Ereignis gespeichert; daraus entstehen Übersicht, Fristenliste und Statistik.

Das Projekt dient zwei Zwecken gleichzeitig: als echtes Werkzeug für die eigene Jobsuche und als Portfolio-Projekt, das sauberes Go, Postgres, REST/OpenAPI und Angular zeigt.

### Rahmenbedingungen

- Ein Nutzer, kein Login. Läuft lokal.
- Betrieb per Docker Compose (`docker compose up --build`).
- Fristen werden nur in der App angezeigt (keine E-Mails, kein Kalender-Feed).

### Bewusst nicht enthalten

Login, mehrere Nutzer, E-Mail-Erinnerungen, iCal-Feed, Datei-Uploads (Lebenslauf, Anschreiben), Kontaktpersonen, Gehaltsfelder, E2E-Browsertests.

## Grundsatzentscheidungen

1. **Ereignisse sind die Wahrheit.** `application_events` ist append-only und die einzige Quelle des Status. `applications.current_status` ist ein Cache, der in derselben Transaktion wie das Ereignis geschrieben wird und jederzeit aus den Ereignissen neu berechnet werden kann. Kein volles Event Sourcing mit Projektionen.
2. **Spec-first.** `api/openapi.yaml` ist der Vertrag. Go-Server-Code (oapi-codegen, strict server) und Angular-Client werden daraus generiert.
3. **sqlc statt ORM.** Typsicheres, handgeschriebenes SQL; Migrationen mit goose.
4. **Übergangsregeln nur im Backend.** Das Frontend fragt erlaubte nächste Ereignisse beim Server ab und dupliziert keine Regeln.

## Statusmodell

### Ereignistypen

| Typ | Phase | Reihenfolge | Frist möglich | Datum in Zukunft erlaubt |
|---|---|---|---|---|
| `Vorgemerkt` | Vorbereitung | 0 | ja (Bewerbungsschluss) | nein |
| `Beworben` | Aktiv | 1 | nein | nein |
| `ScreeningGespraech` | Aktiv | 2 | nein | ja (Termin) |
| `ChallengeErhalten` | Aktiv | 3 | ja (Abgabe) | nein |
| `ChallengeAbgegeben` | Aktiv | 4 | nein | nein |
| `Interview` | Aktiv | 5 | nein | ja (Termin) |
| `Kennenlerntag` | Aktiv | 6 | nein | ja (Termin) |
| `AngebotErhalten` | Aktiv | 7 | ja (Antwortfrist) | nein |
| `AngebotAngenommen` | Abgeschlossen | – | nein | nein |
| `AngebotAbgelehnt` | Abgeschlossen | – | nein | nein |
| `Absage` | Abgeschlossen | – | nein | nein |
| `Zurueckgezogen` | Abgeschlossen | – | nein | nein |
| `KeineRueckmeldung` | Abgeschlossen (wiedereröffenbar) | – | nein | nein |

Der aktuelle Status einer Bewerbung ist der Typ ihres zuletzt **erfassten** Ereignisses (Reihenfolge nach `created_at`). `occurred_on` ist das fachliche Datum und kann bei Terminen in der Zukunft liegen; es bestimmt nicht die Reihenfolge.

### Übergangsregeln

`CanApply(history, next)` erlaubt ein Ereignis genau dann, wenn eine der folgenden Regeln greift:

1. **Erstes Ereignis:** nur `Vorgemerkt` oder `Beworben`.
2. **Nach `Vorgemerkt`:** nur `Beworben` oder `Zurueckgezogen`.
3. **Nach einem aktiven Ereignis außer `AngebotErhalten`** (Reihenfolge 1–6):
   - jedes aktive Ereignis mit höherer Reihenfolge (Vorwärtssprünge erlaubt), mit der Einschränkung, dass `ChallengeAbgegeben` nur direkt nach `ChallengeErhalten` erlaubt ist;
   - `Interview` auch nach `Interview` (weitere Runde);
   - `Absage`, `Zurueckgezogen`, `KeineRueckmeldung`.
4. **Nach `AngebotErhalten`:** `AngebotAngenommen`, `AngebotAbgelehnt`, `Absage` (Angebot zurückgezogen), `Zurueckgezogen`.
5. **Nach `KeineRueckmeldung`:** alles, was nach dem letzten Ereignis *vor* `KeineRueckmeldung` erlaubt wäre, außer `KeineRueckmeldung` selbst.
6. **Nach `AngebotAngenommen`, `AngebotAbgelehnt`, `Absage`, `Zurueckgezogen`:** nichts.

Zusätzliche Validierung (Fehler → `422`):

- `occurred_on` darf nicht vor dem früheren der beiden Daten des bisher letzten Ereignisses liegen: dessen `occurred_on` oder dessen Erfassungsdatum (`created_at` als Datum). Im Normalfall ist das `occurred_on`; bei einem im Voraus eingetragenen Termin das Erfassungsdatum – so ist z. B. eine Absage vor einem geplanten Interview möglich.
- `occurred_on` darf nur bei Typen mit „Datum in Zukunft erlaubt“ nach heute liegen.
- `due_on` nur bei Typen mit „Frist möglich“; sonst `400`.

`interview_round` wird vom Server vergeben (Anzahl bisheriger `Interview`-Ereignisse + 1) und kann nicht vom Client gesetzt werden.

### Rückgängig

Nur das jeweils letzte Ereignis darf gelöscht werden. Danach wird `current_status` aus dem neuen letzten Ereignis neu berechnet. Das erste Ereignis kann nicht gelöscht werden (`422`); stattdessen wird die Bewerbung gelöscht.

### Fristen

Eine Frist ist das `due_on` eines Ereignisses. Sie ist **offen**, wenn ihr Ereignis das letzte Ereignis der Bewerbung ist. Sie ist **überfällig**, wenn sie offen ist und `due_on < heute`. Mit dem nächsten Ereignis (z. B. `ChallengeAbgegeben`) erledigt sich die Frist automatisch.

**Anstehende Termine** sind letzte Ereignisse vom Typ `ScreeningGespraech`, `Interview` oder `Kennenlerntag` mit `occurred_on >= heute`.

## Datenmodell (Postgres 17)

```sql
companies (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name        text NOT NULL UNIQUE,
  website     text,
  notes       text,
  created_at  timestamptz NOT NULL DEFAULT now()
)

applications (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id      uuid NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
  position_title  text NOT NULL,
  job_url         text,
  location        text,
  source          text,          -- z. B. LinkedIn, StepStone, Empfehlung
  notes           text,
  current_status  text NOT NULL, -- Cache, siehe Grundsatzentscheidung 1
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
)

application_events (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id   uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  type             text NOT NULL CHECK (type IN (... 13 Typen ...)),
  occurred_on      date NOT NULL,
  due_on           date,
  interview_round  int,
  note             text,
  created_at       timestamptz NOT NULL DEFAULT now()
)
-- Index auf application_events(application_id, created_at)
-- created_at DEFAULT clock_timestamp(), damit mehrere Ereignisse in einer Transaktion eindeutig sortiert sind

latest_events   -- VIEW: letztes Ereignis je Bewerbung (DISTINCT ON application_id ORDER BY created_at DESC)
                -- Grundlage für Listenansicht, Fristen und Termine
```

## REST-API (`/api/v1`, OpenAPI 3.0.3 – oapi-codegen unterstützt 3.1 nicht zuverlässig)

| Methode | Pfad | Zweck |
|---|---|---|
| GET | `/companies` | Liste inkl. Anzahl Bewerbungen |
| POST | `/companies` | Anlegen; doppelter Name → `409` |
| GET / PATCH / DELETE | `/companies/{id}` | Löschen nur ohne Bewerbungen, sonst `409` |
| GET | `/applications?phase=&status=&q=` | Liste; `q` sucht in Firma und Stelle |
| POST | `/applications` | Anlegen inkl. erstem Ereignis (`Vorgemerkt` oder `Beworben`) |
| GET | `/applications/{id}` | Details inkl. Ereignis-Timeline |
| PATCH | `/applications/{id}` | Nur Stammdaten, nicht den Status |
| DELETE | `/applications/{id}` | Löscht inkl. Ereignissen |
| POST | `/applications/{id}/events` | Neues Ereignis, geprüft durch Zustandsautomat |
| DELETE | `/applications/{id}/events/latest` | Rückgängig |
| GET | `/applications/{id}/allowed-events` | Erlaubte nächste Ereignistypen |
| GET | `/deadlines?within_days=7` | Offene Fristen bis heute + N Tage, inkl. überfälliger; `within_days` 0–365 (Standard 7) |
| GET | `/appointments?within_days=14` | Anstehende Termine; `within_days` 0–365 (Standard 14) |
| GET | `/stats/funnel` | Funnel mit Anzahl und Quote je Stufe |
| GET | `/stats/summary` | Antwortzeiten, Absagen je Phase, Erfolg je Quelle |

Zusätzlich liefert das Backend die eingebettete Spec unter `/api/openapi.json` und eine Swagger-UI unter `/api/docs` aus.

### Fehler

Alle Fehler als `application/problem+json` (RFC 9457):

- `400`: Request ungültig (Format, Pflichtfelder, `due_on` bei falschem Typ)
- `404`: Ressource nicht gefunden
- `409`: Konflikt (Firmenname doppelt, Firma mit Bewerbungen löschen)
- `422`: fachlich unerlaubt, mit Erweiterungsfeldern, z. B. `{"type": "/problems/invalid-transition", "from": "Absage", "attempted": "Interview"}`. `RuleError`-Codes: `date-in-future`, `date-before-previous`, `cannot-undo-first-event`, `unknown-company`. Problem-Typen sind relativ (`/problems/<code>`).
- `500`: unerwartet; Details nur im Log

Unbekannte `/api`-Routen liefern ein `404`-Problem. Das Request-Body-Limit beträgt 1 MiB.

## Statistik – Definitionen

Die Statistik wird in einem reinen Go-Paket (`internal/stats`) aus den Ereignisverläufen berechnet; SQL lädt nur die Ereignisse. So ist sie ohne Datenbank testbar, und die Datenmenge (ein Nutzer) ist klein.

- **Basis:** Bewerbungen mit mindestens einem `Beworben`-Ereignis. `Vorgemerkt`-only zählt nicht.
- **Funnel-Stufen:** Beworben, Screening, Challenge, Interview, Kennenlerntag, Angebot, Angenommen. Eine Bewerbung hat eine Stufe **erreicht**, wenn sie ein Ereignis dieser oder einer späteren Stufe (nach Reihenfolge) hat; „Angenommen“ zählt nur `AngebotAngenommen`. Ein geplanter, noch nicht stattgefundener Termin zählt bereits als erreicht (Einladung erhalten). Quote = erreicht / Basis.
- **Tage bis zur ersten Antwort:** Differenz zwischen `Beworben.occurred_on` und dem ersten folgenden Ereignis, das nicht `Zurueckgezogen` oder `KeineRueckmeldung` ist. Als Antwortdatum gilt das frühere von `occurred_on` und Erfassungsdatum (die Einladung zählt, nicht der Termin). Ausgegeben als Median und Durchschnitt.
- **Antwortquote:** Anteil der Basis mit mindestens einer solchen Antwort.
- **Absagen je Phase:** `Absage`-Ereignisse gruppiert nach dem Typ des davor liegenden Ereignisses; `KeineRueckmeldung` wird dabei übersprungen (Ersatz, falls kein anderes Ereignis davor liegt).
- **Erfolg je Quelle:** je `source` die Anzahl Beworben, Interview erreicht, Angebot erreicht. Fehlende oder leere Quelle → „Unbekannt“.

## Backend (Go)

### Struktur

```
api/openapi.yaml
backend/
  cmd/server/main.go       Config, DB-Pool, Migrationen, HTTP-Server
  internal/domain/         EventType, Phase, CanApply, AllowedNext, CanUndo – keine Imports von DB/HTTP
  internal/stats/          Funnel und Kennzahlen aus Ereignisverläufen – rein, ohne DB
  internal/service/        Anwendungsfälle mit Transaktionen
  internal/store/          sqlc-generiert + queries/*.sql
  internal/httpapi/        oapi-codegen strict server, Handler, Fehler-Mapping, Middleware
  internal/config/         Umgebungsvariablen → Config
  internal/db/             Connect, Migrate
  internal/testdb/         Testcontainer-Helfer für Tests
  migrations/              goose-SQL, per go:embed eingebunden
```

Abhängigkeiten: `httpapi → service → domain`, zusätzlich `httpapi → domain` (Fehler-Mapping), `service → store`. `domain` hängt von nichts ab.

### Technik

Go 1.27, `net/http` mit eingebautem Routing, pgx v5, sqlc, goose (Migrationen laufen beim Start), `log/slog` mit Request-Logging-Middleware. Konfiguration über `DATABASE_URL` und `PORT`.

### Ablauf: Ereignis anlegen

1. Handler: Request-Validierung (generiert).
2. Service: Transaktion öffnen, Bewerbung mit `SELECT … FOR UPDATE` sperren (`404`, falls nicht vorhanden).
3. Bisherige Ereignisse laden, `domain.CanApply(history, next)` aufrufen → bei Fehler `ErrInvalidTransition`.
4. Bei `Interview` die Runde vergeben.
5. Ereignis einfügen, `current_status` und `updated_at` setzen, committen.

Rückgängig nutzt denselben Sperr-Mechanismus.

### Fehler-Mapping

Domain und Service liefern typisierte Fehler (`domain.ValidationError`, `domain.TransitionError`, `domain.RuleError`, `service.NotFoundError`, `service.ConflictError`). `httpapi/problem.go` ordnet sie per `errors.As` an genau einer Stelle zu und übersetzt sie in Problem-JSON. Alles andere wird geloggt und als generischer `500` beantwortet.

## Frontend (Angular)

### Technik

Aktuelle Angular-Version, Standalone-Komponenten, Signals, neuer Control Flow. Kein NgRx. Angular Material (Tabelle, Formulare, Dialoge, Datepicker, deutsches Datumsformat). API-Client aus `openapi.yaml` generiert. Statistik-Balken in reinem CSS, keine Chart-Bibliothek.

### Seiten

| Route | Inhalt |
|---|---|
| `/` | Dashboard: überfällige Fristen (rot), Fristen der nächsten 7 Tage, anstehende Termine, Kacheln (aktive Bewerbungen, offene Angebote, Antwortquote) |
| `/bewerbungen` | Tabelle (Firma, Stelle, Status, letztes Ereignis, nächste Frist); Filter Phase/Status; Textsuche |
| `/bewerbungen/neu` | Formular; Firma per Autocomplete oder neu anlegen; Startereignis Vorgemerkt/Beworben mit Datum |
| `/bewerbungen/:id` | Stammdaten (bearbeitbar), vertikale Timeline, Buttons für erlaubte Ereignisse, „Letztes Ereignis rückgängig“ |
| `/statistik` | Funnel, Antwortzeiten, Absagen je Phase, Erfolg je Quelle |
| `/firmen` | Liste mit Bewerbungsanzahl; umbenennen, löschen |

### Statuswechsel

Button öffnet einen Dialog mit Datum (Vorgabe heute), Notiz und – nur bei Typen mit „Frist möglich“ – Fristfeld. Nach dem Speichern werden Timeline und erlaubte Ereignisse neu geladen. Fehler aus Problem-JSON erscheinen als Snackbar.

### Struktur und Auslieferung

`src/app/features/` (dashboard, applications, stats, companies), `src/app/shared/` (Status-Badge, deutsche Labels), `src/app/api/` (generiert). Entwicklung mit `ng serve` und Proxy `/api → localhost:8080`. In Docker: Multi-Stage-Build zu nginx, das `/api` an den Backend-Container weiterleitet.

## Tests

| Ebene | Inhalt | Werkzeug |
|---|---|---|
| Domain + Stats | Jeder erlaubte und verbotene Übergang, Interview-Runden, `KeineRueckmeldung`-Wiedereröffnung, Datumsregeln, Rückgängig | Tabellengetriebene Go-Tests, keine DB |
| Service + Store | Transaktionen, Status-Neuberechnung, Fristen, Termine, Laden der Statistik-Daten | testcontainers-go mit echtem Postgres |
| HTTP | Durchstiche inkl. `400`/`404`/`409`/`422`, Form von Problem-JSON | `httptest` mit echtem Router und Container-DB |
| Frontend | Nur erlaubte Buttons, Fristfeld abhängig vom Typ, Fehler-Snackbar | Angular-Standard-Testrunner, gemockter API-Client |

Das Backend wird testgetrieben entwickelt, beginnend mit `internal/domain`.

## Betrieb und Werkzeuge

- **docker-compose.yml:** `db` (Postgres 17, Volume, Healthcheck), `backend` (Port 8080, wartet auf gesunde DB), `frontend` (nginx, http://localhost:4200).
- **Taskfile (go-task):** `task generate` (oapi-codegen, sqlc, TS-Client), `task test`, `task up`, `task seed` (ca. 25 fiktive Demo-Bewerbungen in allen Phasen).
- **GitHub Actions:** golangci-lint, Go-Tests, Angular-Build und -Tests, Prüfung auf aktuellen generierten Code (`task generate && git diff --exit-code`).

## Portfolio

- README mit Screenshots, Architekturdiagramm, Start in drei Befehlen, Link zur Swagger-UI.
- `docs/decisions/` mit kurzen ADRs: Ereignisse als Wahrheit, Spec-first mit OpenAPI, sqlc statt ORM.
