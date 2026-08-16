-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS contest_problem_results (
    contest_id       UUID NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    problem_id       UUID NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    solved           BOOLEAN NOT NULL DEFAULT FALSE,
    failed_attempts  INTEGER NOT NULL DEFAULT 0,
    first_ac_time    TIMESTAMPTZ,
    time_minutes     INTEGER,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, user_id, problem_id)
);

CREATE INDEX IF NOT EXISTS contest_problem_results_contest_idx ON contest_problem_results (contest_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS contest_problem_results;
-- +goose StatementEnd
