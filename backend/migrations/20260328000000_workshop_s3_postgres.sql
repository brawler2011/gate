-- +goose Up
-- +goose StatementBegin
ALTER TABLE problems
    ADD COLUMN manifest JSONB;

ALTER TABLE problems
    DROP COLUMN IF EXISTS git_commit_hash;

ALTER TABLE problem_packages
    DROP COLUMN IF EXISTS git_commit_hash;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE problems
    ADD COLUMN git_commit_hash VARCHAR(40);

ALTER TABLE problems
    DROP COLUMN IF EXISTS manifest;

ALTER TABLE problem_packages
    ADD COLUMN git_commit_hash VARCHAR(40) NOT NULL DEFAULT repeat('0', 40);

ALTER TABLE problem_packages
    DROP CONSTRAINT IF EXISTS problem_packages_git_commit_hash_check;

ALTER TABLE problem_packages
    ADD CONSTRAINT problem_packages_git_commit_hash_check CHECK (length(git_commit_hash) = 40);
-- +goose StatementEnd
