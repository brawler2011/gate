-- name: ListBookletInputs :many
-- A row per contest and statement language of its problems, with the stored booklet; null filters match every contest.
SELECT c.id AS contest_id, c.organization_id, l.language, c.title, c.place, o.name AS organization_name, o.time_zone, c.start_time,
  ps.problems::jsonb AS problems,
  b.key AS stored_key, b.state AS stored_state, b.diagnostic AS stored_diagnostic, b.updated_at AS stored_updated_at, so.key AS object_key
FROM contests c
JOIN organizations o ON o.id = c.organization_id
CROSS JOIN LATERAL (
  SELECT jsonb_agg(jsonb_build_object('ordinal', cp.ordinal, 'problem_id', cp.problem_id, 'package_id', cp.package_id,
    'title', pp.title, 'time_limit_ms', pp.time_limit_ms, 'memory_limit_mb', pp.memory_limit_mb, 'input_file', pp.input_file, 'output_file', pp.output_file) ORDER BY cp.ordinal) AS problems
  FROM contest_problems cp JOIN problem_packages pp ON pp.id = cp.package_id WHERE cp.contest_id = c.id
) ps
JOIN LATERAL (
  SELECT DISTINCT (s->>'language')::text AS language
  FROM contest_problems cp JOIN problem_packages pp ON pp.id = cp.package_id
  CROSS JOIN LATERAL jsonb_array_elements(pp.statements) s
  WHERE cp.contest_id = c.id
) l ON TRUE
LEFT JOIN contest_booklets b ON b.contest_id = c.id AND b.language = l.language
LEFT JOIN stored_objects so ON so.id = b.object_id
WHERE (sqlc.narg(contest_id)::uuid IS NULL OR c.id = sqlc.narg(contest_id)::uuid)
  AND (sqlc.narg(organization_id)::uuid IS NULL OR c.organization_id = sqlc.narg(organization_id)::uuid)
ORDER BY c.id, l.language;

-- name: QueueBooklet :execrows
INSERT INTO contest_booklets (contest_id, organization_id, language, key)
VALUES (sqlc.arg(contest_id)::uuid, sqlc.arg(organization_id)::uuid, sqlc.arg(language)::text, sqlc.arg(key)::text)
ON CONFLICT (contest_id, language) DO UPDATE
SET key = EXCLUDED.key, state = 'queued', attempt = 0, lease_token = NULL, lease_deadline = NULL, object_id = NULL, diagnostic = NULL, updated_at = clock_timestamp()
WHERE contest_booklets.key <> EXCLUDED.key;

-- name: DeleteOtherBooklets :exec
DELETE FROM contest_booklets b
USING contests c
WHERE c.id = b.contest_id
  AND (sqlc.narg(contest_id)::uuid IS NULL OR c.id = sqlc.narg(contest_id)::uuid)
  AND (sqlc.narg(organization_id)::uuid IS NULL OR c.organization_id = sqlc.narg(organization_id)::uuid)
  AND NOT (b.contest_id::text || '/' || b.language = ANY(COALESCE(sqlc.arg(keep)::text[], '{}')));

-- name: ClaimBooklet :one
UPDATE contest_booklets b
SET state = 'running', attempt = b.attempt + 1, lease_token = sqlc.arg(lease_token)::uuid,
    lease_deadline = clock_timestamp() + make_interval(secs => sqlc.arg(lease_seconds)::float8), updated_at = clock_timestamp()
WHERE (b.contest_id, b.language) = (
  SELECT q.contest_id, q.language FROM contest_booklets q
  WHERE q.state = 'queued' OR (q.state = 'running' AND q.lease_deadline < clock_timestamp())
  ORDER BY q.updated_at
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING b.contest_id, b.organization_id, b.language, b.key, b.attempt;

-- name: CompleteBooklet :execrows
UPDATE contest_booklets
SET state = 'succeeded', object_id = sqlc.arg(object_id)::uuid, lease_token = NULL, lease_deadline = NULL, updated_at = clock_timestamp()
WHERE contest_id = sqlc.arg(contest_id)::uuid AND language = sqlc.arg(language)::text AND key = sqlc.arg(key)::text
  AND state = 'running' AND lease_token = sqlc.arg(lease_token)::uuid;

-- name: FailBooklet :execrows
UPDATE contest_booklets
SET state = 'failed', diagnostic = sqlc.arg(diagnostic)::text, lease_token = NULL, lease_deadline = NULL, updated_at = clock_timestamp()
WHERE contest_id = sqlc.arg(contest_id)::uuid AND language = sqlc.arg(language)::text AND key = sqlc.arg(key)::text
  AND state = 'running' AND lease_token = sqlc.arg(lease_token)::uuid;

-- name: RequeueBooklet :execrows
UPDATE contest_booklets
SET state = 'queued', lease_token = NULL, lease_deadline = NULL, updated_at = clock_timestamp()
WHERE contest_id = sqlc.arg(contest_id)::uuid AND language = sqlc.arg(language)::text AND key = sqlc.arg(key)::text
  AND state = 'running' AND lease_token = sqlc.arg(lease_token)::uuid;
