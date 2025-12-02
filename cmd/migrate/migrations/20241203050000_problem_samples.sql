-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS problem_samples
(
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    problem_id uuid        NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    ordinal    integer     NOT NULL,
    input      text        NOT NULL DEFAULT '',
    output     text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (problem_id, ordinal),
    CHECK (ordinal > 0)
);

CREATE INDEX IF NOT EXISTS problem_samples_problem_id_idx ON problem_samples (problem_id, ordinal);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS problem_samples_problem_id_idx;
DROP TABLE IF EXISTS problem_samples;
-- +goose StatementEnd

