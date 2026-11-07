-- name: AdvisoryTransactionLock :exec
SELECT pg_advisory_xact_lock(@key::bigint);
