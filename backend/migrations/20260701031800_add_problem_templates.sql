-- +goose Up
ALTER TABLE problems ADD COLUMN is_template BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE problems DROP COLUMN is_template;
