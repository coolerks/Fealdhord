-- +goose Up
CREATE VIRTUAL TABLE article_fts USING fts5(title,author,summary,content_text,cjk_terms,content='articles',content_rowid='rowid',tokenize='unicode61');
-- +goose StatementBegin
CREATE TRIGGER articles_fts_insert AFTER INSERT ON articles BEGIN
 INSERT INTO article_fts(rowid,title,author,summary,content_text,cjk_terms) VALUES (new.rowid,new.title,new.author,new.summary,new.content_text,new.cjk_terms);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER articles_fts_delete AFTER DELETE ON articles BEGIN
 INSERT INTO article_fts(article_fts,rowid,title,author,summary,content_text,cjk_terms) VALUES ('delete',old.rowid,old.title,old.author,old.summary,old.content_text,old.cjk_terms);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER articles_fts_update AFTER UPDATE ON articles BEGIN
 INSERT INTO article_fts(article_fts,rowid,title,author,summary,content_text,cjk_terms) VALUES ('delete',old.rowid,old.title,old.author,old.summary,old.content_text,old.cjk_terms);
 INSERT INTO article_fts(rowid,title,author,summary,content_text,cjk_terms) VALUES (new.rowid,new.title,new.author,new.summary,new.content_text,new.cjk_terms);
END;
-- +goose StatementEnd
INSERT INTO article_fts(article_fts) VALUES('rebuild');

-- +goose Down
DROP TRIGGER articles_fts_update;
DROP TRIGGER articles_fts_delete;
DROP TRIGGER articles_fts_insert;
DROP TABLE article_fts;
