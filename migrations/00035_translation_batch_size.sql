-- +goose Up
ALTER TABLE article_ai_settings ADD COLUMN translation_batch_kib INTEGER NOT NULL DEFAULT 0
 CHECK (translation_batch_kib BETWEEN 0 AND 1024);

-- +goose Down
ALTER TABLE article_ai_settings DROP COLUMN translation_batch_kib;
