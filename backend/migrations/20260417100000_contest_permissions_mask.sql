-- +goose Up
-- +goose StatementBegin

ALTER TABLE contest_members
    ADD COLUMN permissions_mask BIGINT NOT NULL DEFAULT 0;

ALTER TABLE contest_teams
    ADD COLUMN permissions_mask BIGINT NOT NULL DEFAULT 0;

-- owner/moderator have full contest action set in the current model.
UPDATE contest_members
SET permissions_mask = CASE role
    WHEN 'owner' THEN 255
    WHEN 'moderator' THEN 255
    WHEN 'participant' THEN 177
    ELSE 0
END;

UPDATE contest_teams
SET permissions_mask = CASE role
    WHEN 'owner' THEN 255
    WHEN 'moderator' THEN 255
    WHEN 'participant' THEN 177
    ELSE 0
END;

CREATE INDEX contest_members_permissions_mask_idx ON contest_members (permissions_mask);
CREATE INDEX contest_teams_permissions_mask_idx ON contest_teams (permissions_mask);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS contest_teams_permissions_mask_idx;
DROP INDEX IF EXISTS contest_members_permissions_mask_idx;

ALTER TABLE contest_teams
    DROP COLUMN IF EXISTS permissions_mask;

ALTER TABLE contest_members
    DROP COLUMN IF EXISTS permissions_mask;

-- +goose StatementEnd
