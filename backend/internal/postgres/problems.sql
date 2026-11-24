
-- name: CreateProblem :one
INSERT INTO problems (id, organization_id, visibility)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProblemByID :one
SELECT p.id, p.organization_id, o.login as org_login, o.name as org_name, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template
FROM problem_catalog p
JOIN organizations o ON p.organization_id = o.id
WHERE p.id = $1;



-- name: UpdateProblem :exec
UPDATE problems
SET visibility = COALESCE(sqlc.narg('visibility'), visibility),
    is_template = COALESCE(sqlc.narg('is_template'), is_template)
WHERE id = $1;

-- name: DeleteProblem :exec
DELETE FROM problems WHERE id = $1;


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
ORDER BY pm.created_at, pm.user_id
LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint;

-- name: UpdateProblemMemberRole :exec
UPDATE problem_members
SET role = $3
WHERE problem_id = $1 AND user_id = $2;

-- name: RemoveProblemMember :exec
DELETE FROM problem_members
WHERE problem_id = $1 AND user_id = $2;

-- name: CountProblemMembers :one
SELECT COUNT(*)
FROM problem_members pm
JOIN users u ON pm.user_id = u.id
WHERE pm.problem_id = $1;
