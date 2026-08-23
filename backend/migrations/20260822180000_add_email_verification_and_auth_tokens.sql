-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_email_verified BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users SET is_email_verified = TRUE;

DO $$ BEGIN
    CREATE TYPE auth_token_type AS ENUM (
        'email_verification',
        'password_reset',
        'email_change'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS auth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_type auth_token_type NOT NULL,
    token_hash TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS auth_tokens_user_id_type_idx ON auth_tokens (user_id, token_type);
CREATE INDEX IF NOT EXISTS auth_tokens_token_hash_idx ON auth_tokens (token_hash);
CREATE INDEX IF NOT EXISTS auth_tokens_expires_at_idx ON auth_tokens (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth_tokens;
DROP TYPE IF EXISTS auth_token_type;
ALTER TABLE users DROP COLUMN IF EXISTS is_email_verified;
-- +goose StatementEnd

