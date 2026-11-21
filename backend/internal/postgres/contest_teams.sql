
-- name: AddContestTeam :exec
INSERT INTO contest_teams (contest_id, team_id, organization_id, role)
SELECT c.id, sqlc.arg('team_id')::uuid, c.organization_id, sqlc.arg('role')::contest_role
FROM contests c WHERE c.id = sqlc.arg('contest_id')::uuid;

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

-- name: GetContestTeam :one
SELECT * FROM contest_teams WHERE contest_id = $1 AND team_id = $2;
