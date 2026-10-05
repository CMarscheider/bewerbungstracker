# Härtung vor Tailscale Funnel – Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Die Agent-API (`/api/agent/*`) kann gefahrlos per Tailscale Funnel ins Internet: Anfragen werden gedrosselt, fehlgeschlagene Anmeldungen fluten das Log nicht, das Log zeigt die weitergereichte Client-Adresse, und Links im Lebenslauf sind nur `http(s)://`.

**Architecture:** Eine neue Middleware `limitAgent` vor `requireAgentToken` begrenzt alle Anfragen unter `/api/agent/` mit einem globalen Token-Bucket (`golang.org/x/time/rate`); Funnel und nginx verdecken die echte IP (RemoteAddr ist immer nginx), daher global statt je IP – es gibt nur einen legitimen Client (die Claude-Routine). Warnungen bei ungültigem Token werden höchstens einmal pro Minute geschrieben, mit Zahl der unterdrückten. `X-Forwarded-For` wird als `xff_untrusted` mitgeloggt. Im OpenAPI-Schema und im Formular müssen `CvLink.url` und `CvProject.url` mit `http://` oder `https://` beginnen.

**Spec:** `docs/superpowers/specs/2026-10-03-agenten-automatisierung-design.md`, „Offene Punkte“ (PDF-Links, Vor dem Freischalten per Tailscale Funnel).

**Umgebung (Windows):** vor Go-/task-Befehlen `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`; Docker Desktop läuft (nicht selbst starten); Commits ohne Attributionszeilen; Branch `feature/funnel-haertung` (existiert).

---

### Task 1: Drosselung und Logging der Agent-API

**Files:**
- Create: `backend/internal/httpapi/agent_limit.go`, `backend/internal/httpapi/agent_limit_test.go`
- Modify: `backend/internal/httpapi/agent_auth.go` (+ Test), `backend/internal/httpapi/logging.go`, `backend/internal/httpapi/router.go`, `backend/go.mod`/`go.sum` (`golang.org/x/time`)

- [ ] **Step 1: Failing Tests**
  - `limitAgent`: Nach Ausschöpfen des Bursts antwortet `/api/agent/...` mit 429, Problem-Typ `problemBase + "too-many-requests"`, Titel „Zu viele Anfragen“, Header `Retry-After` (Sekunden, ganzzahlig ≥ 1). Pfade außerhalb von `/api/agent/` werden nie gedrosselt. Die Drossel greift vor der Token-Prüfung (auch Anfragen ohne Token zählen).
  - Limiter testbar machen: Konstruktor nimmt `rate.Limit` und Burst entgegen (`newAgentLimiter(r rate.Limit, burst int)`); Produktionswerte als Konstanten: `agentRate = rate.Every(500 * time.Millisecond)` (2/s), `agentBurst = 20`.
  - Gedrosseltes Warn-Log: Bei 50 Anfragen mit falschem Token in schneller Folge entsteht genau eine Warnung; nach Ablauf des Intervalls (Uhr injizierbar, z. B. Feld `now func() time.Time`) schreibt die nächste Warnung `suppressed=<Anzahl>` mit. Intervall-Konstante `authWarnInterval = time.Minute`.
  - Logging: `logRequests` schreibt bei vorhandenem `X-Forwarded-For` das Feld `xff_untrusted` mit dem Header-Wert (gekürzt auf 200 Zeichen); ohne Header fehlt das Feld. Die Token-Warnung enthält dasselbe Feld. Test mit `slog.NewJSONHandler` auf einen `bytes.Buffer`.

- [ ] **Step 2: Implementierung**
  - `agent_limit.go`: `limitAgent(l *rate.Limiter, next http.Handler) http.Handler`; bei `!l.Allow()` 429 wie oben; `Retry-After` aus `l.Reserve()`-Delay nicht nötig – fest `1` reicht, da Rate 2/s.
  - `router.go`: Kette `logRequests(recoverPanics(limitBody(limitAgent(limiter, requireAgentToken(...)))))`; Limiter einmal je `NewRouter` erzeugen, Werte über `routerConfig` überschreibbar (`WithAgentRateLimit(r rate.Limit, burst int)` als `RouterOption`, für Tests).
  - `agent_auth.go`: Warnung über einen kleinen Throttler (Mutex, letzter Zeitpunkt, Zähler) statt direkt `logger.Warn`.
  - Kommentare im Stil der Umgebung (deutsch, knapp).

- [ ] **Step 3: Prüfen und committen**
  `go test ./...` und `golangci-lint run ./...` in `backend/`; Commit `feat(agent): Drosselung und gedrosseltes Logging`.

---

### Task 2: Nur http(s)-Links im Lebenslauf

**Files:**
- Modify: `api/openapi.yaml`, generierter Code (`task generate:api`, `task generate:client`), `frontend/src/app/features/cv/cv-form.ts` (+ Spec), `frontend/src/app/features/cv/cv.html` (Fehlertext), Backend-Test in `backend/internal/httpapi/cv_test.go`

- [ ] **Step 1: Failing Tests**
  - Backend: `PUT /api/v1/cv` mit `person.links[0].url = "javascript:alert(1)"` → 400; mit `projects[0].url = "ftp://x"` → 400; `https://github.com/x` und `http://localhost:4200` → 200.
  - Frontend: Link-URL und Projekt-URL sind ungültig bei `javascript:alert(1)` und `github.com/x`, gültig bei `https://github.com/x`; leere Projekt-URL bleibt gültig (optional).

- [ ] **Step 2: Implementierung**
  - `CvLink.url`: `pattern: '^https?://\S'` (ersetzt `'\S'`), `CvProject.url`: `pattern: '^https?://\S'` (bleibt optional).
  - Formular: gemeinsamer `HTTP_URL = /^https?:\/\/\S/` als `Validators.pattern`; Link-URL Pflicht + Muster, Projekt-URL nur Muster. Fehlertext im Formular: „Bitte mit http:// oder https:// beginnen“ (im Stil der vorhandenen `mat-error`).
  - Bestehende Daten auf dem Pi beginnen alle mit `https://` – keine Migration nötig.

- [ ] **Step 3: Prüfen und committen**
  Backend-Tests + Lint, `npx ng test --watch=false`, `npx ng build`; Commit `feat(cv): Links nur mit http(s)`.

---

### Task 3: Dokumentation (Controller)

- README „Agent-API“: Drosselung (2/s, Burst 20, 429) erwähnen; Spec „Offene Punkte“: erledigte Punkte als erledigt markieren.
- Danach Merge, Deploy auf dem Pi, Tailscale Funnel einrichten (eigener Schritt mit dem User).
