-- +goose Up
-- revision=1 表示用户尚未保存文章 AI 设置。
UPDATE article_ai_settings
SET translate_on_open=0, summarize_on_open=1
WHERE id=1 AND revision=1;

-- +goose Down
SELECT 1;
