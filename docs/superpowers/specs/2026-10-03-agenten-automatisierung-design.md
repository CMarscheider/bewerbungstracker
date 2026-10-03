# Agenten-Automatisierung: Jobsuche, Unterlagen, Gmail – Design

Stand: 2026-10-03 · freigegeben

## Ziel

Drei zeitgesteuerte Claude-Routinen nehmen Routinearbeit ab:

- **R1 Jobsuche** findet passende Stellen und legt sie als *Vorgemerkt* an.
- **R2 Unterlagen** erstellt für freigegebene Stellen ein zugeschnittenes Anschreiben, lässt Anschreiben und Lebenslauf als PDF rendern und legt einen Gmail-Entwurf an.
- **R3 Postfach** wertet zweimal täglich Antworten aus, aktualisiert Status und legt Antwortentwürfe auf Rückfragen an.

Der Mensch bleibt an zwei Stellen in der Schleife: Er gibt Stellen für Unterlagen frei und sendet jede Mail selbst.

## Architektur

```
Claude-Routinen (Cloud) ── HTTPS + Bearer-Token ──► Tailscale Funnel ──► Pi: /api/agent/*
        │                                                                 │
        └──────────── Gmail-Konnektor ──► Gmail          Go-Backend ── PostgreSQL ── Gotenberg
                                                                  ▲
                                         Heimnetz: Oberfläche pi.local:4200 (ohne Token)
```

- Die App läuft auf dem Raspberry Pi 4 per Docker Compose (`~/bewerbungsmanager`).
- Tailscale Funnel gibt nur den Pfadpräfix `/api/agent/` öffentlich frei. Oberfläche und bestehende `/api/v1/*` bleiben im Heimnetz.
- Die Routinen sind Claude-Code-Routinen in der Cloud und nutzen den Gmail-Konnektor des Users. Das Token der Agent-API ist ein Secret der Routine.

## Suchprofil (R1)

- Suchbegriffe: „Frontend-Developer“ (inkl. Angular/React/TypeScript-Varianten) und „KI-Entwickler“ (inkl. AI/ML/LLM Engineer), jeweils zusätzlich mit „Junior“ / „Einstieg“ / „Berufseinsteiger“.
- Schwerpunkt Junior- und Einstiegsstellen: Sie werden gezielt gesucht und beim Score bevorzugt. Stellen mit höherem Level (Mid, Senior, Lead) werden **nicht** ausgeschlossen, sondern nur nach Passung bewertet.
- Ort: remote in Deutschland **oder** hybrid/vor Ort im Umkreis von 15 km um PLZ 32339 (Espelkamp).
- Quellen: Bundesagentur-Jobsuche-API, Arbeitnow-API, Websuche (StepStone, Indeed, Karriereseiten). Kein LinkedIn-Scraping.
- Filter: Anzeige höchstens 14 Tage alt; keine Arbeitnehmerüberlassung. Kein Filter nach Erfahrungslevel.
- Höchstens 15 neue Stellen pro Lauf.
- Passungs-Score 0–100 mit kurzer Begründung, gemessen am Lebenslauf (`GET /api/agent/cv`). Die Begründung nennt das geforderte Level, damit Senior-Stellen in der Liste auf einen Blick erkennbar sind.
- Dublette = gleiche normalisierte `job_url` oder gleicher Firmenname + Stellentitel (case-insensitive). Dubletten werden übersprungen.

## Datenmodell

Neue Migration `00002_agent.sql`. Bestehende Tabellen bleiben, `applications` wird erweitert.

**`applications`** – neue Spalten:

