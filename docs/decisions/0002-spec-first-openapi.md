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
