-- +goose Up
-- +goose StatementBegin
ALTER TABLE problems
    ADD COLUMN title TEXT;

UPDATE problems
SET title = COALESCE(NULLIF(titles->>'en', ''), short_name);

ALTER TABLE problems
    ALTER COLUMN title SET NOT NULL;

ALTER TABLE problems
    DROP CONSTRAINT IF EXISTS problems_titles_check;

DROP INDEX IF EXISTS problems_titles_trgm_idx;

ALTER TABLE problems
    DROP COLUMN titles;

CREATE INDEX problems_title_trgm_idx ON problems USING GIN (title gin_trgm_ops);

ALTER TABLE contests
    ADD COLUMN title TEXT;

UPDATE contests
SET title = COALESCE(NULLIF(titles->>'en', ''), short_name);

ALTER TABLE contests
    ALTER COLUMN title SET NOT NULL;

ALTER TABLE contests
    DROP CONSTRAINT IF EXISTS contests_titles_check;

DROP INDEX IF EXISTS contests_titles_trgm_idx;

ALTER TABLE contests
    DROP COLUMN titles;

CREATE INDEX contests_title_trgm_idx ON contests USING GIN (title gin_trgm_ops);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE problems
    ADD COLUMN titles JSONB;

UPDATE problems
SET titles = jsonb_build_object('en', title);

ALTER TABLE problems
    ALTER COLUMN titles SET NOT NULL;

ALTER TABLE problems
    ADD CONSTRAINT problems_titles_check CHECK (jsonb_typeof(titles) = 'object');

DROP INDEX IF EXISTS problems_title_trgm_idx;

ALTER TABLE problems
    DROP COLUMN title;

CREATE INDEX problems_titles_trgm_idx ON problems USING GIN ((titles->>'en') gin_trgm_ops);

ALTER TABLE contests
    ADD COLUMN titles JSONB;

UPDATE contests
SET titles = jsonb_build_object('en', title);

ALTER TABLE contests
    ALTER COLUMN titles SET NOT NULL;

ALTER TABLE contests
    ADD CONSTRAINT contests_titles_check CHECK (jsonb_typeof(titles) = 'object');

DROP INDEX IF EXISTS contests_title_trgm_idx;

ALTER TABLE contests
    DROP COLUMN title;

CREATE INDEX contests_titles_trgm_idx ON contests USING GIN ((titles->>'en') gin_trgm_ops);
-- +goose StatementEnd
