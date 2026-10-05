-- name: CreateCVReview :one
INSERT INTO cv_reviews (state, based_on_updated_at)
VALUES ('angefordert', $1)
RETURNING *;

-- name: GetCVReview :one
SELECT * FROM cv_reviews WHERE id = $1;

-- name: GetOpenCVReview :one
SELECT * FROM cv_reviews WHERE state IN ('angefordert', 'fertig');

-- name: ListCVReviewsByState :many
SELECT * FROM cv_reviews WHERE state = $1 ORDER BY requested_at;

-- name: CompleteCVReview :one
UPDATE cv_reviews
SET state = 'fertig', proposal = $2, notes = $3, completed_at = now()
WHERE id = $1 AND state = 'angefordert'
RETURNING *;

-- name: CloseOpenCVReviews :exec
UPDATE cv_reviews
SET state = 'abgeschlossen'
WHERE state IN ('angefordert', 'fertig');
