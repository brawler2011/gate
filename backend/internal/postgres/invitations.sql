-- name: CreateInvitation :one
INSERT INTO invitations (organization_id, contest_id, role, user_id, inviter_id)
VALUES (sqlc.narg('organization_id'), sqlc.narg('contest_id'), $1, $2, $3)
RETURNING id;

-- name: GetInvitation :one
SELECT i.id, i.organization_id, i.team_id, i.contest_id, i.role, i.user_id, i.created_at,
    u.username, inv.username AS inviter_username,
    o.id AS target_organization_id, o.login AS organization_login, o.name AS organization_name,
    c.login AS contest_login, c.title AS contest_title
FROM invitations i
JOIN users inv ON inv.id = i.inviter_id
LEFT JOIN users u ON u.id = i.user_id
LEFT JOIN teams t ON t.id = i.team_id
LEFT JOIN contests c ON c.id = i.contest_id
JOIN organizations o ON o.id = COALESCE(i.organization_id, t.organization_id, c.organization_id)
WHERE i.id = $1;

-- name: ListInvitations :many
SELECT i.id, i.role, i.user_id, i.created_at, u.username, inv.username AS inviter_username
FROM invitations i
JOIN users u ON u.id = i.user_id
JOIN users inv ON inv.id = i.inviter_id
WHERE (i.organization_id = sqlc.narg('organization_id') OR i.contest_id = sqlc.narg('contest_id'))
ORDER BY i.created_at DESC;

-- name: DeleteInvitation :execrows
DELETE FROM invitations WHERE id = $1 AND user_id IS NOT NULL;

-- name: GetJoinLink :one
SELECT token FROM invitations
WHERE (team_id = sqlc.narg('team_id') OR contest_id = sqlc.narg('contest_id')) AND user_id IS NULL;

-- name: SetJoinLink :exec
INSERT INTO invitations (team_id, contest_id, role, token, inviter_id)
VALUES (sqlc.narg('team_id'), sqlc.narg('contest_id'), $1, sqlc.arg('token')::text, $2)
ON CONFLICT (organization_id, team_id, contest_id, user_id)
DO UPDATE SET token = EXCLUDED.token, inviter_id = EXCLUDED.inviter_id, created_at = now();

-- name: DeleteJoinLink :exec
DELETE FROM invitations
WHERE (team_id = sqlc.narg('team_id') OR contest_id = sqlc.narg('contest_id')) AND user_id IS NULL;

-- name: GetJoinLinkByToken :one
SELECT i.team_id, i.contest_id, o.id AS organization_id, o.login AS organization_login, o.name AS organization_name,
    t.name AS team_name, c.login AS contest_login, c.title AS contest_title
FROM invitations i
LEFT JOIN teams t ON t.id = i.team_id
LEFT JOIN contests c ON c.id = i.contest_id
JOIN organizations o ON o.id = COALESCE(t.organization_id, c.organization_id)
WHERE i.token = sqlc.arg('token')::text;

-- name: GetOrganizationRole :one
SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2;

-- name: GetContestRole :one
SELECT role::contest_role FROM contest_access WHERE contest_id = $1 AND user_id = $2;

-- name: JoinOrganization :exec
INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: JoinTeam :exec
INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'member')
ON CONFLICT DO NOTHING;

-- name: JoinContest :exec
INSERT INTO contest_members (contest_id, user_id, role) VALUES ($1, $2, 'participant')
ON CONFLICT DO NOTHING;
