-- name: CreateDraft :one
INSERT INTO contest_drafts (
    contest_id,
    user_id,
    code
) VALUES (
    @contest_id::uuid,
    @user_id::uuid,
    @code
)
RETURNING id;

-- name: GetDraft :one
SELECT cd.id,
    cd.contest_id,
    cd.user_id,
    u.username,
    cd.code,
    cd.created_at,
    cd.updated_at
FROM contest_drafts cd
    LEFT JOIN users u ON cd.user_id = u.id
WHERE cd.id = @id::uuid;

-- name: GetDraftsCount :one
SELECT COUNT(*)
FROM contest_drafts
WHERE contest_id = @contest_id::uuid
  AND user_id = @user_id::uuid;

-- name: ListDrafts :many
SELECT cd.id,
    cd.contest_id,
    cd.user_id,
    u.username,
    cd.code,
    cd.created_at,
    cd.updated_at
FROM contest_drafts cd
    LEFT JOIN users u ON cd.user_id = u.id
WHERE cd.contest_id = @contest_id::uuid
  AND cd.user_id = @user_id::uuid
ORDER BY cd.created_at DESC
LIMIT @limit_val OFFSET @offset_val;

-- name: CountDrafts :one
SELECT COUNT(*)
FROM contest_drafts cd
WHERE cd.contest_id = @contest_id::uuid
  AND cd.user_id = @user_id::uuid;

-- name: DeleteDraft :exec
DELETE FROM contest_drafts
WHERE id = @id::uuid;

