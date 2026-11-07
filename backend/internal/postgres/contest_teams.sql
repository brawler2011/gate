-- Contest Teams queries (team-based access control)

-- name: AddContestTeam :exec
INSERT INTO contest_teams (contest_id, team_id, role)
VALUES ($1, $2, $3);

-- name: ListContestTeams :many
SELECT ct.*, t.name as team_name, t.slug as team_slug
FROM contest_teams ct
JOIN teams t ON ct.team_id = t.id
WHERE ct.contest_id = $1
ORDER BY ct.created_at;

-- name: UpdateContestTeamRole :exec
UPDATE contest_teams
SET role = $3
WHERE contest_id = $1 AND team_id = $2;

-- name: RemoveContestTeam :exec
DELETE FROM contest_teams
WHERE contest_id = $1 AND team_id = $2;

-- name: GetTeamContests :many
SELECT c.* FROM contests c
INNER JOIN contest_teams ct ON c.id = ct.contest_id
WHERE ct.team_id = sqlc.arg('team_id')::uuid
  AND (
    c.visibility = 'public'
    OR c.owner_id = sqlc.arg('viewer_id')::uuid
    OR EXISTS (SELECT 1 FROM users u
               WHERE u.id = sqlc.arg('viewer_id')::uuid AND u.role = 'admin')
    OR EXISTS (SELECT 1 FROM organization_members om
               WHERE om.organization_id = c.organization_id
                 AND om.user_id = sqlc.arg('viewer_id')::uuid
                 AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM contest_members m
               WHERE m.contest_id = c.id AND m.user_id = sqlc.arg('viewer_id')::uuid)
    OR EXISTS (SELECT 1 FROM contest_teams linked
               JOIN team_members tm ON tm.team_id = linked.team_id
               JOIN teams t ON t.id = linked.team_id
               WHERE linked.contest_id = c.id AND tm.user_id = sqlc.arg('viewer_id')::uuid
                 AND t.organization_id = c.organization_id)
  )
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint;

-- name: CountTeamContests :one
SELECT COUNT(*) FROM contests c
INNER JOIN contest_teams ct ON c.id = ct.contest_id
WHERE ct.team_id = sqlc.arg('team_id')::uuid
  AND (
    c.visibility = 'public'
    OR c.owner_id = sqlc.arg('viewer_id')::uuid
    OR EXISTS (SELECT 1 FROM users u
               WHERE u.id = sqlc.arg('viewer_id')::uuid AND u.role = 'admin')
    OR EXISTS (SELECT 1 FROM organization_members om
               WHERE om.organization_id = c.organization_id
                 AND om.user_id = sqlc.arg('viewer_id')::uuid
                 AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM contest_members m
               WHERE m.contest_id = c.id AND m.user_id = sqlc.arg('viewer_id')::uuid)
    OR EXISTS (SELECT 1 FROM contest_teams linked
               JOIN team_members tm ON tm.team_id = linked.team_id
               JOIN teams t ON t.id = linked.team_id
               WHERE linked.contest_id = c.id AND tm.user_id = sqlc.arg('viewer_id')::uuid
                 AND t.organization_id = c.organization_id)
  );
