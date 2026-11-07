-- Teams queries

-- name: CreateTeam :one
INSERT INTO teams (id, organization_id, name, slug, description, privacy, parent_team_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetTeamByID :one
SELECT * FROM teams WHERE id = $1;

-- name: GetTeamBySlug :one
SELECT * FROM teams WHERE organization_id = $1 AND slug = $2;



-- name: UpdateTeam :exec
UPDATE teams
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    privacy = COALESCE(sqlc.narg('privacy'), privacy)
WHERE id = $1;

-- name: DeleteTeam :exec
DELETE FROM teams WHERE id = $1;

-- Team Members

-- name: AddTeamMember :exec
INSERT INTO team_members (team_id, user_id, role)
VALUES ($1, $2, $3);

-- name: GetTeamMember :one
SELECT * FROM team_members
WHERE team_id = $1 AND user_id = $2;

-- name: ListTeamMembers :many
SELECT tm.team_id, tm.user_id, tm.role, tm.created_at,
       u.username, u.email
FROM team_members tm
JOIN users u ON tm.user_id = u.id
WHERE tm.team_id = $1
ORDER BY tm.created_at, tm.user_id
LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint;

-- name: UpdateTeamMemberRole :exec
UPDATE team_members
SET role = $3
WHERE team_id = $1 AND user_id = $2;

-- name: RemoveTeamMember :exec
DELETE FROM team_members
WHERE team_id = $1 AND user_id = $2;

-- name: GetUserTeamsByOrganization :many
SELECT t.* FROM teams t
INNER JOIN team_members tm ON t.id = tm.team_id
WHERE tm.user_id = $1 AND t.organization_id = $2
ORDER BY t.created_at DESC;


-- name: GetAuthorizationUserTeamsByOrganization :many
SELECT t.* FROM teams t
INNER JOIN team_members tm ON t.id = tm.team_id
WHERE tm.user_id = $1 AND t.organization_id = $2
ORDER BY t.created_at DESC
FOR SHARE OF tm;

-- name: GetAuthorizationTeamMember :one
SELECT * FROM team_members WHERE team_id = $1 AND user_id = $2 FOR SHARE;

-- name: CountTeamMembers :one
SELECT COUNT(*)
FROM team_members tm
JOIN users u ON tm.user_id = u.id
WHERE tm.team_id = $1;
