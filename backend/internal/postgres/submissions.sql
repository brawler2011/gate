-- name: CreateSubmission :one
INSERT INTO submissions (
    contest_id,
    package_id,
    problem_id,
    owner_id,
    kind,
    source,
    language,
    penalty,
    state,
    ban_reason,
    outputs,
    revision
  )
VALUES (
    @contest_id::uuid,
    (SELECT cp.package_id FROM contest_problems cp WHERE cp.contest_id = @contest_id::uuid AND cp.problem_id = @problem_id::uuid),
    @problem_id::uuid,
    @owner_id::uuid,
    @kind::submission_kind,
    @source,
    @language,
    @penalty,
    COALESCE(sqlc.narg('state')::integer, 1),
    sqlc.narg('ban_reason'),
    sqlc.narg('outputs'),
    1
  )
RETURNING id;

-- name: GetSubmission :one
SELECT s.id,
  s.owner_id,
  s.kind,
  u.username,
  s.source,
  s.state,
  s.score,
  s.penalty,
  COALESCE((s.test_details->>'time_stat')::integer, 0)::integer AS time_stat,
  COALESCE((s.test_details->>'memory_stat')::integer, 0)::integer AS memory_stat,
  s.language,
  COALESCE((s.test_details->>'failed_test')::integer, 0)::integer AS failed_test,
  s.test_details,
  s.ban_reason,
  s.problem_id,
  pp.title AS problem_title,
  pp.short_name AS problem_short_name,
  cp.ordinal AS problem_ordinal,
  s.contest_id,
  c.title AS contest_title,
  c.login AS contest_login,
  c.visibility AS contest_visibility,
  COALESCE(oc.login, op.login, '')::text AS organization_login,
  s.updated_at,
  s.revision,
  s.created_at,
  s.pretests,
  s.outputs
FROM submissions s
  LEFT JOIN users u ON s.owner_id = u.id
  LEFT JOIN problem_catalog p ON s.problem_id = p.id
 LEFT JOIN problem_packages pp ON pp.id=s.package_id
  LEFT JOIN organizations op ON p.organization_id = op.id
  LEFT JOIN contest_problems cp ON p.id = cp.problem_id
  AND cp.contest_id = s.contest_id
  LEFT JOIN contests c ON s.contest_id = c.id
  LEFT JOIN organizations oc ON c.organization_id = oc.id
WHERE s.id = @id::uuid;

-- name: UpdateSubmission :one
WITH previous AS (
  SELECT id, state FROM submissions WHERE id = @id::uuid FOR UPDATE
)
UPDATE submissions s
SET state = @state, score = @score, test_details = @test_details, pretests = @pretests,
    judge_k = sqlc.narg(judge_k), judge_benchmark = sqlc.narg(judge_benchmark),
    updated_at = NOW(), revision = s.revision + 1
FROM previous WHERE s.id = previous.id
RETURNING s.id, previous.state AS previous_state, s.revision;

-- name: BlockSubmission :exec
UPDATE submissions
SET state = 300, score = 0, ban_reason = @ban_reason,
    updated_at = NOW(), revision = revision + 1
WHERE id = @id::uuid;

-- name: BlockUserProblemSubmissions :many
WITH previous AS (
  SELECT id, state FROM submissions
  WHERE id = ANY(@ids::uuid[]) ORDER BY id FOR UPDATE
)
UPDATE submissions s
SET state = 300, score = 0, ban_reason = @ban_reason,
    updated_at = NOW(), revision = s.revision + 1
FROM previous WHERE s.id = previous.id
RETURNING s.id, previous.state AS previous_state, s.revision;

