-- +goose Up
ALTER TABLE chat_groups ADD COLUMN icon TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_groups ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_groups ADD COLUMN archived_at INTEGER;

-- +goose Down
SELECT 1;
