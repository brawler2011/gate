-- Organizations queries

-- name: CreateOrganization :one
INSERT INTO organizations (id, login, name, description, avatar_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrganizationByLogin :one
SELECT * FROM organizations WHERE login = $1;

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
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url)
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
       u.username, u.email, u.name, u.surname
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
