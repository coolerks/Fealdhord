-- +goose Up
ALTER TABLE article_ai_settings ADD COLUMN requests_per_hour INTEGER NOT NULL DEFAULT 100 CHECK (requests_per_hour BETWEEN 1 AND 1000);
ALTER TABLE article_ai_settings ADD COLUMN tokens_per_hour INTEGER NOT NULL DEFAULT 200000 CHECK (tokens_per_hour BETWEEN 1000 AND 2000000);
ALTER TABLE article_ai_settings ADD COLUMN minimum_importance REAL NOT NULL DEFAULT 0.7 CHECK (minimum_importance BETWEEN 0 AND 1);
ALTER TABLE article_ai_settings ADD COLUMN minimum_relevance REAL NOT NULL DEFAULT 0.6 CHECK (minimum_relevance BETWEEN 0 AND 1);
ALTER TABLE article_ai_settings ADD COLUMN profile_revision INTEGER NOT NULL DEFAULT 1;
UPDATE article_ai_settings SET profile_revision=revision;

-- +goose Down
SELECT 1;
