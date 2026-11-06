-- +goose Up
-- 早期开发库的 00006 已记账，但可能缺少分页出生表和对应写入逻辑。
CREATE TABLE IF NOT EXISTS article_query_birth (
 article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
 generation INTEGER NOT NULL
);
INSERT INTO article_query_birth(article_id,generation)
 SELECT a.id,s.generation FROM articles a CROSS JOIN search_state s
 WHERE s.id=1 AND NOT EXISTS (
  SELECT 1 FROM article_query_birth birth WHERE birth.article_id=a.id
 );
DROP TRIGGER IF EXISTS articles_generation_insert;
-- +goose StatementBegin
CREATE TRIGGER articles_generation_insert AFTER INSERT ON articles BEGIN
 UPDATE search_state SET generation=generation+1 WHERE id=1;
 INSERT INTO article_query_birth(article_id,generation) SELECT new.id,generation FROM search_state WHERE id=1;
END;
-- +goose StatementEnd

-- +goose Down
-- 保留 00006 的预期结构，避免降回 00007 后再次丢失查询能力。
SELECT 1;
