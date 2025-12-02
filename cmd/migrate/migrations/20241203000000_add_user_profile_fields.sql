-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN email VARCHAR(255);
ALTER TABLE users ADD COLUMN name VARCHAR(100);
ALTER TABLE users ADD COLUMN surname VARCHAR(100);
ALTER TABLE users ADD COLUMN bio TEXT;
ALTER TABLE users ADD COLUMN img TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS users_email_idx ON users (email) WHERE email IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_email_idx;
ALTER TABLE users DROP COLUMN IF EXISTS img;
ALTER TABLE users DROP COLUMN IF EXISTS bio;
ALTER TABLE users DROP COLUMN IF EXISTS surname;
ALTER TABLE users DROP COLUMN IF EXISTS name;
ALTER TABLE users DROP COLUMN IF EXISTS email;
-- +goose StatementEnd

