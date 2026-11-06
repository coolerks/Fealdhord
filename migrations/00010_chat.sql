-- +goose Up
CREATE TABLE conversations (
 id TEXT PRIMARY KEY,
 scope_type TEXT NOT NULL CHECK (scope_type IN ('global','category','article')),
 scope_id TEXT NOT NULL,
 title TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
);
CREATE INDEX conversations_scope ON conversations(scope_type,scope_id,updated_at DESC);
CREATE TABLE messages (
 id TEXT PRIMARY KEY,
 conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
 position INTEGER NOT NULL,
 role TEXT NOT NULL CHECK (role IN ('user','assistant')),
 content TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('complete','running','failed','cancelled','interrupted')),
 error_code TEXT NOT NULL DEFAULT '',
 model_name TEXT NOT NULL DEFAULT '',
 UNIQUE(conversation_id,position)
);
CREATE UNIQUE INDEX messages_one_run ON messages(conversation_id) WHERE status='running';
-- +goose Down
SELECT 1;
