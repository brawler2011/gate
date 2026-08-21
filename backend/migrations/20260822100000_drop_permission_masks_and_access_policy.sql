-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS contest_teams_permissions_mask_idx;
DROP INDEX IF EXISTS contest_members_permissions_mask_idx;

ALTER TABLE contest_teams
    DROP COLUMN IF EXISTS permissions_mask;

ALTER TABLE contest_members
    DROP COLUMN IF EXISTS permissions_mask;

ALTER TABLE contests
    DROP COLUMN IF EXISTS access_policy;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE contests
    ADD COLUMN IF NOT EXISTS access_policy JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE contest_members
    ADD COLUMN IF NOT EXISTS permissions_mask BIGINT NOT NULL DEFAULT 0;

ALTER TABLE contest_teams
    ADD COLUMN IF NOT EXISTS permissions_mask BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS contest_members_permissions_mask_idx ON contest_members (permissions_mask);
CREATE INDEX IF NOT EXISTS contest_teams_permissions_mask_idx ON contest_teams (permissions_mask);
-- +goose StatementEnd
