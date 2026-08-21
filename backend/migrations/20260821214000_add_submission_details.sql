-- +goose Up
ALTER TABLE submissions
    ADD COLUMN failed_test INTEGER NULL,
    ADD COLUMN test_details JSONB NULL;

-- +goose Down
ALTER TABLE submissions
    DROP COLUMN IF EXISTS failed_test,
    DROP COLUMN IF EXISTS test_details;
