-- +goose Up
ALTER TABLE article_ai_settings
  ADD COLUMN translation_engine TEXT NOT NULL DEFAULT 'ai'
  CHECK (translation_engine IN (
    'ai',
    'google_free','google_byok',
    'microsoft_edge','microsoft_azure',
    'alibaba','caiyun','baidu','youdao'
  ));

CREATE TABLE translation_providers (
  id TEXT PRIMARY KEY CHECK (id IN (
    'google_byok','microsoft_azure','alibaba','caiyun','baidu','youdao'
  )),
  app_id TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  endpoint TEXT NOT NULL DEFAULT '',
  edition TEXT NOT NULL DEFAULT '',
  credential_ref TEXT NOT NULL DEFAULT '',
  updated_at INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
SELECT 1;
