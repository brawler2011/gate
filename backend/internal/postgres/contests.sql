-- Contests queries (new schema with Organizations)

-- name: CreateContest :one
INSERT INTO contests (id, organization_id, owner_id, visibility, title, login, description, settings, start_time, end_time)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetContestByID :one
SELECT c.*, o.login as org_login, o.name as org_name
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE c.id = $1;

-- name: GetContestByLogin :one
SELECT c.*, o.login as org_login, o.name as org_name
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE c.organization_id = $1 AND LOWER(c.login) = LOWER($2);

-- name: GetContestByOrgLoginAndContestLogin :one
SELECT c.*, o.login as org_login, o.name as org_name
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE LOWER(o.login) = LOWER($1) AND LOWER(c.login) = LOWER($2);

-- name: ListContests :many
SELECT c.*, o.login as org_login, o.name as org_name
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE c.organization_id = sqlc.arg('organization_id')::uuid
  AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (c.visibility = 'public'
    OR EXISTS (SELECT 1 FROM users u WHERE u.id = sqlc.arg('viewer_id')::uuid AND u.role = 'admin')
    OR EXISTS (SELECT 1 FROM organization_members om
               WHERE om.organization_id = c.organization_id AND om.user_id = sqlc.arg('viewer_id')::uuid))
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: CountContests :one
SELECT COUNT(*) FROM contests c
WHERE c.organization_id = sqlc.arg('organization_id')::uuid
  AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (c.visibility = 'public'
    OR EXISTS (SELECT 1 FROM users u WHERE u.id = sqlc.arg('viewer_id')::uuid AND u.role = 'admin')
    OR EXISTS (SELECT 1 FROM organization_members om
               WHERE om.organization_id = c.organization_id AND om.user_id = sqlc.arg('viewer_id')::uuid));

-- name: ListAllContests :many
SELECT c.*, o.login as org_login, o.name as org_name
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('visibility')::text = '' OR c.visibility = sqlc.arg('visibility')::contest_visibility)
ORDER BY
  CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text = 'asc' THEN c.created_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text <> 'asc' THEN c.created_at END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text = 'asc' THEN c.updated_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.updated_at END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text = 'asc' THEN c.title END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.title END DESC,
  c.id DESC
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: CountAllContests :one
SELECT COUNT(*) FROM contests c
WHERE (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('visibility')::text = '' OR c.visibility = sqlc.arg('visibility')::contest_visibility);

-- name: UpdateContest :exec
UPDATE contests
SET login = COALESCE(sqlc.narg('login'), login),
    title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    settings = jsonb_strip_nulls(COALESCE(settings, '{}'::jsonb) || COALESCE(sqlc.narg('settings')::jsonb, '{}'::jsonb)),
    start_time = CASE WHEN sqlc.arg('start_time_set')::boolean THEN sqlc.narg('start_time')::timestamptz ELSE start_time END,
    end_time = CASE WHEN sqlc.arg('end_time_set')::boolean THEN sqlc.narg('end_time')::timestamptz ELSE end_time END,
    owner_id = COALESCE(sqlc.narg('owner_id'), owner_id)
WHERE id = $1;

-- name: DeleteContest :exec
DELETE FROM contests WHERE id = $1;

-- Contest Members (direct access control)

-- name: AddContestMember :exec
INSERT INTO contest_members (contest_id, user_id, role)
VALUES ($1, $2, $3);

-- name: GetContestMember :one
SELECT cm.*, u.username
FROM contest_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.contest_id = $1 AND cm.user_id = $2
FOR SHARE OF cm;

-- name: ListContestMembers :many
SELECT cm.*, u.username, u.email
FROM contest_members cm
JOIN users u ON cm.user_id = u.id
WHERE cm.contest_id = sqlc.arg('contest_id')::uuid
ORDER BY cm.created_at, cm.user_id
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: CountContestMembers :one
SELECT COUNT(*) FROM contest_members WHERE contest_id = $1;

-- name: UpdateContestMemberRole :exec
UPDATE contest_members
SET role = $3
WHERE contest_id = $1 AND user_id = $2;

-- name: RemoveContestMember :exec
DELETE FROM contest_members
WHERE contest_id = $1 AND user_id = $2;

-- name: ListUserContestMemberships :many
SELECT contest_id, role
FROM contest_members
WHERE user_id = $1;

-- name: AddContestMemberIfNotExists :exec
INSERT INTO contest_members (contest_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (contest_id, user_id) DO NOTHING;


-- Contest Problems (linking to packages)

-- name: AddContestProblem :exec
INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal)
VALUES ($1, $2, $3, $4);

-- name: GetContestProblem :one
SELECT cp.*, p.title, p.short_name, p.visibility, p.time_limit_ms, p.memory_limit_mb, p.updated_at AS problem_updated_at
FROM contest_problems cp
JOIN problems p ON cp.problem_id = p.id
WHERE cp.contest_id = $1 AND cp.problem_id = $2;

-- name: ListContestProblems :many
SELECT cp.*, p.title, p.short_name, p.visibility, p.time_limit_ms, p.memory_limit_mb, p.updated_at AS problem_updated_at, pp.url as package_url
FROM contest_problems cp
JOIN problems p ON cp.problem_id = p.id
JOIN problem_packages pp ON cp.package_id = pp.id
WHERE cp.contest_id = $1
ORDER BY cp.ordinal;

-- name: UpdateContestProblemOrdinal :exec
UPDATE contest_problems
SET ordinal = $3
WHERE contest_id = $1 AND problem_id = $2;

