-- name: CreateApplication :one
INSERT INTO applications (company_id, position_title, job_url, location, source, notes, current_status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: LockApplication :one
SELECT * FROM applications WHERE id = $1 FOR UPDATE;

-- name: GetApplication :one
SELECT a.id, a.company_id, c.name AS company_name, a.position_title, a.job_url, a.location,
       a.source, a.notes, a.current_status, a.created_at, a.updated_at
FROM applications a
JOIN companies c ON c.id = a.company_id
WHERE a.id = $1;

-- name: ListApplications :many
SELECT a.id, a.company_id, c.name AS company_name, a.position_title, a.current_status,
       a.updated_at, le.occurred_on AS last_event_on, le.due_on AS open_due_on
FROM applications a
JOIN companies c ON c.id = a.company_id
JOIN latest_events le ON le.application_id = a.id
WHERE (sqlc.narg('statuses')::text[] IS NULL OR a.current_status = ANY (sqlc.narg('statuses')::text[]))
  AND (sqlc.narg('query')::text IS NULL
       OR strpos(lower(c.name), lower(sqlc.narg('query')::text)) > 0
       OR strpos(lower(a.position_title), lower(sqlc.narg('query')::text)) > 0)
ORDER BY a.updated_at DESC, a.id;

-- name: UpdateApplication :one
UPDATE applications
SET company_id = $2, position_title = $3, job_url = $4, location = $5, source = $6, notes = $7,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetApplicationStatus :exec
UPDATE applications SET current_status = $2, updated_at = now() WHERE id = $1;

-- name: DeleteApplication :execrows
DELETE FROM applications WHERE id = $1;
