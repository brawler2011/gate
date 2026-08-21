-- +goose Up
-- +goose StatementBegin
UPDATE submissions
SET test_details = jsonb_strip_nulls(
    COALESCE(test_details, '{}'::jsonb) ||
    jsonb_build_object(
        'time_stat', CASE WHEN time_stat > 0 THEN time_stat ELSE NULL END,
        'memory_stat', CASE WHEN memory_stat > 0 THEN memory_stat ELSE NULL END,
        'failed_test', failed_test
    )
)
WHERE time_stat > 0 OR memory_stat > 0 OR failed_test IS NOT NULL;

ALTER TABLE submissions
    DROP COLUMN IF EXISTS time_stat,
    DROP COLUMN IF EXISTS memory_stat,
    DROP COLUMN IF EXISTS failed_test;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS time_stat INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS memory_stat INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS failed_test INTEGER NULL;

UPDATE submissions
SET time_stat = COALESCE((test_details->>'time_stat')::integer, 0),
    memory_stat = COALESCE((test_details->>'memory_stat')::integer, 0),
    failed_test = (test_details->>'failed_test')::integer
WHERE test_details IS NOT NULL;
-- +goose StatementEnd
