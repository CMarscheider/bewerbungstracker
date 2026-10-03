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
