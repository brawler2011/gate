-- Outbox event operations

-- name: InsertEvent :one
INSERT INTO outbox_events (aggregate_id, aggregate_type, event_type, payload, status, retry_count)
VALUES (@aggregate_id::uuid, @aggregate_type, @event_type, @payload, @status, @retry_count)
RETURNING id, created_at;

-- name: GetPendingEvents :many
SELECT id, aggregate_id, aggregate_type, event_type, payload, status, 
       created_at, processed_at, retry_count, error_message
FROM outbox_events
WHERE status = @status
ORDER BY created_at ASC
LIMIT sqlc.arg('limit');

-- name: MarkAsProcessing :exec
UPDATE outbox_events
SET status = @status
WHERE id = @id::uuid;

-- name: MarkAsCompleted :exec
UPDATE outbox_events
SET status = @status, processed_at = @processed_at
WHERE id = @id::uuid;

-- name: MarkAsFailed :exec
UPDATE outbox_events
SET status = @status, retry_count = retry_count + 1, error_message = @error_message
WHERE id = @id::uuid;

-- name: ResetFailedToPending :exec
UPDATE outbox_events
SET status = @status
WHERE status = @status_old AND retry_count < @max_retries;

-- name: DeleteOldEvents :exec
DELETE FROM outbox_events
WHERE status = @status AND processed_at < @before_date;
