-- Invitations & Join Requests queries

-- ============================================================================
-- Organization Invitations
-- ============================================================================

-- name: CreateOrganizationInvitation :one
INSERT INTO organization_invitations (
    id,
    organization_id,
    user_id,
    inviter_id,
    role,
    status,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    'pending',
    NOW(),
    NOW()
) RETURNING *;

-- name: GetOrganizationInvitationByID :one
SELECT oi.*, o.name as organization_name, o.login as organization_login, u.username, u.email, inv.username as inviter_username
FROM organization_invitations oi
JOIN organizations o ON oi.organization_id = o.id
JOIN users u ON oi.user_id = u.id
JOIN users inv ON oi.inviter_id = inv.id
WHERE oi.id = $1;

-- name: GetPendingOrganizationInvitation :one
SELECT * FROM organization_invitations
WHERE organization_id = $1 AND user_id = $2 AND status = 'pending'
LIMIT 1;

-- name: ListOrganizationInvitations :many
SELECT oi.*, u.username, u.email, inv.username as inviter_username
FROM organization_invitations oi
JOIN users u ON oi.user_id = u.id
JOIN users inv ON oi.inviter_id = inv.id
WHERE oi.organization_id = $1
  AND (sqlc.narg('status')::text IS NULL OR oi.status = sqlc.narg('status'))
ORDER BY oi.created_at DESC;

-- name: ListUserOrganizationInvitations :many
SELECT oi.*, o.name as organization_name, o.login as organization_login, o.avatar_url as organization_avatar_url, inv.username as inviter_username
FROM organization_invitations oi
JOIN organizations o ON oi.organization_id = o.id
JOIN users inv ON oi.inviter_id = inv.id
WHERE oi.user_id = $1
  AND (sqlc.narg('status')::text IS NULL OR oi.status = sqlc.narg('status'))
ORDER BY oi.created_at DESC;

-- name: UpdateOrganizationInvitationStatus :exec
UPDATE organization_invitations
SET status = $2, updated_at = NOW()
WHERE id = $1;

-- ============================================================================
-- Organization Join Requests
-- ============================================================================

-- name: CreateOrganizationJoinRequest :one
INSERT INTO organization_join_requests (
    id,
    organization_id,
    user_id,
    message,
    status,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    'pending',
    NOW(),
    NOW()
) RETURNING *;

-- name: GetOrganizationJoinRequestByID :one
SELECT ojr.*, o.name as organization_name, o.login as organization_login, u.username, u.email, rev.username as reviewer_username
FROM organization_join_requests ojr
JOIN organizations o ON ojr.organization_id = o.id
JOIN users u ON ojr.user_id = u.id
LEFT JOIN users rev ON ojr.reviewed_by = rev.id
WHERE ojr.id = $1;

-- name: GetPendingOrganizationJoinRequest :one
SELECT * FROM organization_join_requests
WHERE organization_id = $1 AND user_id = $2 AND status = 'pending'
LIMIT 1;

-- name: ListOrganizationJoinRequests :many
SELECT ojr.*, u.username, u.email, rev.username as reviewer_username
FROM organization_join_requests ojr
JOIN users u ON ojr.user_id = u.id
LEFT JOIN users rev ON ojr.reviewed_by = rev.id
WHERE ojr.organization_id = $1
  AND (sqlc.narg('status')::text IS NULL OR ojr.status = sqlc.narg('status'))
ORDER BY ojr.created_at DESC;

-- name: ListUserOrganizationJoinRequests :many
SELECT ojr.*, o.name as organization_name, o.login as organization_login
FROM organization_join_requests ojr
JOIN organizations o ON ojr.organization_id = o.id
WHERE ojr.user_id = $1
ORDER BY ojr.created_at DESC;

-- name: UpdateOrganizationJoinRequestStatus :exec
UPDATE organization_join_requests
SET status = $2, reviewed_by = $3, updated_at = NOW()
WHERE id = $1;

-- ============================================================================
-- Contest Join Requests
-- ============================================================================

-- name: CreateContestJoinRequest :one
INSERT INTO contest_join_requests (
    id,
    contest_id,
    user_id,
    message,
    status,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    'pending',
    NOW(),
    NOW()
) RETURNING *;

-- name: GetContestJoinRequestByID :one
SELECT cjr.*, c.title as contest_title, c.login as contest_login, o.login as organization_login, u.username, u.email, rev.username as reviewer_username
FROM contest_join_requests cjr
JOIN contests c ON cjr.contest_id = c.id
JOIN organizations o ON c.organization_id = o.id
JOIN users u ON cjr.user_id = u.id
LEFT JOIN users rev ON cjr.reviewed_by = rev.id
WHERE cjr.id = $1;

-- name: GetPendingContestJoinRequest :one
SELECT * FROM contest_join_requests
WHERE contest_id = $1 AND user_id = $2 AND status = 'pending'
LIMIT 1;

-- name: ListContestJoinRequests :many
SELECT cjr.*, u.username, u.email, rev.username as reviewer_username
FROM contest_join_requests cjr
JOIN users u ON cjr.user_id = u.id
LEFT JOIN users rev ON cjr.reviewed_by = rev.id
WHERE cjr.contest_id = $1
  AND (sqlc.narg('status')::text IS NULL OR cjr.status = sqlc.narg('status'))
ORDER BY cjr.created_at DESC;

-- name: ListUserContestJoinRequests :many
SELECT cjr.*, c.title as contest_title, c.login as contest_login, o.login as organization_login
FROM contest_join_requests cjr
JOIN contests c ON cjr.contest_id = c.id
JOIN organizations o ON c.organization_id = o.id
WHERE cjr.user_id = $1
ORDER BY cjr.created_at DESC;

-- name: UpdateContestJoinRequestStatus :exec
UPDATE contest_join_requests
SET status = $2, reviewed_by = $3, updated_at = NOW()
WHERE id = $1;
