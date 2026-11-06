-- name: ListAIProviders :many
SELECT * FROM ai_providers ORDER BY created_at,id;

-- name: GetAIProvider :one
SELECT * FROM ai_providers WHERE id = ?;

-- name: CreateAIProvider :one
INSERT INTO ai_providers (id,name,protocol,base_url,credential_ref,enabled,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?) RETURNING *;

-- name: UpdateAIProvider :one
UPDATE ai_providers SET name = ?, protocol = ?, base_url = ?, credential_ref = ?,
 enabled = ?, revision = revision+1, updated_at = ?
WHERE id = ? AND revision = ? RETURNING *;

-- name: DeleteAIProvider :execrows
DELETE FROM ai_providers WHERE id = ? AND revision = ?;

-- name: ListAIModels :many
SELECT * FROM ai_models ORDER BY created_at,id;

-- name: CreateAIModel :one
INSERT INTO ai_models (id,provider_id,model,display_name,enabled,created_at,updated_at)
VALUES (?,?,?,?,?,?,?) RETURNING *;

-- name: UpdateAIModel :one
UPDATE ai_models SET model = ?, display_name = ?, enabled = ?, revision = revision+1, updated_at = ?
WHERE id = ? AND provider_id = ? AND revision = ? RETURNING *;

-- name: DeleteAIModel :execrows
DELETE FROM ai_models WHERE id = ? AND revision = ?;

-- name: ListModelAssignments :many
SELECT * FROM model_assignments ORDER BY task_type;

-- name: SetModelAssignment :execrows
INSERT INTO model_assignments (task_type,model_id)
SELECT sqlc.arg(task_type), m.id FROM ai_models m JOIN ai_providers p ON p.id=m.provider_id
WHERE m.id=sqlc.arg(model_id) AND m.enabled=1 AND p.enabled=1
ON CONFLICT(task_type) DO UPDATE SET model_id=excluded.model_id;

-- name: ClearModelAssignment :exec
DELETE FROM model_assignments WHERE task_type = ?;

-- name: ResolveModelAssignment :one
SELECT sqlc.embed(ai_providers), sqlc.embed(ai_models)
FROM model_assignments
JOIN ai_models ON ai_models.id=model_assignments.model_id
JOIN ai_providers ON ai_providers.id=ai_models.provider_id
WHERE task_type=? AND ai_models.enabled=1 AND ai_providers.enabled=1;

-- name: GetEnabledAIModel :one
SELECT sqlc.embed(ai_providers), sqlc.embed(ai_models)
FROM ai_models JOIN ai_providers ON ai_providers.id=ai_models.provider_id
WHERE ai_models.id=? AND ai_models.enabled=1 AND ai_providers.enabled=1;
