-- name: CreateUser :exec
INSERT INTO users (id, username, role, kratos_id)
VALUES (@id::uuid, @username, @role, @kratos_id);

-- name: GetUserById :one
SELECT *
FROM users
WHERE id = @id::uuid
LIMIT 1;

-- name: GetUserByKratosId :one
SELECT *
FROM users
WHERE kratos_id = @kratos_id
LIMIT 1;

-- name: ListUsers :many
SELECT *
FROM users
WHERE (
        sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR word_similarity(username, sqlc.narg('search')) > 0.1
    )
    AND (
        sqlc.narg('role')::text IS NULL
        OR sqlc.narg('role') = ''
        OR role::text = sqlc.narg('role')
    )
ORDER BY CASE
        WHEN sqlc.narg('search')::text IS NOT NULL
        AND sqlc.narg('search') != '' THEN word_similarity(username, sqlc.narg('search'))
    END DESC NULLS LAST,
    created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountUsers :one
SELECT COUNT(*)
FROM users
WHERE (
        sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR word_similarity(username, sqlc.narg('search')) > 0.1
    )
    AND (
        sqlc.narg('role')::text IS NULL
        OR sqlc.narg('role') = ''
        OR role::text = sqlc.narg('role')
    );
