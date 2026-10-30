-- +goose Up
ALTER TABLE problems ADD COLUMN workspace_hash text NOT NULL DEFAULT '';
ALTER TABLE problems DROP COLUMN manifest;

-- +goose Down
ALTER TABLE problems ADD COLUMN manifest jsonb;
ALTER TABLE problems DROP COLUMN workspace_hash;
