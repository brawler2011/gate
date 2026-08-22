-- +goose Up
-- Contest Announcements table (Broadcasts from jury to participants)
CREATE TABLE IF NOT EXISTS contest_announcements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    problem_id UUID REFERENCES problems(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS contest_announcements_contest_idx 
    ON contest_announcements(contest_id, created_at DESC);

-- Contest Clarifications table (Q&A between participant and jury)
CREATE TABLE IF NOT EXISTS contest_clarifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    problem_id UUID REFERENCES problems(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    answer TEXT,
    answered_by UUID REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    answered_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS contest_clarifications_contest_user_idx 
    ON contest_clarifications(contest_id, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS contest_clarifications_contest_status_idx 
    ON contest_clarifications(contest_id, status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS contest_clarifications;
DROP TABLE IF EXISTS contest_announcements;
