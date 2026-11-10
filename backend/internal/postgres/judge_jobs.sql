-- name: ScheduleJudgeJob :one
INSERT INTO judge_jobs(id,submission_id,generation,package_id,package_hash)
SELECT sqlc.arg(id)::uuid,s.id,s.generation,p.id,p.package_hash
FROM submissions s JOIN problem_packages p ON p.id=s.package_id
WHERE s.id=sqlc.arg(submission_id)::uuid
RETURNING *;

-- name: ClaimJudgeJob :one
UPDATE judge_jobs j
SET state='processing',attempt=j.attempt+1,lease_token=sqlc.arg(lease_token)::uuid,
    deadline_at=now()+interval '30 seconds'
FROM submissions s
WHERE j.id=sqlc.arg(id)::uuid AND j.generation=sqlc.arg(generation)::bigint
  AND s.id=sqlc.arg(submission_id)::uuid AND j.submission_id=s.id
  AND j.generation=s.generation AND s.state<>300
  AND (j.state='pending' OR (j.state='processing' AND j.deadline_at<now()))
RETURNING j.*;

-- name: JudgeJobActive :one
SELECT EXISTS (
    SELECT 1 FROM judge_jobs j JOIN submissions s ON s.id=j.submission_id
    WHERE j.id=sqlc.arg(id)::uuid AND j.generation=sqlc.arg(generation)::bigint
      AND s.id=sqlc.arg(submission_id)::uuid AND j.generation=s.generation AND s.state<>300
      AND j.state='processing' AND j.deadline_at>now()
) AS active;

-- name: RenewJudgeJob :execrows
UPDATE judge_jobs j SET deadline_at=now()+interval '30 seconds'
FROM submissions s
WHERE j.id=sqlc.arg(id)::uuid AND j.lease_token=sqlc.arg(lease_token)::uuid
  AND j.state='processing' AND j.deadline_at>now()
  AND s.id=j.submission_id AND s.generation=j.generation AND s.state<>300;

-- name: ReleaseJudgeJob :execrows
UPDATE judge_jobs SET state='pending',lease_token=NULL,deadline_at=NULL
WHERE id=sqlc.arg(id)::uuid AND lease_token=sqlc.arg(lease_token)::uuid AND state='processing' AND deadline_at>now();

-- name: CompleteJudgeJob :execrows
UPDATE judge_jobs j SET state='completed',completed_at=now(),lease_token=NULL,deadline_at=NULL
FROM submissions s
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

-- name: CompleteStalledJudgeJob :execrows
UPDATE judge_jobs
SET state = 'completed', completed_at = clock_timestamp(), lease_token = NULL, deadline_at = NULL
WHERE id = sqlc.arg(id)::uuid AND generation = sqlc.arg(generation)::bigint
  AND state = 'processing' AND deadline_at < clock_timestamp();

-- name: RequeueStalledJudgeJob :execrows
UPDATE judge_jobs
SET state = 'pending', lease_token = NULL, deadline_at = NULL
WHERE id = sqlc.arg(id)::uuid AND generation = sqlc.arg(generation)::bigint
  AND state = 'processing' AND deadline_at < clock_timestamp();
