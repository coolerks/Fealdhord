-- name: CreateConversation :one
INSERT INTO conversations (id,scope_type,scope_id,title,created_at,updated_at)
VALUES (?,?,?,?,?,?) RETURNING *;

-- name: ListConversations :many
SELECT * FROM conversations WHERE scope_type=? AND scope_id=? ORDER BY updated_at DESC,id LIMIT 100;

-- name: GetConversation :one
SELECT * FROM conversations WHERE id=?;

-- name: ListConversationMessages :many
SELECT * FROM (SELECT * FROM messages WHERE conversation_id=? ORDER BY position DESC LIMIT 100) ORDER BY position;

-- name: AppendChatMessage :one
INSERT INTO messages (id,conversation_id,position,role,content,status,model_name)
VALUES (sqlc.arg(id),sqlc.arg(conversation_id),
 (SELECT COALESCE(MAX(position),0)+1 FROM messages WHERE conversation_id=sqlc.arg(conversation_id)),
 sqlc.arg(role),sqlc.arg(content),sqlc.arg(status),sqlc.arg(model_name)) RETURNING *;

-- name: FinishChatMessage :execrows
UPDATE messages SET status=?, content=?, error_code=?, reply_details=? WHERE id=? AND status='running';

-- name: InterruptChatMessages :exec
UPDATE messages SET status='interrupted' WHERE status='running';

-- name: TouchConversation :exec
UPDATE conversations SET updated_at=? WHERE id=?;

-- name: ChatCategoryContext :many
SELECT a.title, a.summary FROM effective_article_categories e
JOIN articles a ON a.id=e.article_id
WHERE e.category_id=? ORDER BY a.sort_at DESC,a.id LIMIT 5;

-- name: CheckpointChatMessage :exec
UPDATE messages SET content=? WHERE id=? AND status='running';

-- name: ChatArticleContext :one
SELECT title, summary FROM articles WHERE id=?;

-- name: StartChatToolAudit :exec
INSERT INTO chat_tool_audit(message_id,call_id,tool_name,status,started_at)
VALUES(?,?,?,'running',?);

-- name: FinishChatToolAudit :exec
UPDATE chat_tool_audit SET status=?,error_code=?,finished_at=?
WHERE message_id=? AND call_id=? AND status='running';

-- name: InterruptChatToolAudit :exec
UPDATE chat_tool_audit SET status='interrupted',error_code='Interrupted',finished_at=?
WHERE status='running';

-- name: ListChatToolAudit :many
SELECT a.* FROM chat_tool_audit a JOIN (
 SELECT id FROM messages WHERE conversation_id=? ORDER BY position DESC LIMIT 100
) recent ON recent.id=a.message_id
ORDER BY a.rowid;

-- name: ListContextMessages :many
SELECT * FROM (
 SELECT * FROM messages WHERE conversation_id = sqlc.arg(conversation_id)
 AND position > sqlc.arg(context_after) ORDER BY position DESC LIMIT 200
) ORDER BY position;

-- name: SaveConversationContext :execrows
UPDATE conversations SET context_after = (SELECT COALESCE(MAX(position),0) FROM messages WHERE conversation_id=sqlc.arg(id)),
 context_summary = sqlc.arg(summary) WHERE id=sqlc.arg(id) AND scope_type='global';

-- name: RestoreConversationContext :execrows
UPDATE conversations SET context_after=0, context_summary='' WHERE id=? AND scope_type='global';

-- name: SaveContextPins :execrows
UPDATE conversations SET context_pins=? WHERE id=? AND scope_type='global';

-- name: RecordContextUsage :exec
UPDATE conversations SET last_input_tokens=?, last_usage_model=? WHERE id=?;

-- name: ArticleInToolScope :one
SELECT COUNT(1)
FROM articles a JOIN feeds f ON f.id=a.feed_id
WHERE a.id=sqlc.arg(article_id) AND f.archived_at IS NULL
AND ((sqlc.arg(scope_kind)='feed' AND a.feed_id=sqlc.arg(scope_id))
 OR (sqlc.arg(scope_kind)='category' AND EXISTS(
 SELECT 1 FROM effective_article_categories c WHERE c.article_id=a.id AND c.category_id=sqlc.arg(scope_id))));

-- name: SetAutomaticCompaction :execrows
UPDATE conversations SET auto_compact=? WHERE id=? AND scope_type='global';

-- name: AdvanceContextSummary :execrows
UPDATE conversations SET context_after=sqlc.arg(new_after), context_summary=sqlc.arg(summary)
WHERE id=sqlc.arg(id) AND context_after=sqlc.arg(old_after) AND auto_compact=1;

-- name: CreateChatBranch :one
INSERT INTO conversations (id, scope_type, scope_id, title, created_at, updated_at, context_after, context_summary, auto_compact)
SELECT sqlc.arg(new_id), c.scope_type, c.scope_id, c.title, sqlc.arg(now), sqlc.arg(now),
 min(c.context_after, m.position), CASE WHEN c.context_after <= m.position THEN c.context_summary ELSE '' END, c.auto_compact
FROM conversations c JOIN messages m ON m.conversation_id=c.id
WHERE c.id=sqlc.arg(source_id) AND m.id=sqlc.arg(message_id) AND m.role='assistant' AND m.status<>'running'
RETURNING *;

-- name: CopyChatBranchMessages :exec
INSERT INTO messages (id, conversation_id, position, role, content, status, error_code, model_name, reply_details)
SELECT lower(hex(randomblob(16))), sqlc.arg(new_id), source.position, source.role, source.content, source.status, source.error_code, source.model_name, source.reply_details
FROM messages source
WHERE source.conversation_id=sqlc.arg(source_id) AND source.position <= (
 SELECT branch_point.position FROM messages branch_point WHERE branch_point.id=sqlc.arg(message_id)
);
