# Bewerbungs-Tracker

[![CI](https://github.com/CMarscheider/bewerbungstracker/actions/workflows/ci.yml/badge.svg)](https://github.com/CMarscheider/bewerbungstracker/actions/workflows/ci.yml)

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
