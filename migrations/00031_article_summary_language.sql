-- +goose Up
ALTER TABLE article_summaries ADD COLUMN target_lang TEXT NOT NULL DEFAULT '';

-- +goose Down
SELECT 1;
