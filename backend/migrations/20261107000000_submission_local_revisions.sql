-- +goose Up
ALTER TABLE submissions ALTER COLUMN revision SET DEFAULT 1;
ALTER TABLE contests DROP COLUMN submissions_revision;

-- +goose Down
ALTER TABLE submissions ALTER COLUMN revision SET DEFAULT 0;
ALTER TABLE contests ADD COLUMN submissions_revision bigint NOT NULL DEFAULT 0;
UPDATE contests c SET submissions_revision = (
    SELECT COALESCE(MAX(s.revision), 0) FROM submissions s WHERE s.contest_id = c.id
);
