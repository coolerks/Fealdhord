-- +goose Up
ALTER TABLE conversations ADD COLUMN context_after INTEGER NOT NULL DEFAULT 0 CHECK(context_after >= 0);
ALTER TABLE conversations ADD COLUMN context_summary TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE conversations DROP COLUMN context_summary;
ALTER TABLE conversations DROP COLUMN context_after;
