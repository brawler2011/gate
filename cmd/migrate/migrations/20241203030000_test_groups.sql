-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS test_groups
(
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    problem_id uuid        NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    ordinal    integer     NOT NULL,
    name       varchar(64) NOT NULL DEFAULT '',
    points     integer     NOT NULL DEFAULT 0,
    is_sample  boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (problem_id, ordinal),
    CHECK (ordinal > 0),
    CHECK (points >= 0)
);

CREATE INDEX IF NOT EXISTS test_groups_problem_id_idx ON test_groups (problem_id, ordinal);

-- Add group_id to problem_tests table
ALTER TABLE problem_tests
    ADD COLUMN group_id uuid REFERENCES test_groups (id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS problem_tests_group_id_idx ON problem_tests (group_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS problem_tests_group_id_idx;
ALTER TABLE problem_tests DROP COLUMN IF EXISTS group_id;
DROP INDEX IF EXISTS test_groups_problem_id_idx;
DROP TABLE IF EXISTS test_groups;
-- +goose StatementEnd

