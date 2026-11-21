
-- name: AddProblemTeam :exec
INSERT INTO problem_teams (problem_id, team_id, organization_id, role)
SELECT p.id, sqlc.arg('team_id')::uuid, p.organization_id, sqlc.arg('role')::problem_role
FROM problems p WHERE p.id = sqlc.arg('problem_id')::uuid;

-- name: ListProblemTeams :many
SELECT pt.*, t.name as team_name, t.slug as team_slug
FROM problem_teams pt
JOIN teams t ON pt.team_id = t.id
WHERE pt.problem_id = $1
ORDER BY pt.created_at;

-- name: UpdateProblemTeamRole :exec
UPDATE problem_teams
SET role = $3
WHERE problem_id = $1 AND team_id = $2;

-- name: RemoveProblemTeam :exec
DELETE FROM problem_teams
WHERE problem_id = $1 AND team_id = $2;

-- name: GetProblemTeam :one
SELECT * FROM problem_teams WHERE problem_id = $1 AND team_id = $2;
