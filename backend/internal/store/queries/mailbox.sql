-- name: ListOpenAgentApplications :many
-- Bewerbungen für die Postfach-Auswertung; statuses = Status der Phasen Vorbereitung und Aktiv plus KeineRueckmeldung.
SELECT a.id, c.name AS company_name, c.website AS company_website, a.position_title, a.current_status,
       a.contact_email, a.gmail_thread_id, a.documents_state, a.gmail_draft_at, d.mail_subject, a.updated_at
FROM applications a
JOIN companies c ON c.id = a.company_id
LEFT JOIN application_documents d ON d.application_id = a.id
WHERE a.current_status = ANY (sqlc.arg('statuses')::text[])
ORDER BY lower(c.name), lower(a.position_title), a.id;

-- name: SetGmailThread :exec
UPDATE applications SET gmail_thread_id = $2, updated_at = now() WHERE id = $1;

-- name: InsertSuggestion :one
INSERT INTO status_suggestions (application_id, suggested_type, occurred_on, due_on, reason,
                                mail_subject, mail_from, mail_url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListOpenSuggestions :many
SELECT s.id, s.application_id, c.name AS company_name, a.position_title, s.suggested_type, s.occurred_on,
       s.due_on, s.reason, s.mail_subject, s.mail_from, s.mail_url, s.state, s.created_at, s.decided_at
FROM status_suggestions s
LEFT JOIN applications a ON a.id = s.application_id
LEFT JOIN companies c ON c.id = a.company_id
WHERE s.state = 'offen'
ORDER BY s.created_at, s.id;

-- name: GetSuggestion :one
SELECT s.id, s.application_id, c.name AS company_name, a.position_title, s.suggested_type, s.occurred_on,
       s.due_on, s.reason, s.mail_subject, s.mail_from, s.mail_url, s.state, s.created_at, s.decided_at
FROM status_suggestions s
LEFT JOIN applications a ON a.id = s.application_id
LEFT JOIN companies c ON c.id = a.company_id
WHERE s.id = $1;

-- name: LockSuggestion :one
SELECT * FROM status_suggestions WHERE id = $1 FOR UPDATE;

-- name: DecideSuggestion :exec
-- Setzt die Entscheidung; application_id bleibt unverändert, wenn keine übergeben wird.
UPDATE status_suggestions
SET state = sqlc.arg(state), decided_at = now(),
    application_id = COALESCE(sqlc.narg(application_id), application_id)
WHERE id = sqlc.arg(id);

-- name: GetProcessedMail :one
SELECT * FROM processed_mails WHERE gmail_message_id = $1;

-- name: InsertProcessedMail :one
INSERT INTO processed_mails (gmail_message_id, application_id, outcome)
VALUES ($1, $2, $3)
ON CONFLICT (gmail_message_id) DO NOTHING
RETURNING *;
