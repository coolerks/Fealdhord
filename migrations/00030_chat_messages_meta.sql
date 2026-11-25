-- +goose Up
ALTER TABLE messages ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN attachments TEXT NOT NULL DEFAULT '';

-- +goose Down
SELECT 1;
