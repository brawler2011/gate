-- name: LockRateLimit :one
INSERT INTO rate_limits (key, expires_at)
VALUES (@key::text, NOW())
ON CONFLICT (key) DO UPDATE SET key = EXCLUDED.key
RETURNING hits, blocked_until;

-- name: SaveRateLimit :exec
UPDATE rate_limits
SET hits = @hits::timestamptz[], blocked_until = sqlc.narg(blocked_until)::timestamptz, expires_at = @expires_at::timestamptz
WHERE key = @key::text;

-- name: DeleteRateLimit :exec
DELETE FROM rate_limits
WHERE key = @key::text;

-- name: DeleteExpiredRateLimits :exec
DELETE FROM rate_limits
WHERE key IN (
    SELECT key FROM rate_limits
    WHERE expires_at < @now::timestamptz
    FOR UPDATE SKIP LOCKED
);
