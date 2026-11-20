-- +goose Up
ALTER TABLE ai_models ADD COLUMN reasoning_levels TEXT NOT NULL DEFAULT '[]'
  CHECK (json_valid(reasoning_levels) AND json_type(reasoning_levels) = 'array');

-- +goose Down
SELECT 1;
