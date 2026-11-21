-- +goose Up
ALTER TABLE conversations ADD COLUMN context_pins TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(context_pins) AND json_type(context_pins)='array');
ALTER TABLE conversations ADD COLUMN last_input_tokens INTEGER NOT NULL DEFAULT 0 CHECK(last_input_tokens >= 0);
ALTER TABLE conversations ADD COLUMN last_usage_model TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN auto_compact INTEGER NOT NULL DEFAULT 0 CHECK(auto_compact IN (0, 1));
-- +goose Down
ALTER TABLE conversations DROP COLUMN auto_compact;
ALTER TABLE conversations DROP COLUMN last_usage_model;
ALTER TABLE conversations DROP COLUMN last_input_tokens;
ALTER TABLE conversations DROP COLUMN context_pins;
