-- Organizations queries

-- name: CreateOrganization :one
INSERT INTO organizations (id, login, name, description, avatar_url, join_policy)
VALUES ($1, $2, $3, $4, $5, COALESCE($6, 'by_request'))
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrganizationByLogin :one
SELECT * FROM organizations WHERE LOWER(login) = LOWER($1);

-- name: ListOrganizations :many
SELECT * FROM organizations
WHERE ($1::text = '' OR name ILIKE '%' || $1 || '%')
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountOrganizations :one
SELECT COUNT(*) FROM organizations
WHERE ($1::text = '' OR name ILIKE '%' || $1 || '%');

-- name: UpdateOrganization :exec
UPDATE organizations
SET login = COALESCE(sqlc.narg('login'), login),
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    join_policy = COALESCE(sqlc.narg('join_policy'), join_policy)
WHERE id = $1;

-- name: DeleteOrganization :exec
DELETE FROM organizations WHERE id = $1;

-- Organization Members

-- name: AddOrganizationMember :exec
INSERT INTO organization_members (organization_id, user_id, role)
VALUES ($1, $2, $3);

-- name: GetOrganizationMember :one
SELECT * FROM organization_members
WHERE organization_id = $1 AND user_id = $2;

-- name: ListOrganizationMembers :many
SELECT om.organization_id, om.user_id, om.role, om.created_at,
       u.username, u.email
FROM organization_members om
JOIN users u ON om.user_id = u.id
WHERE om.organization_id = $1
ORDER BY om.created_at;

-- name: UpdateOrganizationMemberRole :exec
UPDATE organization_members
SET role = $3
WHERE organization_id = $1 AND user_id = $2;

-- name: RemoveOrganizationMember :exec
DELETE FROM organization_members
WHERE organization_id = $1 AND user_id = $2;

-- name: RemoveTeamMembersByOrgAndUser :exec
DELETE FROM team_members
WHERE user_id = $2
  AND team_id IN (SELECT id FROM teams WHERE organization_id = $1);

-- name: RemoveContestMembersByOrgAndUser :exec
DELETE FROM contest_members
WHERE user_id = $2
  AND contest_id IN (SELECT id FROM contests WHERE organization_id = $1);

-- name: RemoveProblemMembersByOrgAndUser :exec
DELETE FROM problem_members
WHERE user_id = $2
  AND problem_id IN (SELECT id FROM problems WHERE organization_id = $1);

-- name: GetUserOrganizations :many
SELECT o.* FROM organizations o
INNER JOIN organization_members om ON o.id = om.organization_id
WHERE om.user_id = $1
ORDER BY o.created_at DESC;

-- name: GetLatestUserOrganizationID :one
SELECT om.organization_id
FROM organization_members om
WHERE om.user_id = $1
ORDER BY om.created_at DESC
LIMIT 1;

-- name: GetSpecificUserOrganizationID :one
SELECT om.organization_id
FROM organization_members om
WHERE om.user_id = $1 AND om.organization_id = $2
LIMIT 1;
