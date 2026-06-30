-- name: CreateUser :exec
INSERT INTO users (
        id,
        username,
        role,
        password_hash,
        email,
        avatar_url
    )
VALUES (
        @id::uuid,
        @username,
        @role,
        @password_hash,
        @email,
        @avatar_url
    );

-- name: GetUserById :one
SELECT *
FROM users
WHERE id = @id::uuid
LIMIT 1;

-- name: GetUserByUsernameOrEmail :one
SELECT *
FROM users
WHERE username = @identifier OR email = @identifier
LIMIT 1;

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = @username
LIMIT 1;

-- name: ListUsers :many
SELECT *
FROM users
WHERE (
        @search::text = ''
        OR word_similarity(username, @search) > 0.1
    )
    AND (
        sqlc.arg('role')::text = ''
        OR role::text = sqlc.arg('role')
    )
ORDER BY CASE
        WHEN @search != '' THEN word_similarity(username, @search)
    END DESC NULLS LAST,
    created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountUsers :one
SELECT COUNT(*)::int4
FROM users
WHERE (
        @search::text = ''
        OR word_similarity(username, @search) > 0.1
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
    avatar_url = COALESCE(sqlc.narg(avatar_url), avatar_url)
WHERE id = @id::uuid;
