-- Contest Teams queries (team-based access control)

-- name: AddContestTeam :exec
INSERT INTO contest_teams (contest_id, team_id, role)
VALUES ($1, $2, $3);

-- name: GetContestTeam :one
SELECT * FROM contest_teams
WHERE contest_id = $1 AND team_id = $2;

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
WHERE ct.team_id = $1
ORDER BY c.created_at DESC;
