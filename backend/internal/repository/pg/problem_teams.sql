-- Problem Teams queries (team-based access control)

-- name: AddProblemTeam :exec
INSERT INTO problem_teams (problem_id, team_id, permission)
VALUES ($1, $2, $3);

-- name: GetProblemTeam :one
SELECT * FROM problem_teams
WHERE problem_id = $1 AND team_id = $2;

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
WHERE pt.team_id = $1
ORDER BY p.created_at DESC;
