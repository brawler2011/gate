-- name: PruneInboxReceipts :execrows
DELETE FROM inbox_receipts
WHERE processed_at < @cutoff::timestamptz;