| Spalte | Typ | Bedeutung |
|---|---|---|
| `contact_email` | text | Bewerbungsadresse aus der Anzeige, sonst NULL |
| `posting_text` | text | Anzeigentext (für R2) |
| `fit_score` | int, 0–100 | Passung laut Agent, NULL bei manuell angelegten |
| `fit_reason` | text | Begründung des Scores |
| `created_by_agent` | boolean, default false | für Filter „Neu vom Agenten“ |
| `documents_state` | text | `keine` · `angefordert` · `erstellt` · `entwurf_angelegt` · `portal` · `fehler` |
| `documents_error` | text | Fehlertext bei `fehler` |
| `gmail_draft_id` | text | ID des Bewerbungsentwurfs |
| `gmail_thread_id` | text | Thread der gesendeten Bewerbung |

Eindeutiger Index auf `job_url` (wo nicht NULL) als Dublettenschutz.

**`cv`** – genau eine Zeile (`id = 1`): `data jsonb` (Kontakt, Profil, Stationen, Skills, Projekte, Ausbildung, Sprachen), `updated_at`.

**`application_documents`** – je Bewerbung genau eine aktuelle Version: `application_id` (PK, FK), `version int`, `language` (`de`/`en`), `cover_letter text`, `profile_line text`, `highlights jsonb` (Liste Schwerpunkte), `pdf bytea`, `rendered_at`. Hochladen mit gleicher `version` ist idempotent.

**`status_suggestions`**: `id`, `application_id` (NULL = nicht zugeordnet), `suggested_type` (EventType), `occurred_on`, `due_on`, `reason text`, `mail_url text`, `state` (`offen`/`angenommen`/`verworfen`), `created_at`.

**`processed_mails`**: `gmail_message_id` (PK), `application_id` (NULL möglich), `outcome text`, `processed_at`.

**`cv_reviews`**: Optimierungsläufe für den Grund-Lebenslauf. `id`, `state` (`angefordert` · `fertig` · `abgeschlossen`), `based_on_updated_at` (Stand des CV bei Anforderung), `proposal jsonb` (vollständiger Vorschlag im `Cv`-Schema), `notes jsonb` (Liste von Hinweisen, z. B. Lücken, fehlende Kennzahlen), `requested_at`, `completed_at`. Höchstens ein Lauf ist gleichzeitig `angefordert` oder `fertig`.

## Agent-API

In `api/openapi.yaml` unter Tag `agent`, Präfix `/api/agent`. Jede Anfrage braucht `Authorization: Bearer <AGENT_TOKEN>` (aus `.env`). Ohne `AGENT_TOKEN` ist die gesamte Agent-API deaktiviert (404). Falsches Token → 401 und Log-Eintrag.

| Methode und Pfad | Zweck |
|---|---|
| `GET /cv` | Lebenslauf-Daten |
| `GET /applications` | Liste (Filter `documents_state`, `status`), inkl. Firma, Kontakt, Gmail-IDs |
| `POST /applications` | Stelle anlegen (Status *Vorgemerkt*, `created_by_agent = true`); bei Dublette 409 mit vorhandener ID |
| `PUT /applications/{id}/documents` | Inhalt hochladen → PDF wird synchron gerendert, `documents_state = erstellt` |
| `GET /applications/{id}/documents/pdf` | PDF abholen |
| `PATCH /applications/{id}/gmail` | `gmail_draft_id`, `gmail_thread_id`, `documents_state` (`entwurf_angelegt`/`portal`) setzen |
| `POST /applications/{id}/events` | Status-Ereignis anlegen; nutzt dieselbe Übergangsprüfung wie die Oberfläche (unerlaubt → 422) |
| `POST /suggestions` | Vorschlag anlegen |
| `GET /processed-mails/{id}` · `POST /processed-mails` | Doppelverarbeitung verhindern |
| `GET /cv-reviews?state=angefordert` | offene Optimierungsanfrage |
| `PUT /cv-reviews/{id}` | Vorschlag und Hinweise abliefern → `fertig` |

Notizen an Ereignissen des Agenten beginnen mit `Agent:` und nennen das Mail-Datum.

## PDF-Erzeugung

