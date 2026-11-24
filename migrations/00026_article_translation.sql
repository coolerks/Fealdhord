-- +goose Up
ALTER TABLE article_ai_settings ADD COLUMN auto_translate INTEGER NOT NULL DEFAULT 0 CHECK (auto_translate IN (0,1));
ALTER TABLE article_ai_settings ADD COLUMN auto_translate_scope TEXT NOT NULL DEFAULT 'summary' CHECK (auto_translate_scope IN ('summary','full'));
ALTER TABLE article_ai_settings ADD COLUMN translate_on_open INTEGER NOT NULL DEFAULT 1 CHECK (translate_on_open IN (0,1));
ALTER TABLE article_ai_settings ADD COLUMN translation_target_lang TEXT NOT NULL DEFAULT 'zh-CN';
ALTER TABLE article_ai_settings ADD COLUMN translation_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN auto_translate_mode TEXT NOT NULL DEFAULT 'global' CHECK (auto_translate_mode IN ('global','on','off'));

CREATE TABLE article_translations (
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
 target_lang TEXT NOT NULL,
 source_mode TEXT NOT NULL CHECK (source_mode IN ('summary','feed','reader')),
 content_hash TEXT NOT NULL,
 detected_lang TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK (status IN ('pending','translating','completed','skipped_same_lang','partial','failed')),
 translated_title TEXT NOT NULL DEFAULT '',
 translated_summary TEXT NOT NULL DEFAULT '',
 blocks_json TEXT NOT NULL DEFAULT '[]',
 translated_blocks INTEGER NOT NULL DEFAULT 0,
 total_blocks INTEGER NOT NULL DEFAULT 0,
 model_id TEXT NOT NULL DEFAULT '',
 input_tokens INTEGER,
 output_tokens INTEGER,
 error_code TEXT NOT NULL DEFAULT '',
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(article_id, target_lang, source_mode)
);
CREATE INDEX article_translations_status ON article_translations(status, updated_at);
CREATE INDEX article_translations_article ON article_translations(article_id, target_lang);

-- +goose Down
SELECT 1;
