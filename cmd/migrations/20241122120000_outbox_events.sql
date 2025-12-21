-- +goose Up
-- +goose StatementBegin
CREATE TYPE outbox_event_status AS ENUM ('pending', 'processing', 'completed', 'failed');

CREATE TABLE IF NOT EXISTS outbox_events
(
    id             uuid PRIMARY KEY          DEFAULT uuid_generate_v4(),
    aggregate_id   uuid             NOT NULL,
    aggregate_type varchar(64)      NOT NULL,
    event_type     varchar(64)      NOT NULL,
    payload        jsonb            NOT NULL DEFAULT '{}',
    status         outbox_event_status NOT NULL DEFAULT 'pending',
    created_at     timestamptz      NOT NULL DEFAULT now(),
    processed_at   timestamptz,
    retry_count    integer          NOT NULL DEFAULT 0,
    error_message  text,
    CHECK (retry_count >= 0),
    CHECK (
        status IN ('pending', 'processing', 'completed', 'failed')
    )
);

CREATE INDEX IF NOT EXISTS outbox_events_status_created_at_idx ON outbox_events (status, created_at);
CREATE INDEX IF NOT EXISTS outbox_events_aggregate_idx ON outbox_events (aggregate_id, aggregate_type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS outbox_events_aggregate_idx;
DROP INDEX IF EXISTS outbox_events_status_created_at_idx;
DROP TABLE IF EXISTS outbox_events;
DROP TYPE IF EXISTS outbox_event_status;
-- +goose StatementEnd

