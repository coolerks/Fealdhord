-- +goose Up
ALTER TABLE feeds ADD COLUMN custom_icon_url TEXT NOT NULL DEFAULT '';

-- +goose Down
SELECT 1;
