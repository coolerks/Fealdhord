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
UPDATE messages SET status=?, content=?, error_code=? WHERE id=? AND status='running';

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
