-- +goose Up
-- Genau ein Lebenslauf; die Struktur legt das OpenAPI-Schema Cv fest.
CREATE TABLE cv (
    id         int PRIMARY KEY CHECK (id = 1),
    data       jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE cv;
