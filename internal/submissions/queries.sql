-- Submission CRUD operations

-- name: CreateSubmission :one
INSERT INTO submissions (
        contest_id,
        problem_id,
        created_by,
        submission,
        language,
        penalty
    )
VALUES (@contest_id::uuid, @problem_id::uuid, @created_by::uuid, @submission, @language, @penalty)
RETURNING id;

-- name: GetSubmission :one
SELECT s.id,
    s.created_by,
    u.username,
    s.submission,
    s.state,
    s.score,
    s.penalty,
    s.time_stat,
    s.memory_stat,
    s.language,
    s.problem_id,
    p.title AS problem_title,
    cp.position,
    s.contest_id,
    c.title AS contest_title,
    s.updated_at,
    s.created_at
FROM submissions s
    LEFT JOIN users u ON s.created_by = u.id
    LEFT JOIN problems p ON s.problem_id = p.id
    LEFT JOIN contest_problem cp ON p.id = cp.problem_id
    AND cp.contest_id = s.contest_id
    LEFT JOIN contests c ON s.contest_id = c.id
WHERE s.id = @id::uuid;

-- name: UpdateSubmission :exec
UPDATE submissions
SET state = @state,
    score = @score,
    time_stat = @time_stat,
    memory_stat = @memory_stat
WHERE id = @id::uuid;

-- Submission listing

-- name: ListSubmissions :many
SELECT s.id,
  s.created_by,
  u.username,
  s.state,
  s.score,
  s.penalty,
  s.time_stat,
  s.memory_stat,
  s.language,
  s.problem_id,
  p.title AS problem_title,
  cp.position,
  s.contest_id,
  c.title AS contest_title,
  s.updated_at,
  s.created_at
FROM submissions s
  LEFT JOIN users u ON s.created_by = u.id
  LEFT JOIN problems p ON s.problem_id = p.id
  LEFT JOIN contest_problem cp ON p.id = cp.problem_id
  AND cp.contest_id = s.contest_id
  LEFT JOIN contests c ON s.contest_id = c.id
WHERE (
    sqlc.narg('contest_id')::uuid IS NULL
    OR s.contest_id = sqlc.narg('contest_id')::uuid
  )
  AND (
    sqlc.narg('created_by')::uuid IS NULL
    OR s.created_by = sqlc.narg('created_by')::uuid
  )
  AND (
    sqlc.narg('problem_id')::uuid IS NULL
    OR s.problem_id = sqlc.narg('problem_id')::uuid
  )
  AND (
    sqlc.narg('language')::integer IS NULL
    OR s.language = sqlc.narg('language')::integer
  )
  AND (
    sqlc.narg('state')::integer IS NULL
    OR s.state = sqlc.narg('state')::integer
  )
ORDER BY CASE
    WHEN sqlc.narg('sort_order')::int < 0 THEN s.created_at
  END DESC,
  CASE
    WHEN sqlc.narg('sort_order')::int >= 0 THEN s.created_at
  END ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountSubmissions :one
SELECT COUNT(*)
FROM submissions s
WHERE (
        sqlc.narg('contest_id')::uuid IS NULL
        OR s.contest_id = sqlc.narg('contest_id')::uuid
    )
    AND (
        sqlc.narg('created_by')::uuid IS NULL
        OR s.created_by = sqlc.narg('created_by')::uuid
    )
    AND (
        sqlc.narg('problem_id')::uuid IS NULL
        OR s.problem_id = sqlc.narg('problem_id')::uuid
    )
    AND (
        sqlc.narg('language')::integer IS NULL
        OR s.language = sqlc.narg('language')::integer
    )
    AND (
        sqlc.narg('state')::integer IS NULL
        OR s.state = sqlc.narg('state')::integer
    );

-- name: GetUntestedSubmissions :many
SELECT s.id,
    s.contest_id,
    s.problem_id,
    s.created_by,
    s.language
FROM submissions s
WHERE s.state = 1 -- Saved state
    AND s.created_at < now() - interval '1 minute' -- Only submissions older than 1 minute
ORDER BY s.created_at ASC
LIMIT sqlc.arg('limit');