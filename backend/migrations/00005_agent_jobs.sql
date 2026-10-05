-- +goose Up
ALTER TABLE applications
    ADD COLUMN contact_email    text,
    ADD COLUMN posting_text     text,
    ADD COLUMN fit_score        int CHECK (fit_score BETWEEN 0 AND 100),
    ADD COLUMN fit_reason       text,
    ADD COLUMN created_by_agent boolean NOT NULL DEFAULT false;

-- Dublettenschutz: dieselbe Anzeige (ohne Schrägstrich am Ende, ohne Groß-/Kleinschreibung) nur einmal.
CREATE UNIQUE INDEX applications_job_url_unique ON applications (lower(rtrim(job_url, '/'))) WHERE job_url IS NOT NULL;
-- Firmen ohne Groß-/Kleinschreibung eindeutig, damit der Agent „ACME GmbH“ und „Acme GmbH“ nicht doppelt anlegt.
CREATE UNIQUE INDEX companies_name_ci_unique ON companies (lower(name));

-- +goose Down
DROP INDEX companies_name_ci_unique;
DROP INDEX applications_job_url_unique;
ALTER TABLE applications
    DROP COLUMN created_by_agent,
    DROP COLUMN fit_reason,
    DROP COLUMN fit_score,
    DROP COLUMN posting_text,
    DROP COLUMN contact_email;
