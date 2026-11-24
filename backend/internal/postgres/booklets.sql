-- name: QueueContestBooklet :exec
INSERT INTO contest_booklets(contest_id,organization_id,language,source,source_sha256)
SELECT c.id,c.organization_id,sqlc.arg(language)::text,sqlc.arg(source)::text,sqlc.arg(source_sha256)::text FROM contests c WHERE c.id=sqlc.arg(contest_id)::uuid
ON CONFLICT (contest_id,language) DO UPDATE SET source=EXCLUDED.source,source_sha256=EXCLUDED.source_sha256,state='queued',attempt=0,
 lease_token=NULL,lease_deadline=NULL,object_id=NULL,diagnostic=NULL,updated_at=clock_timestamp()
WHERE contest_booklets.source_sha256<>EXCLUDED.source_sha256;

-- name: DeleteContestBookletsExcept :exec
DELETE FROM contest_booklets WHERE contest_id=sqlc.arg(contest_id)::uuid AND NOT language=ANY(sqlc.arg(languages)::text[]);

-- name: ListContestBooklets :many
SELECT b.language,b.state,b.source_sha256,b.diagnostic,b.updated_at,o.key AS object_key
FROM contest_booklets b LEFT JOIN stored_objects o ON o.id=b.object_id
WHERE b.contest_id=$1 ORDER BY b.language;

-- name: ClaimContestBooklet :one
-- A running booklet whose lease expired is claimed again.
UPDATE contest_booklets b SET state='running',attempt=b.attempt+1,lease_token=sqlc.arg(token)::uuid,
 lease_deadline=clock_timestamp()+sqlc.arg(lease_ms)::bigint*interval '1 millisecond'
FROM (SELECT contest_id,language FROM contest_booklets
 WHERE state='queued' OR (state='running' AND lease_deadline<=clock_timestamp())
 ORDER BY updated_at LIMIT 1 FOR UPDATE SKIP LOCKED) n
WHERE b.contest_id=n.contest_id AND b.language=n.language
RETURNING b.contest_id,b.organization_id,b.language,b.source,b.attempt;

-- name: CompleteContestBooklet :execrows
UPDATE contest_booklets SET state='succeeded',object_id=sqlc.arg(object_id)::uuid,lease_token=NULL,lease_deadline=NULL,updated_at=clock_timestamp()
WHERE contest_id=sqlc.arg(contest_id)::uuid AND language=sqlc.arg(language)::text AND state='running' AND lease_token=sqlc.arg(token)::uuid;

-- name: FailContestBooklet :execrows
UPDATE contest_booklets SET state='failed',diagnostic=sqlc.arg(diagnostic)::text,lease_token=NULL,lease_deadline=NULL,updated_at=clock_timestamp()
WHERE contest_id=sqlc.arg(contest_id)::uuid AND language=sqlc.arg(language)::text AND state='running' AND lease_token=sqlc.arg(token)::uuid;

-- name: RequeueContestBooklet :execrows
UPDATE contest_booklets SET state='queued',lease_token=NULL,lease_deadline=NULL,updated_at=clock_timestamp()
WHERE contest_id=sqlc.arg(contest_id)::uuid AND language=sqlc.arg(language)::text AND state='running' AND lease_token=sqlc.arg(token)::uuid;