- Neuer Compose-Dienst `gotenberg` (offizielles arm64-Image), nur intern erreichbar.
- Go-Paket `internal/documents` erzeugt aus CV-Daten + Bewerbungsinhalt HTML über `html/template` (Vorlagen eingebettet per `embed`) und schickt es an Gotenbergs Chromium-Endpunkt.
- Ein PDF: Seite 1 Anschreiben (max. eine Seite), danach Lebenslauf. Profil-Satz und Schwerpunkte ersetzen bzw. ordnen die entsprechenden CV-Abschnitte; alle anderen Inhalte kommen unverändert aus dem CV.
- Dateiname: `Bewerbung_<Nachname>_<Firma>.pdf`.

## Oberfläche

- **Lebenslauf** (neue Seite): Formular für die CV-Daten, Vorschau-PDF. Knopf **„Mit Claude optimieren“** legt einen `cv_review` an. Ist er `fertig`, zeigt die Seite Vorher/Nachher je Abschnitt (Profil, jede Station, Kenntnisse, Projekte) mit „Übernehmen“ je Abschnitt und „Alles übernehmen“, dazu die Hinweise. „Abschließen“ setzt den Lauf auf `abgeschlossen`. Hat sich der CV seit der Anforderung geändert, weist die Seite darauf hin.
- **Bewerbungen**: Spalte/Sortierung Score, Filter „Neu vom Agenten“.
- **Detail**: Score mit Begründung, Anzeigentext einklappbar, Knopf **„Unterlagen erstellen“** (setzt `angefordert`), Status der Unterlagen, editierbares Anschreiben mit „Neu rendern“, PDF-Download, Link zum Gmail-Entwurf bzw. Hinweis „Über Portal bewerben“, Fehlertext bei `fehler`.
- **Dashboard**: Box „Vorschläge des Agenten“ mit Übernehmen (legt Ereignis an) und Verwerfen.
- Für die Oberfläche gibt es passende Endpunkte unter `/api/v1` (CV lesen/speichern, Unterlagen anfordern/bearbeiten/neu rendern, Vorschläge annehmen/verwerfen) ohne Token, wie die bestehende API.

## Routinen

Alle Routinen: Vorab `GET /api/agent/cv` als Erreichbarkeitsprüfung. Ist der Pi nicht erreichbar, bricht der Lauf ab, ohne Gmail anzufassen.

**R1 Jobsuche** – täglich 07:00. Sucht laut Suchprofil, bewertet, legt neue Stellen an.

**R2 Unterlagen** – stündlich 08:00–20:00. Für jede Stelle mit `angefordert`:
1. Anschreiben (Deutsch; Englisch, wenn die Anzeige englisch ist), Profil-Satz und 3–5 Schwerpunkte schreiben. Nur Fakten aus dem CV, nichts erfinden. Einzige Ausnahme, vom User freigegeben: Erwähnt die Anzeige KI, darf „nutzt KI für Automatisierungen“ in Profil-Satz und Anschreiben ergänzt werden; sonst bleibt KI unerwähnt.
2. `PUT …/documents`, dann `GET …/documents/pdf`.
3. Mit `contact_email`: Gmail-Entwurf (Betreff „Bewerbung als <Titel>“, 3–5 Sätze Mailtext, PDF als Anhang), dann `PATCH …/gmail` mit `entwurf_angelegt` und Draft-ID. Ohne Adresse: `PATCH …/gmail` mit `portal`.

Zusätzlich bearbeitet R2 bei jedem Lauf eine offene Lebenslauf-Optimierung (`cv_reviews` mit `angefordert`): Formulierungen schärfen, Stichpunkte mit Wirkung und Kennzahlen (nur wenn im CV belegt), Keywords für Frontend- und KI-Stellen, Reihenfolge nach Relevanz für Junior-Stellen. Neue Fakten werden nie erfunden; fehlende Angaben landen als Hinweis in `notes`.

