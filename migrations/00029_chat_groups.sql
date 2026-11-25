-- +goose Up
CREATE TABLE chat_groups (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE TABLE chat_group_folders (
 group_id TEXT NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
 position INTEGER NOT NULL,
 path TEXT NOT NULL,
 PRIMARY KEY(group_id, position),
 UNIQUE(group_id, path)
);
ALTER TABLE conversations ADD COLUMN group_id TEXT REFERENCES chat_groups(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN workspace_day TEXT NOT NULL DEFAULT '';
UPDATE conversations SET workspace_day=COALESCE(strftime('%Y-%m-%d', created_at / 1000, 'unixepoch', 'localtime'), '')
WHERE scope_type='global';
CREATE INDEX conversations_group ON conversations(group_id, pinned_at DESC, updated_at DESC, id);

-- +goose Down
SELECT 1;
