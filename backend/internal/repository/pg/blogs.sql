-- name: CreatePost :one
INSERT INTO posts (title, text, description, author_uuid, author_name, image_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: GetPostByID :one
SELECT id, created_at, updated_at, title, text, description, author_uuid, author_name, image_key
FROM posts
WHERE id = $1;

-- name: ListPostsDesc :many
SELECT id, created_at, updated_at, title, text, description, author_uuid, author_name, image_key
FROM posts
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListPostsAsc :many
SELECT id, created_at, updated_at, title, text, description, author_uuid, author_name, image_key
FROM posts
ORDER BY created_at ASC
LIMIT $1 OFFSET $2;

-- name: CountPosts :one
SELECT COUNT(*) FROM posts;

-- name: UpdatePost :exec
UPDATE posts
SET title = COALESCE($2, title),
    text = COALESCE($3, text),
    description = COALESCE($4, description),
    image_key = COALESCE($5, image_key),
    updated_at = NOW()
WHERE id = $1;

-- name: DeletePost :exec
DELETE FROM posts WHERE id = $1;