**R3 Postfach** – 08:00 und 17:00. Betrachtet Mails der letzten 3 Tage, die nicht in `processed_mails` stehen:
1. Gesendete Bewerbungsmails (Draft-ID/Betreff) → Ereignis *Beworben* mit Versanddatum, `gmail_thread_id` speichern.
2. Eingehende Mails zuordnen: über `gmail_thread_id`, sonst Absender-Domain ↔ Firma/Website. Newsletter und Jobportal-Alerts ignorieren.
3. Eindeutig → Ereignis direkt: Eingangsbestätigung (keine Statusänderung, nur verarbeitet), Absage, Einladung zu Screening/Interview/Kennenlerntag mit Datum, Challenge mit Frist, Angebot.
4. Unklar oder nicht zuordenbar → `POST /suggestions`.
5. Enthält die Mail eine Rückfrage → Antwortentwurf im Thread (`replyToMessageId`), nur aus CV-Fakten; fehlt Wissen, steht im Entwurf ein markierter Platzhalter `[[bitte ergänzen: …]]`.
6. Jede betrachtete Mail → `POST /processed-mails`.

Routinen senden nie selbst Mails.

## Fehlerbehandlung

- Schreibende Agent-Endpunkte sind wiederholbar (Dublettenschutz über URL, Mail-ID, Dokument-Version).
- PDF-Rendering scheitert → `documents_state = fehler` mit `documents_error`; Oberfläche zeigt den Fehler, „Unterlagen erstellen“ setzt wieder `angefordert`.
- Gotenberg nicht erreichbar → 503, R2 lässt die Stelle auf `angefordert`.

## Tests

- Go: Agent-API mit testcontainers (Token fehlt/falsch, Dublette, unerlaubter Übergang, idempotente Uploads, processed-mails), `internal/documents` gegen einen Gotenberg-Testcontainer (PDF entsteht, Seitenzahl ≥ 2).
- Angular: Lebenslauf-Formular, Unterlagen-Bereich im Detail, Vorschlags-Box.
- Routinen: je ein Probelauf gegen den Pi; R2 und R3 zuerst mit einer Test-Bewerbung an die eigene Adresse.

## Reihenfolge

1. Lebenslauf (Tabelle, API, Seite); Erstbefüllung mit einer gemeinsam mit Claude optimierten Fassung
2. Datenmodell und Agent-API mit Token, inkl. `cv_reviews` und Optimierungs-Ansicht
3. PDF-Renderer (Gotenberg, Vorlagen)
4. Oberfläche: Score, Freigabe, Unterlagen, Vorschläge
5. Tailscale Funnel auf dem Pi
6. Routinen R1 → R2 → R3, jeweils mit Probelauf

## Offene Punkte aus dem Lebenslauf-Review

Für die Folgepläne festgehalten (Review des Lebenslauf-Branches, 2026-10-03):

- **Agent-API:** Der Server prüft nur `person.name` auf reine Leerzeichen. Da Agenten die Oberfläche umgehen, bekommen Pflicht-Strings im `Cv`-Schema zusätzlich `pattern: '\S'`.
- **PDF:** `CvLink.url` und `CvProject.url` sind beliebige Strings. Vor dem Rendern als Link auf `^https?://` einschränken (Schema), zusätzlich zur Bereinigung durch `html/template`.
- **Optimierungs-Ansicht:** Stationen haben keine stabilen IDs; R2 darf sie umsortieren. Der Vorher/Nachher-Vergleich übernimmt daher ganze Abschnitte, nicht einzelne Stationen per Index – oder `CvExperience` bekommt ein optionales `id`.
- **Oberfläche:** Listen-Grenzen (`maxItems`, Länge je Stichpunkt) prüft nur der Server, mit technischer Meldung; Fehlermeldungen des Validators eindeutschen oder im Formular prüfen.

## Nicht enthalten

Automatisches Senden, Ausfüllen von Bewerbungsportalen, mehrere Lebenslauf-Varianten, LinkedIn-Scraping, Login für die Oberfläche (bleibt im Heimnetz).
