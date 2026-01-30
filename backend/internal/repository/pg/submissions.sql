-- name: CreateSubmission :one
INSERT INTO submissions (
    contest_id,
    problem_id,
    owner_id,
    source,
    language,
    penalty
  )
VALUES (
    @contest_id::uuid,
    @problem_id::uuid,
    @owner_id::uuid,
    @source,
    @language,
    @penalty
  )
RETURNING id;

-- name: GetSubmission :one
SELECT s.id,
  s.owner_id,
  u.username,
  s.source,
  s.state,
  s.score,
  s.penalty,
  s.time_stat,
  s.memory_stat,
  s.language,
  s.problem_id,
  p.titles AS problem_titles,
  p.short_name AS problem_short_name,
  cp.ordinal AS problem_ordinal,
  s.contest_id,
  c.titles AS contest_titles,
  c.short_name AS contest_short_name,
  c.visibility AS contest_visibility,
  s.updated_at,
  s.created_at
FROM submissions s
  LEFT JOIN users u ON s.owner_id = u.id
  LEFT JOIN problems p ON s.problem_id = p.id
  LEFT JOIN contest_problems cp ON p.id = cp.problem_id
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
  s.owner_id,
  u.username,
  s.state,
  s.score,
  s.penalty,
  s.time_stat,
  s.memory_stat,
  s.language,
  s.problem_id,
  p.titles AS problem_titles,
  p.short_name AS problem_short_name,
  cp.ordinal AS problem_ordinal,
  s.contest_id,
  c.titles AS contest_titles,
  c.short_name AS contest_short_name,
  s.updated_at,
  s.created_at
FROM submissions s
  LEFT JOIN users u ON s.owner_id = u.id
  LEFT JOIN problems p ON s.problem_id = p.id
  LEFT JOIN contest_problems cp ON p.id = cp.problem_id
  AND cp.contest_id = s.contest_id
  LEFT JOIN contests c ON s.contest_id = c.id
WHERE (
    sqlc.narg('contest_id')::uuid IS NULL
    OR s.contest_id = sqlc.narg('contest_id')::uuid
  )
  AND (
    sqlc.narg('owner_id')::uuid IS NULL
    OR s.owner_id = sqlc.narg('owner_id')::uuid
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
    sqlc.narg('owner_id')::uuid IS NULL
    OR s.owner_id = sqlc.narg('owner_id')::uuid
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
