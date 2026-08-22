-- Notifications queries

-- name: CreateNotification :one
INSERT INTO notifications (
    id,
    user_id,
    type,
    title,
    body,
    link,
    data,
    is_read,
    created_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    NOW()
) RETURNING *;

-- name: ListNotificationsByUserID :many
SELECT * FROM notifications
WHERE user_id = $1
  AND (sqlc.narg('unread_only')::boolean IS NOT TRUE OR is_read = FALSE)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountNotificationsByUserID :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1
  AND (sqlc.narg('unread_only')::boolean IS NOT TRUE OR is_read = FALSE);

-- name: GetUnreadNotificationsCount :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1 AND is_read = FALSE;

-- name: GetNotificationByID :one
SELECT * FROM notifications
WHERE id = $1 AND user_id = $2;

-- name: MarkNotificationAsRead :exec
UPDATE notifications
SET is_read = TRUE
WHERE id = $1 AND user_id = $2;

-- name: MarkAllNotificationsAsRead :exec
UPDATE notifications
SET is_read = TRUE
WHERE user_id = $1 AND is_read = FALSE;
