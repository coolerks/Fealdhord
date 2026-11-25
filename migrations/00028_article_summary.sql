-- +goose Up
ALTER TABLE articles ADD COLUMN feed_content_explicit INTEGER NOT NULL DEFAULT 0 CHECK (feed_content_explicit IN (0,1));
ALTER TABLE article_ai_settings ADD COLUMN summarize_on_open INTEGER NOT NULL DEFAULT 0 CHECK (summarize_on_open IN (0,1));
ALTER TABLE article_ai_settings ADD COLUMN summary_han_limit INTEGER NOT NULL DEFAULT 600 CHECK (summary_han_limit BETWEEN 1 AND 9007199254740991);
ALTER TABLE article_ai_settings ADD COLUMN summary_english_limit INTEGER NOT NULL DEFAULT 350 CHECK (summary_english_limit BETWEEN 1 AND 9007199254740991);

CREATE TABLE article_summaries (
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
 source_mode TEXT NOT NULL CHECK (source_mode IN ('feed','reader')),
 source_version TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('generated','skipped_length','skipped_model')),
 summary_text TEXT NOT NULL,
 model_id TEXT NOT NULL,
 han_limit INTEGER NOT NULL,
 english_limit INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY (article_id, source_mode, source_version),
 CHECK ((status = 'generated') = (length(summary_text) > 0)),
 CHECK ((status = 'skipped_length' AND han_limit > 0 AND english_limit > 0) OR
        (status <> 'skipped_length' AND han_limit = 0 AND english_limit = 0))
);

-- +goose Down
SELECT 1;
