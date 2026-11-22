-- +goose Up
ALTER TABLE messages ADD COLUMN reply_details TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE messages DROP COLUMN reply_details;
