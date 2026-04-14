-- +goose Up
-- +goose StatementBegin
ALTER TABLE problem_packages ADD COLUMN version INT;

UPDATE problem_packages pp
SET version = sub.rn
FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY problem_id ORDER BY created_at ASC) AS rn
    FROM problem_packages
) sub
WHERE pp.id = sub.id;

ALTER TABLE problem_packages ALTER COLUMN version SET NOT NULL;

ALTER TABLE problem_packages ADD CONSTRAINT uq_problem_packages_problem_version UNIQUE (problem_id, version);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE problem_packages DROP CONSTRAINT IF EXISTS uq_problem_packages_problem_version;
ALTER TABLE problem_packages DROP COLUMN IF EXISTS version;
-- +goose StatementEnd
