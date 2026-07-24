-- Problems queries (new schema with Organizations)

-- name: CreateProblem :one
INSERT INTO problems (id, organization_id, owner_id, visibility, title, short_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProblemByID :one
SELECT id, organization_id, owner_id, visibility, title, short_name, created_at, updated_at, time_limit_ms, memory_limit_mb, is_template FROM problems WHERE id = $1;

-- name: GetProblemByShortName :one
SELECT id, organization_id, owner_id, visibility, title, short_name, created_at, updated_at, time_limit_ms, memory_limit_mb, is_template FROM problems
WHERE organization_id = $1 AND short_name = $2;

-- name: ListProblems :many
SELECT id, organization_id, owner_id, visibility, title, short_name, created_at, updated_at, time_limit_ms, memory_limit_mb, is_template FROM problems
WHERE organization_id = $1
  AND ($2::text = '' OR title ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR visibility = $3::problem_visibility)
  AND (sqlc.narg('is_template')::boolean IS NULL OR is_template = sqlc.narg('is_template')::boolean)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountProblems :one
SELECT COUNT(*) FROM problems
WHERE organization_id = $1
  AND ($2::text = '' OR title ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR visibility = $3::problem_visibility)
  AND (sqlc.narg('is_template')::boolean IS NULL OR is_template = sqlc.narg('is_template')::boolean);

-- name: ListAllProblems :many
SELECT p.id, p.organization_id, p.owner_id, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template FROM problems p
WHERE ($1::text = '' OR p.title ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR p.visibility = $2::problem_visibility)
  AND (sqlc.narg('is_template')::boolean IS NULL OR p.is_template = sqlc.narg('is_template')::boolean)
ORDER BY p.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAllProblems :one
SELECT COUNT(*) FROM problems p
WHERE ($1::text = '' OR p.title ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR p.visibility = $2::problem_visibility)
  AND (sqlc.narg('is_template')::boolean IS NULL OR p.is_template = sqlc.narg('is_template')::boolean);

-- name: UpdateProblem :exec
UPDATE problems
SET title = COALESCE(sqlc.narg('title'), title),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    owner_id = COALESCE(sqlc.narg('owner_id'), owner_id),
    is_template = COALESCE(sqlc.narg('is_template'), is_template)
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
SELECT p.id, p.organization_id, p.owner_id, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template FROM problems p
WHERE user_has_problem_access($1, p.id)
ORDER BY p.created_at DESC
LIMIT $2 OFFSET $3;

-- Workshop integration queries

-- name: UpdateProblemLimits :exec
UPDATE problems
SET time_limit_ms = $2, memory_limit_mb = $3
WHERE id = $1;

-- name: GetProblemManifest :one
SELECT manifest FROM problems WHERE id = $1;

-- name: UpdateProblemManifest :exec
UPDATE problems SET manifest = $2 WHERE id = $1;

-- name: ListDashboardProblems :many
SELECT 
    p.id as problem_id,
    p.title as problem_title,
    p.time_limit_ms,
    p.memory_limit_mb,
    p.updated_at,
    o.id as org_id,
    o.name as org_name
FROM problems p
JOIN organizations o ON p.organization_id = o.id
LEFT JOIN problem_members pm ON p.id = pm.problem_id
WHERE p.owner_id = $1 OR pm.user_id = $1
GROUP BY p.id, o.id
ORDER BY p.updated_at DESC
LIMIT $2;
