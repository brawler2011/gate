-- +goose Up
DROP INDEX IF EXISTS organizations_login_idx;
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_login_key;
CREATE UNIQUE INDEX organizations_login_lower_idx ON organizations (LOWER(login));

-- +goose Down
DROP INDEX IF EXISTS organizations_login_lower_idx;
ALTER TABLE organizations ADD CONSTRAINT organizations_login_key UNIQUE (login);
CREATE INDEX organizations_login_idx ON organizations (login);
