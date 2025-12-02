-- +goose Up
-- +goose StatementBegin
CREATE TYPE contest_invitation_status AS ENUM ('pending', 'accepted', 'declined', 'revoked');

CREATE TABLE IF NOT EXISTS contest_invitations
(
    id         uuid PRIMARY KEY              DEFAULT uuid_generate_v4(),
    contest_id uuid                 NOT NULL REFERENCES contests (id) ON DELETE CASCADE,
    user_id    uuid                 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    invited_by uuid                 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status     contest_invitation_status NOT NULL DEFAULT 'pending',
    created_at timestamptz          NOT NULL DEFAULT now(),
    updated_at timestamptz          NOT NULL DEFAULT now()
);

-- Allow multiple invitations to same user if previous ones were revoked
CREATE UNIQUE INDEX IF NOT EXISTS contest_invitations_user_active_idx 
    ON contest_invitations (contest_id, user_id) 
    WHERE status != 'revoked';

CREATE INDEX IF NOT EXISTS contest_invitations_contest_id_idx ON contest_invitations (contest_id);
CREATE INDEX IF NOT EXISTS contest_invitations_user_id_idx ON contest_invitations (user_id);
CREATE INDEX IF NOT EXISTS contest_invitations_status_idx ON contest_invitations (contest_id, status);

CREATE TRIGGER on_contest_invitations_update
    BEFORE UPDATE
    ON contest_invitations
    FOR EACH ROW
EXECUTE FUNCTION updated_at_update();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS on_contest_invitations_update ON contest_invitations;
DROP INDEX IF EXISTS contest_invitations_status_idx;
DROP INDEX IF EXISTS contest_invitations_user_id_idx;
DROP INDEX IF EXISTS contest_invitations_contest_id_idx;
DROP INDEX IF EXISTS contest_invitations_user_active_idx;
DROP TABLE IF EXISTS contest_invitations;
DROP TYPE IF EXISTS contest_invitation_status;
-- +goose StatementEnd

