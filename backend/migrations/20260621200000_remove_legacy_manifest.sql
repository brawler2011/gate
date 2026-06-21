-- +goose Up
ALTER TABLE problems DROP COLUMN manifest;

-- +goose Down
ALTER TABLE problems ADD COLUMN manifest JSONB;
