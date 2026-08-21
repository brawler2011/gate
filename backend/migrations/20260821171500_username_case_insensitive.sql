-- +goose Up
DROP INDEX IF EXISTS users_username_trgm_idx;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
CREATE UNIQUE INDEX users_username_lower_idx ON users (LOWER(username));
CREATE INDEX users_username_trgm_idx ON users USING GIN (LOWER(username) gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS users_username_lower_idx;
DROP INDEX IF EXISTS users_username_trgm_idx;
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);
CREATE INDEX users_username_trgm_idx ON users USING GIN (username gin_trgm_ops);
