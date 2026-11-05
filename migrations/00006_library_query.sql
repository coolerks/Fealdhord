-- +goose Up
CREATE TABLE search_state (id INTEGER PRIMARY KEY CHECK(id=1), generation INTEGER NOT NULL DEFAULT 0);
INSERT INTO search_state(id,generation) VALUES(1,0);
CREATE TABLE article_query_birth (article_id TEXT PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE, generation INTEGER NOT NULL);
INSERT INTO article_query_birth(article_id,generation) SELECT id,0 FROM articles;
-- +goose StatementBegin
CREATE TRIGGER articles_generation_insert AFTER INSERT ON articles BEGIN
 UPDATE search_state SET generation=generation+1 WHERE id=1;
 INSERT INTO article_query_birth(article_id,generation) SELECT new.id,generation FROM search_state WHERE id=1;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER articles_generation_update AFTER UPDATE ON articles BEGIN
 UPDATE search_state SET generation=generation+1 WHERE id=1;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER articles_generation_delete AFTER DELETE ON articles BEGIN
 UPDATE search_state SET generation=generation+1 WHERE id=1;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER articles_generation_insert;
DROP TRIGGER articles_generation_update;
DROP TRIGGER articles_generation_delete;
DROP TABLE article_query_birth;
DROP TABLE search_state;
