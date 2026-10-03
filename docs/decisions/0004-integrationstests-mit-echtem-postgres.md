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
