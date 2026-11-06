-- +goose Up
CREATE TABLE ai_providers (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 protocol TEXT NOT NULL CHECK (protocol IN ('chat_completions','responses','messages')),
 base_url TEXT NOT NULL,
 credential_ref TEXT NOT NULL DEFAULT '',
 enabled INTEGER NOT NULL CHECK (enabled IN (0,1)),
 revision INTEGER NOT NULL DEFAULT 1,
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL
);

CREATE TABLE ai_models (
 id TEXT PRIMARY KEY,
 provider_id TEXT NOT NULL REFERENCES ai_providers(id) ON DELETE CASCADE,
 model TEXT NOT NULL,
 display_name TEXT NOT NULL,
 enabled INTEGER NOT NULL CHECK (enabled IN (0,1)),
 revision INTEGER NOT NULL DEFAULT 1,
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 UNIQUE (provider_id, model)
);

CREATE TABLE model_assignments (
 task_type TEXT PRIMARY KEY CHECK (task_type IN ('chat','classification','summary','translation','featured')),
 model_id TEXT NOT NULL REFERENCES ai_models(id) ON DELETE CASCADE
);

-- +goose Down
SELECT 1;
