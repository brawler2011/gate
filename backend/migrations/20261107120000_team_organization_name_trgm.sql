-- +goose Up
CREATE INDEX teams_name_trgm_idx ON teams USING GIN (name gin_trgm_ops);
CREATE INDEX organizations_name_trgm_idx ON organizations USING GIN (name gin_trgm_ops);

-- +goose Down
DROP INDEX organizations_name_trgm_idx;
DROP INDEX teams_name_trgm_idx;
