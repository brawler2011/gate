-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
ALTER COLUMN kratos_id TYPE uuid USING kratos_id::uuid;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
ALTER COLUMN kratos_id TYPE text USING kratos_id::text;
-- +goose StatementEnd