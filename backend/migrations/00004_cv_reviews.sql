-- +goose Up
-- Optimierungsläufe für den Lebenslauf: angefordert (Seite) → fertig (Agent) → abgeschlossen (Seite).
CREATE TABLE cv_reviews (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    state               text NOT NULL CHECK (state IN ('angefordert', 'fertig', 'abgeschlossen')),
    based_on_updated_at timestamptz NOT NULL,
    proposal            jsonb,
    notes               jsonb NOT NULL DEFAULT '[]',
    requested_at        timestamptz NOT NULL DEFAULT now(),
    completed_at        timestamptz,
    CHECK (state <> 'fertig' OR proposal IS NOT NULL)
);

-- Höchstens ein offener Lauf.
CREATE UNIQUE INDEX cv_reviews_one_open ON cv_reviews ((true)) WHERE state IN ('angefordert', 'fertig');

-- +goose Down
DROP TABLE cv_reviews;
