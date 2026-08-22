-- name: CreateUser :exec
INSERT INTO users (
        id,
        username,
        role,
        password_hash,
        email,
        avatar_url,
        expires_at,
        is_email_verified
    )
VALUES (
        @id::uuid,
        @username,
        @role,
        @password_hash,
        @email,
        @avatar_url,
        @expires_at,
        @is_email_verified
    );

-- name: GetUserById :one
SELECT *
FROM users
WHERE id = @id::uuid
LIMIT 1;

-- name: GetUserByUsernameOrEmail :one
SELECT *
FROM users
WHERE LOWER(username) = LOWER(@identifier) OR (email IS NOT NULL AND LOWER(email) = LOWER(@identifier))
LIMIT 1;

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE LOWER(username) = LOWER(@username)
LIMIT 1;

-- name: ListUsers :many
SELECT *
FROM users
WHERE (
        @search::text = ''
        OR word_similarity(LOWER(username), LOWER(@search)) > 0.1
    )
    AND (
        sqlc.arg('role')::text = ''
        OR role::text = sqlc.arg('role')
    )
ORDER BY CASE
        WHEN @search != '' THEN word_similarity(LOWER(username), LOWER(@search))
    END DESC NULLS LAST,
    created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountUsers :one
SELECT COUNT(*)::int4
FROM users
WHERE (
        @search::text = ''
        OR word_similarity(LOWER(username), LOWER(@search)) > 0.1
    )
    AND (
        sqlc.arg('role')::text = ''
        OR role::text = sqlc.arg('role')
    );

-- name: UpdateUser :exec
UPDATE users
SET username = COALESCE(sqlc.narg(username), username),
    role = COALESCE(sqlc.narg(role), role),
    email = COALESCE(sqlc.narg(email), email),
    avatar_url = COALESCE(sqlc.narg(avatar_url), avatar_url),
    expires_at = COALESCE(sqlc.narg(expires_at), expires_at),
    is_email_verified = COALESCE(sqlc.narg(is_email_verified), is_email_verified)
WHERE id = @id::uuid;

-- name: SetUserEmailVerified :exec
UPDATE users
SET is_email_verified = @is_email_verified::boolean
WHERE id = @id::uuid;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = @password_hash
WHERE id = @id::uuid;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = @email,
    is_email_verified = @is_email_verified::boolean
WHERE id = @id::uuid;

-- name: ClaimTemporaryUser :exec
UPDATE users
SET claimed_by_user_id = @claimed_by_user_id::uuid,
    claimed_at = @claimed_at::timestamptz
WHERE id = @id::uuid;

-- name: ListClaimedAccountsByUserId :many
SELECT *
FROM users
WHERE claimed_by_user_id = @claimed_by_user_id::uuid
ORDER BY claimed_at DESC;

-- name: ListExistingUsernamesByPrefix :many
SELECT username
FROM users
WHERE username ILIKE @prefix::text || '%';



