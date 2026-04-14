-- +goose Up
-- +goose StatementBegin
ALTER TABLE problems
    ADD COLUMN time_limit_ms  INT NOT NULL DEFAULT 1000,
    ADD COLUMN memory_limit_mb INT NOT NULL DEFAULT 256;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE problems
    DROP COLUMN time_limit_ms,
    DROP COLUMN memory_limit_mb;
-- +goose StatementEnd
