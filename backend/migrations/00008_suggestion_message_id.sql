-- +goose Up
-- Mail, aus der ein Vorschlag stammt; verhindert doppelte Vorschläge zur selben Mail.
ALTER TABLE status_suggestions ADD COLUMN gmail_message_id text;
CREATE UNIQUE INDEX status_suggestions_gmail_message ON status_suggestions (gmail_message_id)
    WHERE gmail_message_id IS NOT NULL;

-- +goose Down
DROP INDEX status_suggestions_gmail_message;
ALTER TABLE status_suggestions DROP COLUMN gmail_message_id;
