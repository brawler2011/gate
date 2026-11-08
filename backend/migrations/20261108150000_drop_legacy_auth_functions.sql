-- +goose Up
DROP FUNCTION IF EXISTS user_has_problem_access(UUID, UUID);
DROP FUNCTION IF EXISTS user_has_contest_access(UUID, UUID);
DROP FUNCTION IF EXISTS user_is_contest_moderator(UUID, UUID);

-- +goose Down
-- Obsolete access functions replaced by in-memory policies and SQL view predicates.
