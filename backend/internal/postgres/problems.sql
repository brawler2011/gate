-- Problems queries (new schema with Organizations)

-- name: CreateProblem :one
INSERT INTO problems (id, organization_id, owner_id, visibility)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetProblemByID :one
SELECT p.id, p.organization_id, o.login as org_login, o.name as org_name, p.owner_id, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template
FROM problem_catalog p
JOIN organizations o ON p.organization_id = o.id
WHERE p.id = $1;



-- name: UpdateProblem :exec
UPDATE problems
SET visibility = COALESCE(sqlc.narg('visibility'), visibility),
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
ORDER BY pm.created_at, pm.user_id
LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint;

-- name: UpdateProblemMemberRole :exec
UPDATE problem_members
SET role = $3
WHERE problem_id = $1 AND user_id = $2;

-- name: RemoveProblemMember :exec
DELETE FROM problem_members
WHERE problem_id = $1 AND user_id = $2;

-- name: ListDashboardProblems :many
WITH actor AS (
  SELECT u.id, u.role FROM users u
  WHERE u.id = sqlc.narg('viewer_id')::uuid
    AND u.role IN ('user', 'admin')
    AND (u.expires_at IS NULL OR u.expires_at > sqlc.arg('now')::timestamptz)
)
SELECT 
    p.id as problem_id,
    p.title as problem_title,
    p.time_limit_ms,
    p.memory_limit_mb,
    p.updated_at,
    o.id as org_id,
    o.name as org_name,
    o.login as org_login
FROM problem_catalog p
JOIN organizations o ON p.organization_id = o.id
WHERE (sqlc.narg('viewer_id')::uuid IS NULL OR sqlc.narg('viewer_id')::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR EXISTS (SELECT 1 FROM actor))
AND NOT EXISTS (SELECT 1 FROM organization_members invalid WHERE invalid.organization_id = p.organization_id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('member', 'admin', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM problem_members invalid WHERE invalid.problem_id = p.id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('viewer', 'moderator', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM problem_teams link JOIN team_members tm ON tm.team_id = link.team_id WHERE link.problem_id = p.id AND tm.user_id = sqlc.narg('viewer_id')::uuid AND (tm.role::text NOT IN ('member', 'maintainer') OR link.permission::text NOT IN ('read', 'write', 'admin')))
AND p.visibility IN ('public', 'private') AND (p.visibility = 'public' OR EXISTS (
    SELECT 1 FROM actor a WHERE a.role = 'admin' OR p.owner_id = a.id
    OR EXISTS (SELECT 1 FROM organization_members om WHERE om.organization_id = p.organization_id AND om.user_id = a.id AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM problem_members m WHERE m.problem_id = p.id AND m.user_id = a.id AND m.role IN ('viewer', 'moderator', 'owner'))
    OR EXISTS (SELECT 1 FROM problem_teams pt
      JOIN teams t ON t.id = pt.team_id JOIN team_members tm ON tm.team_id = t.id
      WHERE pt.problem_id = p.id AND tm.user_id = a.id
        AND t.organization_id = p.organization_id AND tm.role IN ('member', 'maintainer')
        AND pt.permission IN ('read', 'write', 'admin'))))
ORDER BY p.updated_at DESC, p.id DESC
LIMIT sqlc.arg('page_size')::int;


-- name: GetAuthorizationProblemMember :one
SELECT * FROM problem_members
WHERE problem_id = $1 AND user_id = $2
FOR SHARE;

-- name: CountProblemMembers :one
SELECT COUNT(*)
FROM problem_members pm
JOIN users u ON pm.user_id = u.id
WHERE pm.problem_id = $1;
