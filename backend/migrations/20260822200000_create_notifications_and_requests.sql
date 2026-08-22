-- +goose Up
-- Add join_policy to organizations
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS join_policy VARCHAR(32) NOT NULL DEFAULT 'by_request';

-- Notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(64) NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    link TEXT,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS notifications_user_unread_idx ON notifications(user_id, is_read, created_at DESC);
CREATE INDEX IF NOT EXISTS notifications_user_created_idx ON notifications(user_id, created_at DESC);

-- Organization Invitations table (Admin/Owner -> User)
CREATE TABLE IF NOT EXISTS organization_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL DEFAULT 'member',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS org_invitations_pending_unique_idx 
    ON organization_invitations(organization_id, user_id) 
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS org_invitations_user_idx ON organization_invitations(user_id, status);
CREATE INDEX IF NOT EXISTS org_invitations_org_idx ON organization_invitations(organization_id, status);

-- Organization Join Requests table (User -> Org)
CREATE TABLE IF NOT EXISTS organization_join_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS org_join_requests_pending_unique_idx 
    ON organization_join_requests(organization_id, user_id) 
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS org_join_requests_user_idx ON organization_join_requests(user_id, status);
CREATE INDEX IF NOT EXISTS org_join_requests_org_idx ON organization_join_requests(organization_id, status);

-- Contest Join Requests table (User -> Contest)
CREATE TABLE IF NOT EXISTS contest_join_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS contest_join_requests_pending_unique_idx 
    ON contest_join_requests(contest_id, user_id) 
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS contest_join_requests_user_idx ON contest_join_requests(user_id, status);
CREATE INDEX IF NOT EXISTS contest_join_requests_contest_idx ON contest_join_requests(contest_id, status);

-- +goose Down
DROP TABLE IF EXISTS contest_join_requests;
DROP TABLE IF EXISTS organization_join_requests;
DROP TABLE IF EXISTS organization_invitations;
DROP TABLE IF EXISTS notifications;
ALTER TABLE organizations DROP COLUMN IF EXISTS join_policy;
