-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS posts
(
    id              UUID PRIMARY KEY     DEFAULT uuid_generate_v7(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    title           TEXT        NOT NULL,
    text            TEXT        NOT NULL,
    description     TEXT        NOT NULL,
    author_uuid     UUID        NOT NULL,
    author_name     TEXT        NOT NULL,
    image_key       TEXT        NOT NULL
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_author_uuid ON posts(author_uuid);

-- Add trigger to auto-update updated_at
CREATE TRIGGER posts_updated_at_trigger
    BEFORE UPDATE ON posts
    FOR EACH ROW
    EXECUTE FUNCTION updated_at_update();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS posts_updated_at_trigger ON posts;
DROP INDEX IF EXISTS idx_posts_author_uuid;
DROP INDEX IF EXISTS idx_posts_created_at;
DROP TABLE IF EXISTS posts;
-- +goose StatementEnd
