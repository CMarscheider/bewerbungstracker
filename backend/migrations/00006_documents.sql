-- +goose Up
ALTER TABLE applications
    ADD COLUMN documents_state text NOT NULL DEFAULT 'keine' CHECK (documents_state IN
        ('keine', 'angefordert', 'erstellt', 'entwurf_angelegt', 'portal', 'fehler')),
    ADD COLUMN documents_error text,
    ADD COLUMN gmail_draft_at  timestamptz;

CREATE INDEX applications_documents_state ON applications (documents_state) WHERE documents_state = 'angefordert';

-- Je Bewerbung genau eine aktuelle Fassung der Unterlagen.
CREATE TABLE application_documents (
    application_id uuid PRIMARY KEY REFERENCES applications (id) ON DELETE CASCADE,
    version        int  NOT NULL CHECK (version >= 1),
    language       text NOT NULL CHECK (language IN ('de', 'en')),
    cover_letter   text NOT NULL,
    profile_line   text,
    highlights     jsonb NOT NULL DEFAULT '[]',
    mail_subject   text NOT NULL,
    mail_body      text NOT NULL,
    pdf            bytea,
    file_name      text,
    rendered_at    timestamptz,
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE application_documents;
DROP INDEX applications_documents_state;
ALTER TABLE applications DROP COLUMN gmail_draft_at, DROP COLUMN documents_error, DROP COLUMN documents_state;
