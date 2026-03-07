-- Problems queries (new schema with Organizations)

-- name: CreateProblem :one
INSERT INTO problems (id, organization_id, owner_id, visibility, titles, short_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProblemByID :one
SELECT id, organization_id, owner_id, visibility, titles, short_name, git_commit_hash, created_at, updated_at, time_limit_ms, memory_limit_mb FROM problems WHERE id = $1;

-- name: GetProblemByShortName :one
SELECT id, organization_id, owner_id, visibility, titles, short_name, git_commit_hash, created_at, updated_at, time_limit_ms, memory_limit_mb FROM problems 
WHERE organization_id = $1 AND short_name = $2;

-- name: ListProblems :many
SELECT id, organization_id, owner_id, visibility, titles, short_name, git_commit_hash, created_at, updated_at, time_limit_ms, memory_limit_mb FROM problems
WHERE organization_id = $1
  AND ($2::text = '' OR (titles->>'en') ILIKE '%' || $2 || '%' OR (titles->>'ru') ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR visibility = $3::problem_visibility)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountProblems :one
SELECT COUNT(*) FROM problems
WHERE organization_id = $1
  AND ($2::text = '' OR (titles->>'en') ILIKE '%' || $2 || '%' OR (titles->>'ru') ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR visibility = $3::problem_visibility);

-- name: ListAllProblems :many
SELECT p.id, p.organization_id, p.owner_id, p.visibility, p.titles, p.short_name, p.git_commit_hash, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb FROM problems p
WHERE ($1::text = '' OR (p.titles->>'en') ILIKE '%' || $1 || '%' OR (p.titles->>'ru') ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR p.visibility = $2::problem_visibility)
ORDER BY p.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAllProblems :one
SELECT COUNT(*) FROM problems p
WHERE ($1::text = '' OR (p.titles->>'en') ILIKE '%' || $1 || '%' OR (p.titles->>'ru') ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR p.visibility = $2::problem_visibility);

-- name: UpdateProblem :exec
UPDATE problems
SET titles = COALESCE(sqlc.narg('titles'), titles),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    owner_id = COALESCE(sqlc.narg('owner_id'), owner_id),
    git_commit_hash = COALESCE(sqlc.narg('git_commit_hash'), git_commit_hash)
WHERE id = $1;

-- name: DeleteProblem :exec
DELETE FROM problems WHERE id = $1;

-- Problem Members (direct access control)

-- name: AddProblemMember :exec
INSERT INTO problem_members (problem_id, user_id, role)
VALUES ($1, $2, $3);

-- name: GetProblemMember :one
SELECT * FROM problem_members
WHERE problem_id = $1 AND user_id = $2;

-- name: ListProblemMembers :many
SELECT pm.*, u.username, u.email
FROM problem_members pm
JOIN users u ON pm.user_id = u.id
WHERE pm.problem_id = $1
ORDER BY pm.created_at;

-- name: UpdateProblemMemberRole :exec
UPDATE problem_members
SET role = $3
WHERE problem_id = $1 AND user_id = $2;

-- name: RemoveProblemMember :exec
DELETE FROM problem_members
WHERE problem_id = $1 AND user_id = $2;

-- Access check helpers

-- name: CheckUserHasProblemAccess :one
SELECT user_has_problem_access($1, $2) as has_access;

-- name: ListUserAccessibleProblems :many
SELECT p.id, p.organization_id, p.owner_id, p.visibility, p.titles, p.short_name, p.git_commit_hash, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb FROM problems p
WHERE user_has_problem_access($1, p.id)
ORDER BY p.created_at DESC
LIMIT $2 OFFSET $3;

-- Workshop integration queries

-- name: UpdateProblemGitCommit :exec
UPDATE problems
SET git_commit_hash = $2
WHERE id = $1;

-- name: UpdateProblemLimits :exec
UPDATE problems
SET time_limit_ms = $2, memory_limit_mb = $3
WHERE id = $1;

-- name: GetProblemWorkshopStatus :one
SELECT id, titles, git_commit_hash, updated_at
FROM problems
WHERE id = $1;
