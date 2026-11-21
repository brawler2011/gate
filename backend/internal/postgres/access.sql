-- An expired account reads as missing.
-- name: GetAccessActor :one
SELECT id, role, expires_at FROM users
WHERE id = $1 AND (expires_at IS NULL OR expires_at > now());

-- Commands hold the account row so expiry or demotion waits for them or aborts them.
-- name: LockAccessActor :one
SELECT id, role, expires_at FROM users
WHERE id = $1 AND (expires_at IS NULL OR expires_at > now())
FOR SHARE;

-- name: LoadOrganizationAccess :many
SELECT o.id, o.join_policy, COALESCE(om.role::text, '')::text AS role
FROM organizations o
LEFT JOIN organization_members om ON om.organization_id = o.id AND om.user_id = sqlc.arg('user_id')::uuid
WHERE o.id = ANY(sqlc.arg('ids')::uuid[]);

-- name: LoadTeamAccess :many
SELECT t.id, t.organization_id, t.privacy, COALESCE(a.role::text, '')::text AS role,
    EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = sqlc.arg('user_id')::uuid) AS member
FROM teams t
LEFT JOIN team_access a ON a.team_id = t.id AND a.user_id = sqlc.arg('user_id')::uuid
WHERE t.id = ANY(sqlc.arg('ids')::uuid[]);

-- name: LoadProblemAccess :many
SELECT p.id, p.organization_id, p.visibility, COALESCE(a.role::text, '')::text AS role
FROM problems p
LEFT JOIN problem_access a ON a.problem_id = p.id AND a.user_id = sqlc.arg('user_id')::uuid
WHERE p.id = ANY(sqlc.arg('ids')::uuid[]);

-- name: LoadContestAccess :many
SELECT sqlc.embed(c), o.login AS org_login, COALESCE(a.role::text, '')::text AS role, now()::timestamptz AS now,
    ARRAY(SELECT b.problem_id FROM contest_user_problem_blocks b
          WHERE b.contest_id = c.id AND b.user_id = sqlc.arg('user_id')::uuid)::uuid[] AS blocked
FROM contests c
JOIN organizations o ON o.id = c.organization_id
LEFT JOIN contest_access a ON a.contest_id = c.id AND a.user_id = sqlc.arg('user_id')::uuid
WHERE c.id = ANY(sqlc.arg('ids')::uuid[]);
