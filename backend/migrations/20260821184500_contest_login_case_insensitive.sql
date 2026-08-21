-- +goose Up
DROP INDEX IF EXISTS contests_short_name_idx;
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_organization_id_short_name_key;
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_short_name_check;

ALTER TABLE contests RENAME COLUMN short_name TO login;
ALTER TABLE contests ALTER COLUMN login TYPE VARCHAR(64);
ALTER TABLE contests ADD CONSTRAINT contests_login_check CHECK (length(login) >= 3 AND length(login) <= 64);
CREATE UNIQUE INDEX contests_org_id_login_lower_idx ON contests (organization_id, LOWER(login));

-- +goose Down
DROP INDEX IF EXISTS contests_org_id_login_lower_idx;
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_login_check;
ALTER TABLE contests RENAME COLUMN login TO short_name;
ALTER TABLE contests ALTER COLUMN short_name TYPE VARCHAR(100);
ALTER TABLE contests ADD CONSTRAINT contests_short_name_check CHECK (length(short_name) > 0);
ALTER TABLE contests ADD CONSTRAINT contests_organization_id_short_name_key UNIQUE (organization_id, short_name);
CREATE INDEX contests_short_name_idx ON contests (short_name);
