-- name: CreatePost :one
INSERT INTO posts (title, text, description, author_id, image_key)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: GetPostByID :one
SELECT p.id, p.created_at, p.updated_at, p.title, p.text, p.description, p.author_id, u.username AS author_name, p.image_key
FROM posts p JOIN users u ON u.id = p.author_id
WHERE p.id = $1;

-- name: ListPostsDesc :many
SELECT p.id, p.created_at, p.updated_at, p.title, p.text, p.description, p.author_id, u.username AS author_name, p.image_key
FROM posts p JOIN users u ON u.id = p.author_id
ORDER BY p.created_at DESC, p.id DESC
LIMIT $1 OFFSET $2;

-- name: ListPostsAsc :many
SELECT p.id, p.created_at, p.updated_at, p.title, p.text, p.description, p.author_id, u.username AS author_name, p.image_key
FROM posts p JOIN users u ON u.id = p.author_id
ORDER BY p.created_at ASC, p.id ASC
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
