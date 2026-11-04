-- name: GetPreferences :one
SELECT value_json, schema_version, revision FROM app_settings WHERE key = 'ui_preferences';

-- name: UpdatePreferences :execrows
UPDATE app_settings
SET value_json = ?, revision = revision + 1, updated_at = ?
WHERE key = 'ui_preferences' AND revision = ?;
