-- +goose Up
ALTER TABLE contests ADD COLUMN submissions_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE submissions ADD COLUMN revision bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE submissions DROP COLUMN revision;
ALTER TABLE contests DROP COLUMN submissions_revision;
