-- name: GetArticleSummary :one
SELECT * FROM article_summaries
WHERE article_id=? AND source_mode=? AND source_version=?;

-- name: SaveArticleSummary :exec
INSERT INTO article_summaries(article_id,source_mode,source_version,status,summary_text,model_id,han_limit,english_limit,updated_at,target_lang)
VALUES(?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(article_id,source_mode,source_version) DO UPDATE SET
 status=excluded.status,summary_text=excluded.summary_text,model_id=excluded.model_id,
 han_limit=excluded.han_limit,english_limit=excluded.english_limit,updated_at=excluded.updated_at,target_lang=excluded.target_lang;

-- name: GetArticleSummarySource :one
SELECT a.title,a.content_hash,a.feed_content_explicit,COALESCE(r.source_version,'') AS reader_source_version
FROM articles a LEFT JOIN reader_contents r ON r.article_id=a.id AND r.content_hash=a.content_hash
WHERE a.id=?;
