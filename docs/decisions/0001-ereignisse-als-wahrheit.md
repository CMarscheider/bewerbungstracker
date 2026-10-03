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
