-- +goose Up
CREATE TABLE feeds (
 id TEXT PRIMARY KEY, title TEXT NOT NULL CHECK(length(trim(title))>0), url TEXT NOT NULL,
 normalized_url TEXT NOT NULL UNIQUE, site_url TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', icon_url TEXT NOT NULL DEFAULT '',
 etag TEXT NOT NULL DEFAULT '', last_modified TEXT NOT NULL DEFAULT '', last_checked_at INTEGER, last_success_at INTEGER, next_check_at INTEGER,
 error_count INTEGER NOT NULL DEFAULT 0 CHECK(error_count>=0), last_error TEXT NOT NULL DEFAULT '',
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, archived_at INTEGER, sort_position INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 0 CHECK(revision>=0)
);
CREATE TABLE articles (
 rowid INTEGER PRIMARY KEY, id TEXT NOT NULL UNIQUE, feed_id TEXT NOT NULL REFERENCES feeds(id), guid TEXT,
 title TEXT NOT NULL, url TEXT NOT NULL DEFAULT '', normalized_url TEXT, author TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '',
 content_html TEXT NOT NULL DEFAULT '', content_text TEXT NOT NULL DEFAULT '', cjk_terms TEXT NOT NULL DEFAULT '', published_at INTEGER, updated_at INTEGER,
 fetched_at INTEGER NOT NULL, sort_at INTEGER NOT NULL, content_hash TEXT NOT NULL, dedup_key TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 0 CHECK(revision>=0), UNIQUE(feed_id,dedup_key), UNIQUE(id,feed_id)
);
CREATE INDEX articles_feed_sort ON articles(feed_id,sort_at DESC,id DESC);
CREATE INDEX articles_sort ON articles(sort_at DESC,id DESC);
CREATE TABLE article_identities (
 article_id TEXT NOT NULL, feed_id TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('guid','url','fingerprint')), value TEXT NOT NULL CHECK(length(value)>0),
 FOREIGN KEY(article_id,feed_id) REFERENCES articles(id,feed_id) ON DELETE CASCADE,
 PRIMARY KEY(feed_id,kind,value)
);
CREATE INDEX identities_article ON article_identities(article_id);
CREATE TABLE categories (
 id TEXT PRIMARY KEY, parent_id TEXT REFERENCES categories(id) ON DELETE RESTRICT,
 name TEXT NOT NULL CHECK(length(trim(name))>0), normalized_name TEXT NOT NULL CHECK(length(normalized_name)>0), icon TEXT NOT NULL DEFAULT '', color TEXT NOT NULL DEFAULT '',
 description TEXT NOT NULL DEFAULT '', ai_enabled INTEGER NOT NULL DEFAULT 0 CHECK(ai_enabled IN (0,1)), ai_prompt TEXT NOT NULL DEFAULT '',
 ai_threshold REAL NOT NULL DEFAULT 0.7 CHECK(ai_threshold BETWEEN 0 AND 1), created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 sort_position INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 0 CHECK(revision>=0)
);
CREATE UNIQUE INDEX categories_root_name ON categories(normalized_name) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX categories_sibling_name ON categories(parent_id,normalized_name) WHERE parent_id IS NOT NULL;
CREATE TABLE feed_categories (
 feed_id TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
 created_at INTEGER NOT NULL, PRIMARY KEY(feed_id,category_id)
);
CREATE INDEX feed_categories_reverse ON feed_categories(category_id,feed_id);
CREATE TABLE article_categories (
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE, category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
 source TEXT NOT NULL CHECK(source IN ('manual','feed','ai')), state TEXT NOT NULL CHECK(state IN ('included','excluded')),
 confidence REAL CHECK(confidence BETWEEN 0 AND 1), reason TEXT, model_ref TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
 PRIMARY KEY(article_id,category_id,source)
);
CREATE INDEX article_categories_reverse ON article_categories(category_id,article_id);
CREATE VIEW effective_article_categories AS
SELECT DISTINCT ac.article_id,ac.category_id FROM article_categories ac JOIN categories c ON c.id=ac.category_id
WHERE ac.state='included' AND (
 ac.source='manual' OR ((ac.source='feed' OR (ac.source='ai' AND ac.confidence>=c.ai_threshold)) AND NOT EXISTS (
  SELECT 1 FROM article_categories manual WHERE manual.article_id=ac.article_id AND manual.category_id=ac.category_id AND manual.source='manual'
 ))
);
CREATE TABLE reading_states (
 article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
 is_read INTEGER NOT NULL CHECK(is_read IN (0,1)), read_at INTEGER, updated_at INTEGER NOT NULL, revision INTEGER NOT NULL DEFAULT 0 CHECK(revision>=0)
);
CREATE INDEX reading_states_read ON reading_states(is_read,article_id);
CREATE TABLE favorites (article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,created_at INTEGER NOT NULL);
CREATE TABLE article_media (
 article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE, url TEXT, source TEXT,
 status TEXT NOT NULL DEFAULT 'none' CHECK(status IN ('none','pending','ready','failed')), width INTEGER CHECK(width>0), height INTEGER CHECK(height>0),
 alt TEXT NOT NULL DEFAULT '', checked_at INTEGER, retry_after INTEGER, cache_key TEXT
);
CREATE TABLE reader_contents (
 article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
 sanitized_html TEXT NOT NULL, content_text TEXT NOT NULL, source_url TEXT NOT NULL, fetched_at INTEGER NOT NULL, extractor_version TEXT NOT NULL
);
-- +goose StatementBegin
CREATE TRIGGER category_insert_tree BEFORE INSERT ON categories BEGIN
 SELECT CASE WHEN NEW.parent_id=NEW.id THEN RAISE(ABORT,'category cycle') END;
 SELECT CASE WHEN (WITH RECURSIVE ancestors(id,parent_id,depth) AS (
 SELECT id,parent_id,1 FROM categories WHERE id=NEW.parent_id UNION ALL
 SELECT c.id,c.parent_id,a.depth+1 FROM categories c JOIN ancestors a ON c.id=a.parent_id
 ) SELECT COALESCE(MAX(depth),0)+1 FROM ancestors)>5 THEN RAISE(ABORT,'category depth') END;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER category_update_tree BEFORE UPDATE OF parent_id ON categories BEGIN
 SELECT CASE WHEN NEW.parent_id=NEW.id OR EXISTS (
 WITH RECURSIVE descendants(id) AS (SELECT id FROM categories WHERE parent_id=NEW.id UNION ALL SELECT c.id FROM categories c JOIN descendants d ON c.parent_id=d.id)
 SELECT 1 FROM descendants WHERE id=NEW.parent_id
 ) THEN RAISE(ABORT,'category cycle') END;
 SELECT CASE WHEN (
 WITH RECURSIVE ancestors(id,parent_id,depth) AS (SELECT id,parent_id,1 FROM categories WHERE id=NEW.parent_id UNION ALL SELECT c.id,c.parent_id,a.depth+1 FROM categories c JOIN ancestors a ON c.id=a.parent_id),
 descendants(id,depth) AS (SELECT id,1 FROM categories WHERE id=NEW.id UNION ALL SELECT c.id,d.depth+1 FROM categories c JOIN descendants d ON c.parent_id=d.id)
 SELECT (SELECT COALESCE(MAX(depth),0) FROM ancestors)+(SELECT MAX(depth) FROM descendants)
 )>5 THEN RAISE(ABORT,'category depth') END;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER category_update_tree;
DROP TRIGGER category_insert_tree;
DROP TABLE reader_contents;
DROP TABLE article_media;
DROP TABLE favorites;
DROP TABLE reading_states;
DROP VIEW effective_article_categories;
DROP TABLE article_categories;
DROP TABLE feed_categories;
DROP TABLE categories;
DROP TABLE article_identities;
DROP TABLE articles;
DROP TABLE feeds;
