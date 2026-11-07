-- Problem Teams queries (team-based access control)

-- name: AddProblemTeam :exec
INSERT INTO problem_teams (problem_id, team_id, permission)
VALUES ($1, $2, $3);

-- name: ListProblemTeams :many
SELECT pt.*, t.name as team_name, t.slug as team_slug
FROM problem_teams pt
JOIN teams t ON pt.team_id = t.id
WHERE pt.problem_id = $1
ORDER BY pt.created_at;

-- name: UpdateProblemTeamPermission :exec
UPDATE problem_teams
SET permission = $3
WHERE problem_id = $1 AND team_id = $2;

-- name: RemoveProblemTeam :exec
DELETE FROM problem_teams
WHERE problem_id = $1 AND team_id = $2;

-- name: GetTeamProblems :many
SELECT p.* FROM problems p
INNER JOIN problem_teams pt ON p.id = pt.problem_id
WHERE pt.team_id = sqlc.arg('team_id')::uuid
  AND (
    p.visibility = 'public'
    OR p.owner_id = sqlc.arg('viewer_id')::uuid
    OR EXISTS (SELECT 1 FROM users u
               WHERE u.id = sqlc.arg('viewer_id')::uuid AND u.role = 'admin')
    OR EXISTS (SELECT 1 FROM organization_members om
               WHERE om.organization_id = p.organization_id
                 AND om.user_id = sqlc.arg('viewer_id')::uuid
                 AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM problem_members m
               WHERE m.problem_id = p.id AND m.user_id = sqlc.arg('viewer_id')::uuid)
    OR EXISTS (SELECT 1 FROM problem_teams linked
               JOIN team_members tm ON tm.team_id = linked.team_id
               JOIN teams t ON t.id = linked.team_id
               WHERE linked.problem_id = p.id AND tm.user_id = sqlc.arg('viewer_id')::uuid
                 AND t.organization_id = p.organization_id)
  )
ORDER BY p.created_at DESC;
