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
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: ListPostsAsc :many
SELECT id, created_at, updated_at, title, text, description, author_uuid, author_name, image_key
FROM posts
ORDER BY created_at ASC, id ASC
LIMIT $1 OFFSET $2;

-- name: CountPosts :one
SELECT COUNT(*) FROM posts;

-- name: UpdatePost :execrows
UPDATE posts
SET title = COALESCE(sqlc.narg('title'), title),
    text = COALESCE(sqlc.narg('text'), text),
    description = COALESCE(sqlc.narg('description'), description),
    image_key = COALESCE(sqlc.narg('image_key'), image_key),
    updated_at = NOW()
WHERE id = $1;

-- name: DeletePost :execrows
DELETE FROM posts WHERE id = $1;

-- name: LockPost :one
SELECT id FROM posts WHERE id=$1 FOR UPDATE;
