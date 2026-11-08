-- name: InsertEvent :exec
WITH version AS (
    INSERT INTO event_aggregates (aggregate_id, version)
    VALUES (sqlc.arg(aggregate_id)::uuid, 1)
    ON CONFLICT (aggregate_id) DO UPDATE SET version = event_aggregates.version + 1
    RETURNING version
)
INSERT INTO outbox_events (id, aggregate_id, aggregate_version, event_type, payload, headers)
SELECT sqlc.arg(id)::uuid, sqlc.arg(aggregate_id)::uuid, version, sqlc.arg(event_type), sqlc.arg(payload), sqlc.arg(headers)
FROM version;

-- name: PickEvents :many
WITH picked AS (
    SELECT e.id FROM outbox_events e
    WHERE ((e.status = 'pending' AND e.next_attempt_at <= now())
       OR (e.status = 'processing' AND e.deadline_at < now()))
      AND NOT EXISTS (
          SELECT 1 FROM outbox_events earlier
          WHERE earlier.aggregate_id = e.aggregate_id
            AND earlier.aggregate_version < e.aggregate_version
            AND earlier.status <> 'completed'
      )
    ORDER BY e.created_at, e.id
    LIMIT sqlc.arg(limit_count)::int
    FOR UPDATE OF e SKIP LOCKED
)
UPDATE outbox_events e
SET status = 'processing', lease_token = sqlc.arg(lease_token)::uuid,
    locked_at = now(), deadline_at = now() + (sqlc.arg(timeout_sec)::int * INTERVAL '1 second')
FROM picked WHERE e.id = picked.id
RETURNING e.*;

-- name: MarkAsCompleted :execrows
UPDATE outbox_events
SET status = 'completed', processed_at = now(), lease_token = NULL, deadline_at = NULL
WHERE id = sqlc.arg(id)::uuid AND lease_token = sqlc.arg(lease_token)::uuid
  AND status = 'processing';

-- name: RenewOutboxLease :execrows
UPDATE outbox_events
SET deadline_at = now() + (sqlc.arg(timeout_sec)::int * INTERVAL '1 second')
WHERE id = sqlc.arg(id)::uuid AND lease_token = sqlc.arg(lease_token)::uuid
  AND status = 'processing' AND deadline_at > now();

-- name: MarkAsFailed :execrows
UPDATE outbox_events
SET status = CASE WHEN sqlc.arg(permanent)::boolean THEN 'failed'::outbox_event_status ELSE 'pending'::outbox_event_status END,
    retry_count = retry_count + 1, error_message = sqlc.arg(error_message)::text,
    next_attempt_at = now() + LEAST(300, power(2, LEAST(retry_count, 9))) * INTERVAL '1 second',
    processed_at = now(), lease_token = NULL, deadline_at = NULL
WHERE id = sqlc.arg(id)::uuid AND lease_token = sqlc.arg(lease_token)::uuid
  AND status = 'processing' AND deadline_at > now();

-- name: DeleteOldEvents :exec
DELETE FROM outbox_events
WHERE status = 'completed'
  AND processed_at < now() - (sqlc.arg(retention_days)::int * INTERVAL '1 day');

-- name: RecordInboxReceipt :one
INSERT INTO inbox_receipts (consumer_name, event_id)
VALUES (sqlc.arg(consumer_name), sqlc.arg(event_id)::uuid)
ON CONFLICT DO NOTHING RETURNING event_id;
