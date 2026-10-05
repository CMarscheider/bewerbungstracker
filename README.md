# Bewerbungs-Tracker

[![CI](https://github.com/CMarscheider/bewerbungstracker/actions/workflows/ci.yml/badge.svg)](https://github.com/CMarscheider/bewerbungstracker/actions/workflows/ci.yml)

Verwaltet die eigenen Bewerbungen – Firma, Stelle, Status und Fristen. Jeder Statuswechsel
(beworben, Challenge erhalten, Interview, Angebot …) wird als Ereignis gespeichert; daraus
entstehen Dashboard, Verlauf und Statistik.

**Go · PostgreSQL · REST/OpenAPI · Angular 22 · Docker**

![Dashboard](docs/screenshots/dashboard-desktop.png)

## Funktionen

- **Dashboard:** überfällige Fristen, Fristen der nächsten 7 Tage, anstehende Termine, Kennzahlen
- **Bewerbungen:** Liste mit Filtern und Suche, Anlegen mit Firmen-Autocomplete; vom Agenten gefundene
  Stellen mit Passungs-Score, Begründung und Anzeigentext, filter- und nach Passung sortierbar
- **Verlauf:** Timeline je Bewerbung; nur fachlich erlaubte nächste Schritte werden angeboten,
  Fristen nur dort, wo sie Sinn ergeben (Bewerbungsschluss, Challenge-Abgabe, Antwort auf Angebot)
- **Statistik:** Funnel, Tage bis zur ersten Antwort, Absagen je Phase, Erfolg je Quelle
- **Lebenslauf:** strukturiert gepflegt (Kontakt, Profil, Stationen, Ausbildung, Kenntnisse, Projekte,
  Sprachen) als Grundlage für zugeschnittene Bewerbungsunterlagen
- **Bewerbungsfoto und PDF:** Foto hochladen (wird auf 3:4 zugeschnitten) und den Lebenslauf als PDF
  ansehen – modernes zweispaltiges Layout, Schrift Carlito (metrisch kompatibel zu Calibri)
- **Darstellung:** Hell, Dunkel oder nach Systemeinstellung; Status-Farben je Prozessschritt
- **API-Doku:** Swagger UI unter `/api/docs`

| Bewerbung im Detail | Statistik |
|---|---|
| ![Detail](docs/screenshots/detail-desktop.png) | ![Statistik](docs/screenshots/statistik-desktop.png) |

| Dark Mode | Bewerbungen |
|---|---|
| ![Übersicht im Dark Mode](docs/screenshots/dashboard-desktop-dark.png) | ![Bewerbungsliste](docs/screenshots/bewerbungen-desktop.png) |

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

Der erste `docker compose up` lädt zusätzlich das Gotenberg-Image (ca. 1,5 GB). Auf Raspberry Pi OS wird
`mem_limit` nur mit `cgroup_enable=memory` in `/boot/firmware/cmdline.txt` beachtet (sonst ignoriert).

Port ändern: `APP_PORT=8081 docker compose up -d`. Zurücksetzen: `docker compose down -v`.

## Architektur

```mermaid
flowchart LR
  B[Browser] -->|HTTP| N[nginx<br/>Angular-App]
  N -->|/api| G[Go-Backend<br/>net/http · oapi-codegen]
  G --> D[(PostgreSQL 17)]
  G -->|HTML → PDF| P[Gotenberg<br/>Chromium]
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

Optional: `GOTENBERG_URL` (z. B. `http://localhost:3000` für einen lokalen `gotenberg/gotenberg:8.37.0`-Container) aktiviert den PDF-Export des Lebenslaufs; ohne sie antwortet der Export mit 503. Im Compose-Stack ist sie bereits gesetzt.

### Agent-API

Für Claude-Agenten gibt es unter `/api/agent/` eine eigene API, die eine angeforderte
Lebenslauf-Optimierung abholt, den Vorschlag zurückliefert, gefundene Stellen anlegt und das Postfach auswertet. Sie ist nur aktiv, wenn `AGENT_TOKEN`
gesetzt ist (in `.env`, mind. 32 Zeichen, z. B. `openssl rand -hex 32`); ohne Token antwortet sie mit 404.
Jeder Aufruf braucht `Authorization: Bearer <token>`, sonst 401.

Für die Postfach-Auswertung (Routine R3), die fremde und damit möglicherweise manipulierte Mails liest,
gibt es optional ein eingeschränktes zweites Token `AGENT_TOKEN_MAIL` (mind. 32 Zeichen, verschieden
von `AGENT_TOKEN`, nur zusammen mit ihm). Es darf nur `GET /api/agent/cv`,
`GET /api/agent/applications/open`, `POST /api/agent/applications/{id}/events`,
`PUT /api/agent/applications/{id}/gmail-thread`, `POST /api/agent/suggestions` und
`GET`/`POST /api/agent/processed-mails…` aufrufen; alles andere → 403. Beide Tokens teilen sich die Drossel.

- `GET /api/agent/cv` – aktueller Lebenslauf
- `GET /api/agent/cv-reviews?state=angefordert` – Optimierungen in einem Zustand (`state` ist Pflicht)
- `PUT /api/agent/cv-reviews/{id}` – Vorschlag abliefern: `proposal` ist ein vollständiger Lebenslauf
  (`updated_at` wird ignoriert), dazu `notes` (Hinweise). 409, wenn der Lauf nicht mehr `angefordert` ist
  (in der Oberfläche abgeschlossen oder schon abgeliefert); nach einer verlorenen Antwort vor einem
  erneuten Versuch die Liste neu lesen.
- `POST /api/agent/applications` – gefundene Stelle als *Vorgemerkt* anlegen (Firma wird bei Bedarf
  angelegt; `fit_score` 0–100 und `fit_reason` sind Pflicht). Dublette (gleiche Anzeigen-URL oder
  gleiche Firma + Titel) → 409 mit `existing_id`.
- `GET /api/agent/applications?documents_state=angefordert` – Stellen, für die in der Oberfläche
  Unterlagen angefordert wurden (`documents_state` ist Pflicht), älteste zuerst; mit Anzeigentext und
  `documents_version` (0 = noch keine Unterlagen).
- `PUT /api/agent/applications/{id}/documents` – Unterlagen abliefern: `version` (= `documents_version` + 1),
  `language` (`de`/`en`), `cover_letter`, optional `profile_line` und `highlights` (höchstens 8, nur
  vorhandene Kenntnisse), `mail_subject`, `mail_body`. Der Server rendert Anschreiben + Lebenslauf als ein
  PDF und legt mit Bewerbungsadresse einen Gmail-Entwurf an. Idempotent: dieselbe Version mit gleichem
  Inhalt erneut → 200 ohne Wirkung. Antworten:
  - 400 – ungültige Eingabe (siehe `detail`/`field`): korrigieren, nicht unverändert wiederholen.
  - 404 – unbekannte ID.
  - 409 – nicht (mehr) angefordert, kein Lebenslauf gespeichert oder Version veraltet: Liste neu lesen.
  - 503 – PDF-Dienst nicht erreichbar: später erneut liefern; nichts wurde gespeichert, der Zustand
    bleibt `angefordert`.
  - 500 – Renderfehler: der Zustand wird `fehler`; nicht erneut liefern, bis die Unterlagen in der
    Oberfläche wieder angefordert werden.
- `GET /api/agent/applications/open` – laufende Bewerbungen (inkl. *Keine Rückmeldung*, für späte
  Antworten) mit Kontaktadresse, `gmail_thread_id`, Betreff der Bewerbungsmail und `allowed_events`.
  Ereignisse nur aus `allowed_events` wählen, nicht raten.
- `POST /api/agent/applications/{id}/events` – Ereignis wie in der Oberfläche erfassen (`type`,
  `occurred_on`, optional `due_on`, `note` ≤ 1000 Zeichen); die Notiz bekommt das Präfix `Agent: `.
  Direkt erlaubt sind nur *Beworben*, *ScreeningGespraech*, *ChallengeErhalten*, *Interview*,
  *Kennenlerntag*, *AngebotErhalten* und *Absage*; andere Typen → 400 (Feld `type`), dafür einen
  Vorschlag ablegen. Unerlaubter Übergang → 422.
- `PUT /api/agent/applications/{id}/gmail-thread` – Thread der gesendeten Bewerbung merken
  (`gmail_thread_id`, nur `A–Z a–z 0–9 _ -`) → 204; dieselbe ID erneut ist ein No-op, 409, wenn der
  Thread schon zu einer anderen Bewerbung gehört.
- `POST /api/agent/suggestions` – unklare Antwort als Vorschlag für das Dashboard ablegen
  (`suggested_type`, `occurred_on`, `reason`; optional `application_id`, `due_on`, `mail_subject`,
  `mail_from`, `mail_url` nur `https://mail.google.com/…`, `gmail_message_id`). Ohne `application_id`
  ordnet der User zu. Mit `gmail_message_id` idempotent: 201 beim ersten Mal; gibt es zur Mail schon
  einen Vorschlag (auch einen schon übernommenen oder verworfenen), 200 mit diesem unveränderten Vorschlag.
- `GET /api/agent/processed-mails/{messageId}` – 200, wenn die Mail schon ausgewertet ist, sonst 404.
- `POST /api/agent/processed-mails` – Mail als ausgewertet merken (`gmail_message_id`, `outcome`,
  optional `application_id`). Idempotent: 201 beim ersten Mal, danach 200 mit dem unveränderten
  vorhandenen Eintrag. Erst nach erfolgreicher Verbuchung aufrufen. Höchstens 200 neu gemerkte Mails
  je 24 Stunden, darüber 409 (Schutz gegen massenhaftes Verstecken).

Texte aus Mails (`reason`, `mail_subject`, `mail_from`, `mail_url`, `note`, `outcome`) dürfen keine
Steuer- oder unsichtbaren Formatzeichen enthalten (auch keine Tabs und Zeilenumbrüche) → 400.
In der Oberfläche lassen sich eine falsche Thread-Zuordnung lösen
(`DELETE /api/v1/applications/{id}/gmail-thread`) und fälschlich gemerkte Mails zur erneuten
Auswertung freigeben (`GET /api/v1/processed-mails?limit=50`, `DELETE /api/v1/processed-mails/{messageId}`).

Die Agent-API ist gedrosselt: mit gültigem Token 2 Anfragen/s (Burst 20), ohne gültiges Token 1/s
(Burst 10), darüber 429 mit `Retry-After`. Abgewiesene Anfragen landen höchstens einmal pro Minute
zusammengefasst im Log; die Container-Logs rotieren (3 × 10 MB).

> **Achtung:** `/api/v1` hat keine Anmeldung. Bei einer Freigabe nach außen (z. B. Tailscale Funnel)
> nur den Pfad `/api/agent/` weiterleiten (Ziel `http://127.0.0.1:4200/api/agent/`), nie `/`, `/api/v1`,
> `/api/docs` oder `/api/openapi.json`.

### Gmail-Entwürfe

Mit `GMAIL_ADDRESS` und `GMAIL_APP_PASSWORD` in `.env` (App-Passwort unter
myaccount.google.com/apppasswords, erfordert die Bestätigung in zwei Schritten) legt der Pi für fertige
Unterlagen per IMAP einen Gmail-Entwurf mit dem PDF als Anhang an – an die Bewerbungsadresse der Stelle.
Gesendet wird nie; das Abschicken bleibt Handarbeit in Gmail. Ohne die beiden Variablen werden nur die
Unterlagen erstellt; ohne Bewerbungsadresse (Bewerbung über ein Portal) gibt es keinen Entwurf.

## Projektstruktur

```
api/openapi.yaml        API-Vertrag
backend/                Go: cmd/server, cmd/seed, internal/{domain,stats,service,store,httpapi,documents,db}
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
- **PDF-Export:** Test des Gotenberg-Clients gegen einen echten Gotenberg-Container (testcontainers)
- **Demo-Daten:** jedes Szenario wird gegen den echten Zustandsautomaten geprüft

Alle Firmen und Personen in den Demo-Daten sind frei erfunden.
