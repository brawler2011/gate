-- name: ScheduleJudgeJob :one
INSERT INTO judge_jobs (id,submission_id,generation,package_id,package_hash)
SELECT sqlc.arg(id)::uuid,s.id,s.generation,
       CASE WHEN pinned.package_hash <> '' THEN pinned.id ELSE ready.id END,
       COALESCE(NULLIF(pinned.package_hash,''),ready.package_hash,'')
FROM submissions s
LEFT JOIN contest_problems cp ON cp.contest_id=s.contest_id AND cp.problem_id=s.problem_id
LEFT JOIN problem_packages pinned ON pinned.id=cp.package_id
LEFT JOIN LATERAL (
    SELECT p.id,p.package_hash FROM problem_packages p
    WHERE p.problem_id=s.problem_id AND p.status='ready'
    ORDER BY p.created_at DESC LIMIT 1
) ready ON true
WHERE s.id=sqlc.arg(submission_id)::uuid
RETURNING *;

-- name: ClaimJudgeJob :one
UPDATE judge_jobs j
SET state='processing',attempt=j.attempt+1,lease_token=sqlc.arg(lease_token)::uuid,
    deadline_at=now()+interval '30 seconds'
FROM submissions s
WHERE s.id=sqlc.arg(submission_id)::uuid AND j.submission_id=s.id
  AND j.generation=s.generation AND s.state<>300
  AND (j.state='pending' OR (j.state='processing' AND j.deadline_at<now()))
RETURNING j.*;

-- name: JudgeJobActive :one
SELECT EXISTS (
    SELECT 1 FROM judge_jobs j JOIN submissions s ON s.id=j.submission_id
    WHERE s.id=sqlc.arg(submission_id)::uuid AND j.generation=s.generation AND s.state<>300
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

-- name: LockJudgeContest :one
SELECT c.id FROM contests c JOIN submissions s ON s.contest_id=c.id
WHERE s.id=sqlc.arg(submission_id)::uuid FOR UPDATE OF c;

-- name: CompleteJudgeJob :execrows
UPDATE judge_jobs j SET state='completed',completed_at=now(),lease_token=NULL,deadline_at=NULL
FROM submissions s
WHERE j.id=sqlc.arg(id)::uuid AND j.lease_token=sqlc.arg(lease_token)::uuid
  AND j.state='processing' AND j.deadline_at>now()
  AND s.id=j.submission_id AND s.generation=j.generation AND s.state<>300;
