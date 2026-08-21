-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS contest_drafts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems (id) ON DELETE CASCADE,
    language INTEGER NOT NULL,
    code TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS contest_drafts_contest_user_problem_idx ON contest_drafts (contest_id, user_id, problem_id, created_at DESC);
CREATE INDEX IF NOT EXISTS contest_drafts_contest_user_idx ON contest_drafts (contest_id, user_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS contest_drafts;
-- +goose StatementEnd
