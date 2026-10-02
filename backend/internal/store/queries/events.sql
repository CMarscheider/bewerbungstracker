-- name: InsertEvent :one
INSERT INTO application_events (application_id, type, occurred_on, due_on, interview_round, note)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListEvents :many
SELECT * FROM application_events
WHERE application_id = $1
ORDER BY created_at;

-- name: DeleteEvent :exec
DELETE FROM application_events WHERE id = $1;
