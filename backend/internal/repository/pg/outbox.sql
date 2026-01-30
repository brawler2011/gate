-- name: InsertEvent :exec
INSERT INTO outbox_events (id, aggregate_id, event_type, payload)
VALUES (sqlc.arg(id)::uuid, sqlc.arg(aggregate_id)::uuid, sqlc.arg(event_type), sqlc.arg(payload));

-- name: PickEvents :many
UPDATE outbox_events
SET status = 'processing',
    locked_at = NOW(),
    deadline_at = NOW() + (sqlc.arg(timeout_sec)::int * INTERVAL '1 second')
WHERE id IN (
    SELECT id
    FROM outbox_events
    WHERE status = 'pending' 
       OR (status = 'processing' AND deadline_at < NOW())
    ORDER BY created_at ASC
    LIMIT sqlc.arg(limit_count)::int
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkAsCompleted :exec
UPDATE outbox_events
SET status       = 'completed',
    processed_at = NOW()
WHERE id = sqlc.arg(id)::uuid;

-- name: MarkAsFailed :exec
UPDATE outbox_events
SET status        = 'failed',
    retry_count   = retry_count + 1,
    processed_at  = NOW(),
    error_message = sqlc.arg(error_message)::text
WHERE id = sqlc.arg(id)::uuid;

-- name: ResetFailedToPending :exec
UPDATE outbox_events
SET status = 'pending'
WHERE status = 'failed' 
  AND retry_count < sqlc.arg(max_retries)::int 
  AND processed_at < NOW() - (sqlc.arg(retry_delay_sec)::int * INTERVAL '1 second');

-- name: DeleteOldEvents :exec
DELETE FROM outbox_events
WHERE status = sqlc.arg(status)
  AND created_at < NOW() - (sqlc.arg(retention_days)::int * INTERVAL '1 day');
