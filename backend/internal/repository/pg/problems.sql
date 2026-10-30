-- Problems queries (new schema with Organizations)

-- name: CreateProblem :one
INSERT INTO problems (id, organization_id, owner_id, visibility, title, short_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProblemByID :one
SELECT p.id, p.organization_id, o.login as org_login, o.name as org_name, p.owner_id, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template
FROM problems p
JOIN organizations o ON p.organization_id = o.id
WHERE p.id = $1;

-- name: GetProblemByShortName :one
SELECT p.id, p.organization_id, o.login as org_login, o.name as org_name, p.owner_id, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template
FROM problems p
JOIN organizations o ON p.organization_id = o.id
WHERE p.organization_id = $1 AND short_name = $2;

-- name: ListAllProblems :many
SELECT p.id, p.organization_id, o.login as org_login, o.name as org_name, p.owner_id, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template
FROM problems p
JOIN organizations o ON p.organization_id = o.id
WHERE (sqlc.narg('organization_id')::uuid IS NULL OR p.organization_id = sqlc.narg('organization_id')::uuid)
  AND (sqlc.narg('owner_id')::uuid IS NULL OR p.owner_id = sqlc.narg('owner_id')::uuid)
  AND (sqlc.arg('search')::text = '' OR p.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('visibility')::text = '' OR p.visibility = sqlc.arg('visibility')::problem_visibility)
  AND (sqlc.narg('is_template')::boolean IS NULL OR p.is_template = sqlc.narg('is_template')::boolean)
  AND (
    sqlc.narg('viewer_id')::uuid IS NULL
    OR p.visibility = 'public'
    OR (sqlc.narg('viewer_id')::uuid <> '00000000-0000-0000-0000-000000000000'::uuid AND (
      p.owner_id = sqlc.narg('viewer_id')::uuid
      OR EXISTS (SELECT 1 FROM users u WHERE u.id = sqlc.narg('viewer_id')::uuid AND u.role = 'admin')
      OR EXISTS (SELECT 1 FROM organization_members om
                 WHERE om.organization_id = p.organization_id AND om.user_id = sqlc.narg('viewer_id')::uuid
                   AND om.role IN ('owner', 'admin'))
      OR EXISTS (SELECT 1 FROM problem_members pm
                 WHERE pm.problem_id = p.id AND pm.user_id = sqlc.narg('viewer_id')::uuid)
      OR EXISTS (SELECT 1 FROM problem_teams pt
                 JOIN team_members tm ON tm.team_id = pt.team_id
                 JOIN teams t ON t.id = pt.team_id
                 WHERE pt.problem_id = p.id AND tm.user_id = sqlc.narg('viewer_id')::uuid
                   AND t.organization_id = p.organization_id)
    ))
  )
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: CountAllProblems :one
SELECT COUNT(*) FROM problems p
WHERE (sqlc.narg('organization_id')::uuid IS NULL OR p.organization_id = sqlc.narg('organization_id')::uuid)
  AND (sqlc.narg('owner_id')::uuid IS NULL OR p.owner_id = sqlc.narg('owner_id')::uuid)
  AND (sqlc.arg('search')::text = '' OR p.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('visibility')::text = '' OR p.visibility = sqlc.arg('visibility')::problem_visibility)
  AND (sqlc.narg('is_template')::boolean IS NULL OR p.is_template = sqlc.narg('is_template')::boolean)
  AND (
    sqlc.narg('viewer_id')::uuid IS NULL
    OR p.visibility = 'public'
    OR (sqlc.narg('viewer_id')::uuid <> '00000000-0000-0000-0000-000000000000'::uuid AND (
      p.owner_id = sqlc.narg('viewer_id')::uuid
      OR EXISTS (SELECT 1 FROM users u WHERE u.id = sqlc.narg('viewer_id')::uuid AND u.role = 'admin')
      OR EXISTS (SELECT 1 FROM organization_members om
                 WHERE om.organization_id = p.organization_id AND om.user_id = sqlc.narg('viewer_id')::uuid
                   AND om.role IN ('owner', 'admin'))
      OR EXISTS (SELECT 1 FROM problem_members pm
                 WHERE pm.problem_id = p.id AND pm.user_id = sqlc.narg('viewer_id')::uuid)
      OR EXISTS (SELECT 1 FROM problem_teams pt
                 JOIN team_members tm ON tm.team_id = pt.team_id
                 JOIN teams t ON t.id = pt.team_id
                 WHERE pt.problem_id = p.id AND tm.user_id = sqlc.narg('viewer_id')::uuid
                   AND t.organization_id = p.organization_id)
    ))
  )
;

-- name: UpdateProblem :exec
UPDATE problems
SET title = CASE WHEN workspace_hash = '' THEN COALESCE(sqlc.narg('title'), title) ELSE title END,
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
SELECT p.id, p.organization_id, o.login as org_login, o.name as org_name, p.owner_id, p.visibility, p.title, p.short_name, p.created_at, p.updated_at, p.time_limit_ms, p.memory_limit_mb, p.is_template
FROM problems p
JOIN organizations o ON p.organization_id = o.id
WHERE user_has_problem_access($1, p.id)
ORDER BY p.created_at DESC
LIMIT $2 OFFSET $3;

-- Workshop integration queries

-- name: GetWorkspaceHash :one
SELECT workspace_hash FROM problems WHERE id = $1;

-- name: CommitWorkspace :execrows
UPDATE problems
SET workspace_hash = sqlc.arg('hash'),
    title = COALESCE(sqlc.narg('title')::text, title),
    time_limit_ms = COALESCE(sqlc.narg('time_ms')::integer, time_limit_ms),
    memory_limit_mb = COALESCE(sqlc.narg('memory_mb')::integer, memory_limit_mb),
    updated_at = NOW()
WHERE id = sqlc.arg('id') AND workspace_hash = sqlc.arg('previous_hash');

-- name: ListDashboardProblems :many
SELECT 
    p.id as problem_id,
    p.title as problem_title,
    p.time_limit_ms,
    p.memory_limit_mb,
    p.updated_at,
    o.id as org_id,
    o.name as org_name,
    o.login as org_login
FROM problems p
JOIN organizations o ON p.organization_id = o.id
LEFT JOIN problem_members pm ON p.id = pm.problem_id
WHERE p.owner_id = $1 OR pm.user_id = $1
GROUP BY p.id, o.id
ORDER BY p.updated_at DESC
LIMIT $2;
