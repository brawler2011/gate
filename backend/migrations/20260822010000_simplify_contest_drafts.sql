-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS contest_drafts_contest_user_problem_idx;
ALTER TABLE contest_drafts DROP COLUMN IF EXISTS problem_id;
ALTER TABLE contest_drafts DROP COLUMN IF EXISTS language;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE contest_drafts ADD COLUMN IF NOT EXISTS problem_id UUID REFERENCES problems (id) ON DELETE CASCADE;
ALTER TABLE contest_drafts ADD COLUMN IF NOT EXISTS language INTEGER NOT NULL DEFAULT 10;
CREATE INDEX IF NOT EXISTS contest_drafts_contest_user_problem_idx ON contest_drafts (contest_id, user_id, problem_id, created_at DESC);
-- +goose StatementEnd
