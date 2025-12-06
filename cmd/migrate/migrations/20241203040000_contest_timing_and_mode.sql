-- +goose Up
-- +goose StatementBegin
CREATE TYPE contest_scoring_mode AS ENUM ('points', 'binary');

ALTER TABLE contests
    ADD COLUMN start_time   timestamptz           DEFAULT now(),
    ADD COLUMN end_time     timestamptz           DEFAULT NULL,
    ADD COLUMN scoring_mode contest_scoring_mode  NOT NULL DEFAULT 'binary';

-- Add index for time-based queries
CREATE INDEX IF NOT EXISTS contests_time_idx ON contests (start_time, end_time);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS contests_time_idx;
ALTER TABLE contests DROP COLUMN IF EXISTS scoring_mode;
ALTER TABLE contests DROP COLUMN IF EXISTS end_time;
ALTER TABLE contests DROP COLUMN IF EXISTS start_time;
DROP TYPE IF EXISTS contest_scoring_mode;
-- +goose StatementEnd

