-- name: GetCV :one
SELECT data, updated_at FROM cv WHERE id = 1;

-- name: UpsertCV :one
INSERT INTO cv (id, data, updated_at)
VALUES (1, $1, now())
ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = now()
RETURNING data, updated_at;
