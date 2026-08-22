-- +goose Up
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
DROP INDEX IF EXISTS users_email_idx;
CREATE UNIQUE INDEX users_email_idx ON users (email) WHERE email IS NOT NULL;

ALTER TABLE users ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS claimed_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS users_claimed_by_user_id_idx ON users(claimed_by_user_id);
CREATE INDEX IF NOT EXISTS users_expires_at_idx ON users(expires_at);

-- +goose Down
DROP INDEX IF EXISTS users_expires_at_idx;
DROP INDEX IF EXISTS users_claimed_by_user_id_idx;
ALTER TABLE users DROP COLUMN IF EXISTS claimed_at;
ALTER TABLE users DROP COLUMN IF EXISTS claimed_by_user_id;
ALTER TABLE users DROP COLUMN IF EXISTS expires_at;
DROP INDEX IF EXISTS users_email_idx;
CREATE UNIQUE INDEX users_email_idx ON users (email);
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
