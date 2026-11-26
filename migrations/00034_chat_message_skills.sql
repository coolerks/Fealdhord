-- +goose Up
ALTER TABLE messages ADD COLUMN selected_skills TEXT;

-- +goose Down
SELECT 1;
