-- +goose Up
CREATE TABLE companies (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL UNIQUE,
    website    text,
    notes      text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE applications (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id     uuid NOT NULL REFERENCES companies (id) ON DELETE RESTRICT,
    position_title text NOT NULL,
    job_url        text,
    location       text,
    source         text,
    notes          text,
    current_status text NOT NULL CHECK (current_status IN (
        'Vorgemerkt', 'Beworben', 'ScreeningGespraech', 'ChallengeErhalten', 'ChallengeAbgegeben',
        'Interview', 'Kennenlerntag', 'AngebotErhalten', 'AngebotAngenommen', 'AngebotAbgelehnt',
        'Absage', 'Zurueckgezogen', 'KeineRueckmeldung')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX applications_by_company ON applications (company_id);

CREATE TABLE application_events (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  uuid NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    type            text NOT NULL CHECK (type IN (
        'Vorgemerkt', 'Beworben', 'ScreeningGespraech', 'ChallengeErhalten', 'ChallengeAbgegeben',
        'Interview', 'Kennenlerntag', 'AngebotErhalten', 'AngebotAngenommen', 'AngebotAbgelehnt',
        'Absage', 'Zurueckgezogen', 'KeineRueckmeldung')),
    occurred_on     date NOT NULL,
    due_on          date,
    interview_round int CHECK (interview_round IS NULL OR interview_round >= 1),
    note            text,
    -- clock_timestamp statt now(): eindeutige Reihenfolge auch innerhalb einer Transaktion
    created_at      timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX application_events_by_application ON application_events (application_id, created_at);

-- Letztes Ereignis je Bewerbung: Grundlage für Liste, Fristen und Termine.
CREATE VIEW latest_events AS
SELECT DISTINCT ON (application_id)
       application_id, id AS event_id, type, occurred_on, due_on, interview_round
FROM application_events
ORDER BY application_id, created_at DESC;

-- +goose Down
DROP VIEW latest_events;
DROP TABLE application_events;
DROP TABLE applications;
DROP TABLE companies;
