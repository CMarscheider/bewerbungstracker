-- +goose Up
ALTER TABLE applications ADD COLUMN gmail_thread_id text;
CREATE UNIQUE INDEX applications_gmail_thread ON applications (gmail_thread_id) WHERE gmail_thread_id IS NOT NULL;

-- Vorschläge des Agenten aus der Postfach-Auswertung, die der Nutzer übernimmt oder verwirft.
CREATE TABLE status_suggestions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid REFERENCES applications (id) ON DELETE CASCADE, -- NULL = nicht zugeordnet
    suggested_type text NOT NULL CHECK (suggested_type IN (
        'Vorgemerkt', 'Beworben', 'ScreeningGespraech', 'ChallengeErhalten', 'ChallengeAbgegeben',
        'Interview', 'Kennenlerntag', 'AngebotErhalten', 'AngebotAngenommen', 'AngebotAbgelehnt',
        'Absage', 'Zurueckgezogen', 'KeineRueckmeldung')),
    occurred_on    date NOT NULL,
    due_on         date,
    reason         text NOT NULL,
    mail_subject   text,
    mail_from      text,
    mail_url       text,
    state          text NOT NULL DEFAULT 'offen' CHECK (state IN ('offen', 'angenommen', 'verworfen')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    decided_at     timestamptz
);
CREATE INDEX status_suggestions_open ON status_suggestions (created_at) WHERE state = 'offen';

-- Bereits ausgewertete Mails; verhindert Doppelverarbeitung.
CREATE TABLE processed_mails (
    gmail_message_id text PRIMARY KEY,
    application_id   uuid REFERENCES applications (id) ON DELETE SET NULL,
    outcome          text NOT NULL,
    processed_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_mails;
DROP TABLE status_suggestions;
DROP INDEX applications_gmail_thread;
ALTER TABLE applications DROP COLUMN gmail_thread_id;
