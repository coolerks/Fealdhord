-- +goose Up
CREATE TABLE article_media_next (
 article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE, url TEXT, source TEXT,
 status TEXT NOT NULL DEFAULT 'none' CHECK(status IN ('none','pending','ready','failed','blocked')),
 width INTEGER CHECK(width>0),height INTEGER CHECK(height>0),alt TEXT NOT NULL DEFAULT '',checked_at INTEGER,retry_after INTEGER,cache_key TEXT,
 generation INTEGER NOT NULL DEFAULT 0,remote_url TEXT NOT NULL DEFAULT '',mime TEXT NOT NULL DEFAULT ''
);
INSERT INTO article_media_next(article_id,url,source,status,width,height,alt,checked_at,retry_after,cache_key)
SELECT article_id,url,source,status,width,height,alt,checked_at,retry_after,cache_key FROM article_media;
DROP TABLE article_media;
ALTER TABLE article_media_next RENAME TO article_media;
CREATE TABLE article_media_candidates (
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,position INTEGER NOT NULL CHECK(position BETWEEN 0 AND 23),
 url TEXT NOT NULL,source TEXT NOT NULL,alt TEXT NOT NULL DEFAULT '',PRIMARY KEY(article_id,position)
);
-- +goose Down
DROP TABLE article_media_candidates;
CREATE TABLE article_media_previous (
 article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,url TEXT,source TEXT,
 status TEXT NOT NULL DEFAULT 'none' CHECK(status IN ('none','pending','ready','failed')),
 width INTEGER CHECK(width>0),height INTEGER CHECK(height>0),alt TEXT NOT NULL DEFAULT '',checked_at INTEGER,retry_after INTEGER,cache_key TEXT
);
INSERT INTO article_media_previous(article_id,url,source,status,width,height,alt,checked_at,retry_after,cache_key)
SELECT article_id,url,source,CASE WHEN status='blocked' THEN 'none' ELSE status END,width,height,alt,checked_at,retry_after,cache_key FROM article_media;
DROP TABLE article_media;
ALTER TABLE article_media_previous RENAME TO article_media;
