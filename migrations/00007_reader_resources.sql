-- +goose Up
ALTER TABLE reader_contents ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';
CREATE TABLE reader_resources (
 id TEXT PRIMARY KEY,
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
 article_hash TEXT NOT NULL,
 mode TEXT NOT NULL CHECK(mode IN ('feed','reader')),
 kind TEXT NOT NULL CHECK(kind IN ('image','link')),
 url TEXT NOT NULL
);
CREATE INDEX reader_resources_article ON reader_resources(article_id,mode);

-- +goose Down
DROP TABLE reader_resources;
ALTER TABLE reader_contents DROP COLUMN content_hash;
