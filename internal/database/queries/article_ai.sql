-- name: GetArticleAISettings :one
SELECT * FROM article_ai_settings WHERE id=1;

-- name: SaveArticleAISettings :one
UPDATE article_ai_settings SET classification=?,featured=?,prompt=?,revision=revision+1
WHERE id=1 AND revision=? RETURNING *;

-- name: ArticleAIInput :one
SELECT a.id, a.content_hash, substr(a.title,1,512) AS title, substr(a.summary,1,2048) AS summary,
 substr(a.content_text,1,4096) AS body
FROM articles a JOIN feeds f ON f.id=a.feed_id WHERE a.id=? AND f.archived_at IS NULL;

-- name: ArticleAICategories :many
SELECT id,name,description FROM categories ORDER BY id LIMIT 129;

-- name: GetArticleAIResult :one
SELECT * FROM article_ai_results WHERE article_id=? AND task=?;

-- name: SaveArticleAIResult :exec
INSERT INTO article_ai_results(article_id,task,input_key,profile_key,content_hash,model_id,model_revision,provider_revision,settings_revision,importance,relevance,reason,input_tokens,output_tokens,evaluated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(article_id,task) DO UPDATE SET input_key=excluded.input_key,profile_key=excluded.profile_key,content_hash=excluded.content_hash,
 model_id=excluded.model_id,model_revision=excluded.model_revision,provider_revision=excluded.provider_revision,
 settings_revision=excluded.settings_revision,importance=excluded.importance,relevance=excluded.relevance,reason=excluded.reason,
 input_tokens=excluded.input_tokens,output_tokens=excluded.output_tokens,evaluated_at=excluded.evaluated_at;

-- name: DeleteArticleAIMemberships :exec
DELETE FROM article_categories WHERE article_id=? AND source='ai';

-- name: AddArticleAIMembership :exec
INSERT INTO article_categories(article_id,category_id,source,state,confidence,reason,model_ref,created_at,updated_at)
VALUES(?,?,'ai','included',?,?,?,?,?);

-- name: PruneArticleAIAttempts :exec
DELETE FROM article_ai_attempts WHERE started_at<=?;

-- name: ReserveArticleAIAttempt :execrows
INSERT INTO article_ai_attempts(id,started_at,estimated_tokens)
SELECT ?,?,sqlc.arg(estimate)
WHERE (SELECT count(*) FROM article_ai_attempts)<100
 AND (SELECT CAST(COALESCE(sum(estimated_tokens),0) AS INTEGER) FROM article_ai_attempts)+sqlc.arg(estimate)<=200000;

-- name: IncrementSearchGeneration :exec
UPDATE search_state SET generation=generation+1 WHERE id=1;

-- name: RecordArticleAIDefer :exec
INSERT INTO article_ai_deferrals(article_id,task,until_at) VALUES(?,?,?)
ON CONFLICT(article_id,task) DO UPDATE SET until_at=excluded.until_at;
