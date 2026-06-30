-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, expires_at, created_at)
VALUES (@id::uuid, @user_id::uuid, @expires_at::timestamptz, NOW());

-- name: GetSession :one
SELECT id, user_id, expires_at, created_at
FROM sessions
WHERE id = @id::uuid
LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = @id::uuid;

-- name: UpdateSessionExpiry :exec
UPDATE sessions
SET expires_at = @expires_at::timestamptz
WHERE id = @id::uuid;

-- name: CleanupExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < NOW()
   OR created_at < @hard_limit_cutoff::timestamptz;

