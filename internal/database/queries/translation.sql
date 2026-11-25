-- name: GetArticleTranslation :one
SELECT * FROM article_translations WHERE article_id=? AND target_lang=? AND source_mode=?;

-- name: GetCurrentTranslationSource :one
SELECT a.content_hash, a.summary, COALESCE(r.source_version,'') AS reader_source_version
FROM articles a JOIN feeds f ON f.id=a.feed_id
LEFT JOIN reader_contents r ON r.article_id=a.id AND r.content_hash=a.content_hash
WHERE a.id=? AND f.archived_at IS NULL;

-- name: SaveArticleTranslation :exec
INSERT INTO article_translations(
 article_id,target_lang,source_mode,content_hash,detected_lang,status,
 translated_title,translated_summary,blocks_json,translated_blocks,total_blocks,
 model_id,input_tokens,output_tokens,error_code,updated_at,source_version,checkpoint_json
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(article_id,target_lang,source_mode) DO UPDATE SET
 content_hash=excluded.content_hash,
 detected_lang=excluded.detected_lang,
 status=excluded.status,
 translated_title=excluded.translated_title,
 translated_summary=excluded.translated_summary,
 blocks_json=excluded.blocks_json,
 translated_blocks=excluded.translated_blocks,
 total_blocks=excluded.total_blocks,
 model_id=excluded.model_id,
 input_tokens=excluded.input_tokens,
 output_tokens=excluded.output_tokens,
 error_code=excluded.error_code,
 updated_at=excluded.updated_at,
 source_version=excluded.source_version,
 checkpoint_json=excluded.checkpoint_json;

-- name: DeleteArticleTranslations :exec
DELETE FROM article_translations WHERE article_id=? AND target_lang=?;

-- name: DeleteAllArticleTranslations :exec
DELETE FROM article_translations WHERE article_id=?;

-- name: GetArticleForTranslation :one
SELECT a.id,a.feed_id,a.title,a.summary,a.content_html,a.content_text,a.url,a.content_hash,
 f.url AS feed_url,f.auto_translate_mode
FROM articles a JOIN feeds f ON f.id=a.feed_id
WHERE a.id=? AND f.archived_at IS NULL;
