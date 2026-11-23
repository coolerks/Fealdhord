-- +goose Up
ALTER TABLE feeds ADD COLUMN reader_open_mode TEXT NOT NULL DEFAULT 'global';

-- +goose Down
SELECT 1;
