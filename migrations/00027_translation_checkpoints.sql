-- +goose Up
ALTER TABLE reader_contents ADD COLUMN source_version TEXT NOT NULL DEFAULT '';
ALTER TABLE article_translations ADD COLUMN source_version TEXT NOT NULL DEFAULT '';
ALTER TABLE article_translations ADD COLUMN checkpoint_json TEXT NOT NULL DEFAULT '{}';

-- +goose Down
SELECT 1;