-- name: ListSubmissions :many
SELECT s.id,
  s.owner_id,
  s.kind,
  u.username,
  s.state,
  s.score,
  s.penalty,
  COALESCE((s.test_details->>'time_stat')::integer, 0)::integer AS time_stat,
  COALESCE((s.test_details->>'memory_stat')::integer, 0)::integer AS memory_stat,
  s.language,
  COALESCE((s.test_details->>'failed_test')::integer, 0)::integer AS failed_test,
  COALESCE((
    SELECT g->>'feedback'
    FROM jsonb_array_elements(s.test_details->'tests') t
      JOIN jsonb_array_elements(s.test_details->'groups') g ON g->>'name' = COALESCE(t->>'group', '')
    WHERE (t->>'test_index')::integer = (s.test_details->>'failed_test')::integer
    LIMIT 1
  ), '')::text AS failed_test_feedback,
  s.ban_reason,
  s.problem_id,
  pp.title AS problem_title,
  pp.short_name AS problem_short_name,
  cp.ordinal AS problem_ordinal,
  s.contest_id,
  c.title AS contest_title,
  c.login AS contest_login,
  COALESCE(oc.login, op.login, '')::text AS organization_login,
  s.updated_at,
  s.revision,
  s.created_at,
  s.pretests
FROM submissions s
  LEFT JOIN users u ON s.owner_id = u.id
  LEFT JOIN problem_catalog p ON s.problem_id = p.id
 LEFT JOIN problem_packages pp ON pp.id=s.package_id
  LEFT JOIN organizations op ON p.organization_id = op.id
  LEFT JOIN contest_problems cp ON p.id = cp.problem_id
  AND cp.contest_id = s.contest_id
  LEFT JOIN contests c ON s.contest_id = c.id
  LEFT JOIN organizations oc ON c.organization_id = oc.id
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
    sqlc.narg('language')::text IS NULL
    OR s.language = sqlc.narg('language')::text
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
  END ASC,
  CASE WHEN sqlc.narg('sort_order')::int < 0 THEN s.id END DESC,
  CASE WHEN sqlc.narg('sort_order')::int >= 0 THEN s.id END ASC
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
    sqlc.narg('language')::text IS NULL
    OR s.language = sqlc.narg('language')::text
  )
  AND (
    sqlc.narg('state')::integer IS NULL
    OR s.state = sqlc.narg('state')::integer
  );

-- name: SubmissionCandidates :many
SELECT id FROM submissions
WHERE contest_id = @contest_id::uuid
  AND (sqlc.narg('problem_id')::uuid IS NULL OR problem_id = sqlc.narg('problem_id')::uuid)
  AND (sqlc.narg('submission_id')::uuid IS NULL OR id = sqlc.narg('submission_id')::uuid)
  AND (sqlc.narg('owner_id')::uuid IS NULL OR owner_id = sqlc.narg('owner_id')::uuid)
ORDER BY id;

-- name: ResetFinishedPretests :many
-- FOR UPDATE rechecks the conditions after a concurrent block or tick commits.
WITH candidates AS (
  SELECT s.id, s.state FROM submissions s JOIN contests c ON c.id = s.contest_id
  WHERE s.pretests AND s.state <> 300 AND c.end_time <= now()
  ORDER BY s.id FOR UPDATE OF s
)
UPDATE submissions s
SET state = 1, score = 0, test_details = NULL,
    ban_reason = NULL, updated_at = NOW(), revision = s.revision + 1
FROM candidates WHERE s.id = candidates.id
RETURNING s.id, candidates.state AS previous_state, s.revision;

-- name: LockSubmission :one
SELECT id, revision, state FROM submissions WHERE id = @id::uuid FOR UPDATE;

-- name: ResetSubmissionsState :many
WITH previous AS (
  SELECT id, state FROM submissions
  WHERE id = ANY(@ids::uuid[]) ORDER BY id FOR UPDATE
)
UPDATE submissions s
SET state = 1, score = 0, test_details = NULL,
    ban_reason = NULL, updated_at = NOW(), revision = s.revision + 1
FROM previous WHERE s.id = previous.id
RETURNING s.id, previous.state AS previous_state, s.revision;
