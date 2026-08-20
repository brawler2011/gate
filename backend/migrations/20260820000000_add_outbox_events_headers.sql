-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox_events ADD COLUMN headers JSONB NOT NULL DEFAULT '{}'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE outbox_events DROP COLUMN IF EXISTS headers;
-- +goose StatementEnd
