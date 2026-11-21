-- +goose Up
ALTER TABLE ai_models ADD COLUMN metadata TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(metadata) AND json_type(metadata)='object');
-- +goose Down
ALTER TABLE ai_models DROP COLUMN metadata;
