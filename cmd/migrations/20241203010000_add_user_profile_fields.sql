-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
ADD COLUMN email VARCHAR(255) NOT NULL;
ALTER TABLE users
ADD COLUMN name VARCHAR(100) NOT NULL;
ALTER TABLE users
ADD COLUMN surname VARCHAR(100) NOT NULL;
ALTER TABLE users
ADD COLUMN bio TEXT NOT NULL;
ALTER TABLE users
ADD COLUMN img_id uuid references images(id);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_idx ON users (email)
WHERE email IS NOT NULL;
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