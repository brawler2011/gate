-- name: GetAccessRevision :one
SELECT revision FROM access_revision;

-- name: GetCommandTime :one
SELECT clock_timestamp()::timestamptz AS now;

-- name: GetCommandActorExpiry :one
SELECT expires_at FROM users WHERE id = $1;
