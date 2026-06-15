-- +goose Up
-- Truncate users and dependent tables because this is a breaking change
TRUNCATE TABLE users CASCADE;

-- Modify users table
ALTER TABLE users DROP COLUMN IF EXISTS kratos_id;
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL;

-- Create sessions table
CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- +goose Down
DROP TABLE IF EXISTS sessions;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users ADD COLUMN kratos_id UUID;
CREATE INDEX IF NOT EXISTS users_kratos_id_idx ON users (kratos_id);
