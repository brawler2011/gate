-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS contest_user_problem_blocks (
    contest_id   UUID NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    problem_id   UUID NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    reason       TEXT,
    created_by   UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, user_id, problem_id)
);

CREATE INDEX IF NOT EXISTS contest_user_problem_blocks_contest_idx ON contest_user_problem_blocks (contest_id);

ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS ban_reason TEXT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE submissions
    DROP COLUMN IF EXISTS ban_reason;

DROP TABLE IF EXISTS contest_user_problem_blocks;
-- +goose StatementEnd
