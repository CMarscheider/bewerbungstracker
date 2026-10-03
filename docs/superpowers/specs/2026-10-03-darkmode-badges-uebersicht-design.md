# Dark Mode, Status-Farben und geordnete Übersicht – Design

Stand: 2026-10-03 · freigegeben

## Status-Farben

Farbe je Prozessschritt, sechs Farbtöne plus eine gefüllte Variante:

| Farbton | Status |
|---|---|
| neutral (grau) | Vorgemerkt, AngebotAbgelehnt, Zurueckgezogen, KeineRueckmeldung |
| blau | Beworben |
| violett | ScreeningGespraech, Interview, Kennenlerntag |
| orange | ChallengeErhalten, ChallengeAbgegeben |
| grün | AngebotErhalten |
| grün, gefüllt | AngebotAngenommen |
| rot | Absage |

- Zuordnung `STATUS_TONE: Record<EventType, StatusTone>` in `shared/labels.ts` (Compiler erzwingt Vollständigkeit).
- Farben als CSS-Variablen `--tone-<ton>-bg` / `--tone-<ton>-fg` in `styles.scss`, per `light-dark()` für beide Modi, Kontrast ≥ 4,5:1.
- `StatusBadge` braucht die Phase nicht mehr; das Input entfällt.

## Dark Mode

- Standard: Systemeinstellung (`color-scheme: light dark`). Material 3 und eigene Farben folgen über `light-dark()`.
- `core/theme.ts`: `ThemeService` mit Modus `system | light | dark`, setzt `data-theme` auf `<html>`, speichert in `localStorage` (Fehler beim Zugriff → System).
- Kopfzeile: Icon-Button mit Menü (System, Hell, Dunkel), Icons als Inline-SVG.
- Inline-Skript in `index.html` setzt `data-theme` vor dem Start von Angular (kein Aufblitzen).

## Übersicht

- Kennzahlen-Kacheln bleiben.
- Darunter zwei Karten „Fristen“ und „Termine“ nebeneinander (unter 900 px untereinander), Anzahl im Kopf.
- Zeile: feste Datumsspalte (Datum + Wochentag), Firma fett mit Stelle darunter, Art mit farbigem Punkt in der Status-Farbe.
- Überfällige Fristen oben in der Fristen-Karte, rot mit „überfällig“.
- Lade-, Fehler- und Leer-Zustand jeweils in der Karte.

## Tests

- StatusBadge prüft Farbton-Klasse.
- ThemeService: Speichern, `data-theme`, gesperrter Speicher.
- Übersicht: Aufbau mit überfälligen Fristen zuerst.
- README-Screenshots neu, zusätzlich Übersicht im Dark Mode.
