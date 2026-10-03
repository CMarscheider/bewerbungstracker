-- name: GetCVPhoto :one
SELECT image, updated_at FROM cv_photo WHERE id = 1;

-- name: UpsertCVPhoto :exec
INSERT INTO cv_photo (id, image, updated_at)
VALUES (1, $1, now())
ON CONFLICT (id) DO UPDATE SET image = EXCLUDED.image, updated_at = now();

-- name: DeleteCVPhoto :exec
DELETE FROM cv_photo WHERE id = 1;
