-- +goose Up
CREATE TABLE chat_tool_audit (
 message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
 call_id TEXT NOT NULL,
 tool_name TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('running','completed','denied','failed','cancelled','interrupted')),
 error_code TEXT NOT NULL DEFAULT '',
 started_at INTEGER NOT NULL,
 finished_at INTEGER,
 PRIMARY KEY(message_id,call_id)
);

-- +goose Down
SELECT 1;
