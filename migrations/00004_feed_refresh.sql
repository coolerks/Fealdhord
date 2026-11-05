-- +goose Up
ALTER TABLE feeds ADD COLUMN last_error_code TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE feeds DROP COLUMN last_error_code;