-- name: UpdateContestProblemPackage :exec
UPDATE contest_problems
SET package_id = $3
WHERE contest_id = $1 AND problem_id = $2;

-- name: RemoveContestProblem :exec
DELETE FROM contest_problems
WHERE contest_id = $1 AND problem_id = $2;

-- Access check helpers

-- name: ListUserAccessibleContests :many
SELECT c.*, o.login as org_login, o.name as org_name
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE user_has_contest_access(sqlc.arg('user_id')::uuid, c.id)
  AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
ORDER BY
  CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text = 'asc' THEN c.created_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text <> 'asc' THEN c.created_at END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text = 'asc' THEN c.updated_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.updated_at END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text = 'asc' THEN c.title END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.title END DESC,
  c.id DESC
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: ListUserAccessibleContestsByOrg :many
SELECT c.*, o.login as org_login, o.name as org_name
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE user_has_contest_access(sqlc.arg('user_id')::uuid, c.id)
  AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND c.organization_id = sqlc.arg('organization_id')::uuid
ORDER BY
  CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text = 'asc' THEN c.created_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text <> 'asc' THEN c.created_at END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text = 'asc' THEN c.updated_at END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.updated_at END DESC,
  CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text = 'asc' THEN c.title END ASC,
  CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.title END DESC,
  c.id DESC
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: ListDashboardContests :many
SELECT 
    c.id as contest_id,
    c.login as contest_login,
    c.title as contest_title,
    c.start_time as contest_start_time,
    c.end_time as contest_end_time,
    c.created_at as contest_created_at,
    o.id as org_id,
    o.name as org_name,
    o.login as org_login,
    COALESCE(
        (SELECT cm.role::text FROM contest_members cm WHERE cm.contest_id = c.id AND cm.user_id = $1),
        CASE WHEN EXISTS(
            SELECT 1 FROM organization_members om 
            WHERE om.organization_id = c.organization_id AND om.user_id = $1 AND om.role IN ('owner', 'admin')
        ) THEN 'moderator' ELSE 'participant' END
    )::text as user_role,
    sub.last_sub_time
FROM contests c
JOIN organizations o ON c.organization_id = o.id
LEFT JOIN (
    SELECT submissions.contest_id, MAX(submissions.created_at) as last_sub_time
    FROM submissions
    WHERE submissions.owner_id = $1
    GROUP BY submissions.contest_id
) sub ON c.id = sub.contest_id
WHERE user_has_contest_access($1, c.id)
ORDER BY COALESCE(sub.last_sub_time, '1970-01-01 00:00:00+00'::timestamptz) DESC, c.created_at DESC
LIMIT $2;

-- Contest Standings (Scoreboard)

-- name: UpsertContestProblemResult :exec
INSERT INTO contest_problem_results (contest_id, user_id, problem_id, solved, failed_attempts, first_ac_time, time_minutes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (contest_id, user_id, problem_id)
DO UPDATE SET
    solved = EXCLUDED.solved,
    failed_attempts = EXCLUDED.failed_attempts,
    first_ac_time = EXCLUDED.first_ac_time,
    time_minutes = EXCLUDED.time_minutes,
    updated_at = NOW();

-- name: GetContestProblemResult :one
SELECT * FROM contest_problem_results
WHERE contest_id = $1 AND user_id = $2 AND problem_id = $3;

-- name: GetContestScoreboardFromStandings :many
SELECT 
    cm.user_id,
    u.username,
    cpr.problem_id,
    cpr.solved,
    cpr.failed_attempts,
    cpr.first_ac_time,
    cpr.time_minutes
FROM contest_members cm
JOIN users u ON cm.user_id = u.id
LEFT JOIN contest_problem_results cpr ON cpr.contest_id = cm.contest_id AND cpr.user_id = cm.user_id
WHERE cm.contest_id = $1 AND cm.role = 'participant';

-- name: GetSubmissionsForScoreboard :many
SELECT state, created_at
FROM submissions
WHERE contest_id = $1 AND owner_id = $2 AND problem_id = $3
ORDER BY created_at ASC;

-- Contest User Problem Blocks
-- name: CreateContestUserProblemBlock :exec
INSERT INTO contest_user_problem_blocks (contest_id, user_id, problem_id, reason, created_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (contest_id, user_id, problem_id)
DO UPDATE SET
    reason = EXCLUDED.reason,
    created_by = EXCLUDED.created_by,
    created_at = NOW();

-- name: DeleteContestUserProblemBlock :exec
DELETE FROM contest_user_problem_blocks
WHERE contest_id = $1 AND user_id = $2 AND problem_id = $3;

-- name: GetContestUserProblemBlock :one
SELECT contest_id, user_id, problem_id, reason, created_by, created_at
FROM contest_user_problem_blocks
WHERE contest_id = $1 AND user_id = $2 AND problem_id = $3;

-- name: ListContestUserProblemBlocks :many
SELECT contest_id, user_id, problem_id, reason, created_by, created_at
FROM contest_user_problem_blocks
WHERE contest_id = $1 AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid);




-- name: LockContest :one
SELECT id FROM contests WHERE id = @id::uuid FOR UPDATE;

-- name: CountUserAccessibleContests :one
SELECT COUNT(*) FROM contests c
WHERE user_has_contest_access(sqlc.arg('user_id')::uuid, c.id)
  AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%');

-- name: CountUserAccessibleContestsByOrg :one
SELECT COUNT(*) FROM contests c
WHERE user_has_contest_access(sqlc.arg('user_id')::uuid, c.id)
  AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
  AND c.organization_id = sqlc.arg('organization_id')::uuid;
