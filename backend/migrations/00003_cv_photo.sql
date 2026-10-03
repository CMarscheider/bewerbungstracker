-- +goose Up
-- Genau ein Bewerbungsfoto (JPEG), getrennt vom Lebenslauf-JSON.
CREATE TABLE cv_photo (
    id         int PRIMARY KEY CHECK (id = 1),
    image      bytea NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE cv_photo;
