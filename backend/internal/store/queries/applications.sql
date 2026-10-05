-- name: CreateApplication :one
INSERT INTO applications (company_id, position_title, job_url, location, source, notes, current_status,
                          contact_email, posting_text, fit_score, fit_reason, created_by_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: LockApplication :one
SELECT * FROM applications WHERE id = $1 FOR UPDATE;

-- name: GetApplication :one
SELECT a.id, a.company_id, c.name AS company_name, a.position_title, a.job_url, a.location,
       a.source, a.notes, a.contact_email, a.posting_text, a.fit_score, a.fit_reason, a.created_by_agent,
       a.current_status, a.documents_state, a.documents_error, a.gmail_draft_at, a.created_at, a.updated_at
FROM applications a
JOIN companies c ON c.id = a.company_id
WHERE a.id = $1;

-- name: ListApplications :many
SELECT a.id, a.company_id, c.name AS company_name, a.position_title, a.current_status,
       a.updated_at, le.occurred_on AS last_event_on, le.due_on AS open_due_on,
       a.fit_score, a.created_by_agent
FROM applications a
JOIN companies c ON c.id = a.company_id
JOIN latest_events le ON le.application_id = a.id
WHERE (sqlc.narg('statuses')::text[] IS NULL OR a.current_status = ANY (sqlc.narg('statuses')::text[]))
  AND (sqlc.narg('query')::text IS NULL
       OR strpos(lower(c.name), lower(sqlc.narg('query')::text)) > 0
       OR strpos(lower(a.position_title), lower(sqlc.narg('query')::text)) > 0)
  AND (sqlc.narg('from_agent')::bool IS NULL OR a.created_by_agent = sqlc.narg('from_agent')::bool)
ORDER BY CASE WHEN sqlc.arg('by_score')::bool THEN a.fit_score END DESC NULLS LAST,
         a.updated_at DESC, a.id;

-- name: UpdateApplication :one
UPDATE applications
SET company_id = $2, position_title = $3, job_url = $4, location = $5, source = $6, notes = $7,
    contact_email = $8, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetApplicationStatus :exec
UPDATE applications SET current_status = $2, updated_at = now() WHERE id = $1;

-- name: DeleteApplication :execrows
DELETE FROM applications WHERE id = $1;

-- name: FindDuplicateApplication :one
-- Gleiche Anzeige (normalisierte URL) oder gleiche Firma + gleicher Titel, jeweils ohne Groß-/Kleinschreibung.
SELECT a.id
FROM applications a
JOIN companies c ON c.id = a.company_id
WHERE (sqlc.narg('job_url')::text IS NOT NULL
       AND lower(rtrim(a.job_url, '/')) = lower(rtrim(sqlc.narg('job_url')::text, '/')))
   OR (lower(c.name) = lower(sqlc.arg('company_name')::text)
       AND lower(a.position_title) = lower(sqlc.arg('position_title')::text))
ORDER BY a.created_at
LIMIT 1;
