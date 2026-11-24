-- +goose Up
ALTER TABLE conversations ADD COLUMN pinned_at INTEGER;
ALTER TABLE conversations ADD COLUMN archived_at INTEGER;
CREATE INDEX conversations_active_order ON conversations(scope_type,scope_id,pinned_at DESC,updated_at DESC,id) WHERE archived_at IS NULL;
CREATE INDEX conversations_archive_order ON conversations(archived_at DESC,id) WHERE scope_type='global' AND archived_at IS NOT NULL;

-- +goose Down
SELECT 1;
