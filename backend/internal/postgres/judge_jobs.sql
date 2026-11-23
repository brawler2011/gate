-- name: ScheduleJudgeJob :one
INSERT INTO judge_jobs(id,submission_id,generation,package_id,package_hash)
SELECT sqlc.arg(id)::uuid,s.id,s.generation,p.id,p.package_hash
FROM submissions s JOIN problem_packages p ON p.id=s.package_id
WHERE s.id=sqlc.arg(submission_id)::uuid
RETURNING *;

-- name: ClaimNextJudgeJob :one
UPDATE judge_jobs j
SET state='processing',attempt=j.attempt+1,lease_token=sqlc.arg(lease_token)::uuid,
    deadline_at=now()+interval '30 seconds'
WHERE j.id=(
    SELECT c.id FROM judge_jobs c JOIN submissions s ON s.id=c.submission_id
    WHERE c.state='pending' AND c.available_at<=now() AND c.generation=s.generation AND s.state<>300
    ORDER BY c.created_at
    LIMIT 1
    FOR UPDATE OF c SKIP LOCKED
)
RETURNING j.*;

-- name: RenewJudgeJob :execrows
UPDATE judge_jobs j SET deadline_at=now()+interval '30 seconds'
FROM submissions s
WHERE j.id=sqlc.arg(id)::uuid AND j.lease_token=sqlc.arg(lease_token)::uuid
  AND j.state='processing' AND j.deadline_at>now()
  AND s.id=j.submission_id AND s.generation=j.generation AND s.state<>300;

-- A released job waits so one that keeps failing cannot hold the head of the queue.
-- name: ReleaseJudgeJob :execrows
UPDATE judge_jobs SET state='pending',lease_token=NULL,deadline_at=NULL,available_at=now()+interval '5 seconds'
WHERE id=sqlc.arg(id)::uuid AND lease_token=sqlc.arg(lease_token)::uuid AND state='processing' AND deadline_at>now();

-- name: CompleteJudgeJob :execrows
DELETE FROM judge_jobs j
USING submissions s
WHERE j.id=sqlc.arg(id)::uuid AND j.lease_token=sqlc.arg(lease_token)::uuid
  AND j.submission_id=sqlc.arg(submission_id)::uuid AND j.generation=sqlc.arg(generation)::bigint
  AND j.state='processing' AND j.deadline_at>clock_timestamp()
  AND s.id=j.submission_id AND s.generation=j.generation AND s.state<>300;

-- name: PickStalledJudgeJobs :many
SELECT j.id, j.submission_id, j.generation, j.attempt
FROM judge_jobs j
WHERE j.state = 'processing' AND j.deadline_at < clock_timestamp()
ORDER BY j.deadline_at
LIMIT sqlc.arg(limit_count)::int;

-- name: DeleteStalledJudgeJob :execrows
DELETE FROM judge_jobs
WHERE id = sqlc.arg(id)::uuid AND generation = sqlc.arg(generation)::bigint
  AND state = 'processing' AND deadline_at < clock_timestamp();

-- name: RequeueStalledJudgeJob :execrows
UPDATE judge_jobs
SET state = 'pending', lease_token = NULL, deadline_at = NULL
WHERE id = sqlc.arg(id)::uuid AND generation = sqlc.arg(generation)::bigint
  AND state = 'processing' AND deadline_at < clock_timestamp();

-- name: DeleteObsoleteJudgeJobs :execrows
DELETE FROM judge_jobs j
WHERE j.state = 'pending' AND NOT EXISTS (
    SELECT 1 FROM submissions s
    WHERE s.id = j.submission_id AND s.generation = j.generation AND s.state <> 300
);
