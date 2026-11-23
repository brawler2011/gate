
-- name: CreateContest :one
INSERT INTO contests (id, organization_id, visibility, title, login, description, start_time, end_time)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetContestByID :one
SELECT sqlc.embed(c), o.login as org_login
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE c.id = $1;

-- name: GetContestByLogin :one
SELECT sqlc.embed(c), o.login as org_login
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE c.organization_id = $1 AND LOWER(c.login) = LOWER($2);

-- name: GetContestByOrgLoginAndContestLogin :one
SELECT sqlc.embed(c), o.login as org_login
FROM contests c
JOIN organizations o ON c.organization_id = o.id
WHERE LOWER(o.login) = LOWER($1) AND LOWER(c.login) = LOWER($2);





-- name: UpdateContest :exec
UPDATE contests
SET login = COALESCE(sqlc.narg('login'), login),
    title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    join_policy = COALESCE(sqlc.narg('join_policy'), join_policy),
    monitor_scope = COALESCE(sqlc.narg('monitor_scope'), monitor_scope),
    submissions_list_scope = COALESCE(sqlc.narg('submissions_list_scope'), submissions_list_scope),
    submissions_review_scope = COALESCE(sqlc.narg('submissions_review_scope'), submissions_review_scope),
    submission_details_scope = COALESCE(sqlc.narg('submission_details_scope'), submission_details_scope),
    freeze_status = COALESCE(sqlc.narg('freeze_status'), freeze_status),
    freeze_duration_minutes = CASE WHEN sqlc.arg('freeze_duration_set')::boolean THEN sqlc.narg('freeze_duration_minutes')::integer ELSE freeze_duration_minutes END,
    hide_statements = COALESCE(sqlc.narg('hide_statements'), hide_statements),
    allow_clarifications = COALESCE(sqlc.narg('allow_clarifications'), allow_clarifications),
    enable_drafts = COALESCE(sqlc.narg('enable_drafts'), enable_drafts),
    enable_upsolving = COALESCE(sqlc.narg('enable_upsolving'), enable_upsolving),
    allow_virtual = COALESCE(sqlc.narg('allow_virtual'), allow_virtual),
    languages = COALESCE(sqlc.narg('languages')::text[], languages),
    start_time = CASE WHEN sqlc.arg('start_time_set')::boolean THEN sqlc.narg('start_time')::timestamptz ELSE start_time END,
    end_time = CASE WHEN sqlc.arg('end_time_set')::boolean THEN sqlc.narg('end_time')::timestamptz ELSE end_time END
WHERE id = $1;

-- name: DeleteContest :exec
DELETE FROM contests WHERE id = $1;


-- name: AddContestMember :exec
INSERT INTO contest_members (contest_id, user_id, role)
VALUES ($1, $2, $3);

-- name: ListContestOwners :many
SELECT user_id FROM contest_members WHERE contest_id = $1 AND role = 'owner' ORDER BY user_id;

-- name: GetContestMember :one
SELECT cm.*, u.username
FROM contest_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.contest_id = $1 AND cm.user_id = $2;

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



-- name: AddContestProblem :exec
INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal)
VALUES ($1, $2, $3, (SELECT COALESCE(MAX(ordinal), 0) + 1 FROM contest_problems WHERE contest_id = $1))
ON CONFLICT (contest_id, problem_id) DO UPDATE SET package_id = EXCLUDED.package_id;

-- name: GetContestProblem :one
SELECT cp.*, pp.title, pp.short_name, 'private'::text AS visibility, pp.time_limit_ms, pp.memory_limit_mb, pp.input_file, pp.output_file, pp.created_at AS problem_updated_at
FROM contest_problems cp
JOIN problem_packages pp ON cp.package_id = pp.id
WHERE cp.contest_id = $1 AND cp.problem_id = $2;

-- name: ListContestProblems :many
SELECT cp.*, pp.title, pp.short_name, 'private'::text AS visibility, pp.time_limit_ms, pp.memory_limit_mb, pp.input_file, pp.output_file, pp.created_at AS problem_updated_at
FROM contest_problems cp
JOIN problem_packages pp ON cp.package_id = pp.id
WHERE cp.contest_id = $1
ORDER BY cp.ordinal;

-- name: GetMaxContestProblemOrdinal :one
SELECT COALESCE(MAX(ordinal), 0)::int FROM contest_problems WHERE contest_id = $1;

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
    COALESCE(a.role, 'participant')::contest_role as user_role,
    sub.last_sub_time
FROM contests c
JOIN organizations o ON c.organization_id = o.id
LEFT JOIN contest_access a ON a.contest_id = c.id AND a.user_id = sqlc.arg('viewer_id')::uuid
LEFT JOIN (
    SELECT submissions.contest_id, MAX(submissions.created_at) as last_sub_time
    FROM submissions
    WHERE submissions.owner_id = sqlc.arg('viewer_id')::uuid
    GROUP BY submissions.contest_id
) sub ON c.id = sub.contest_id
WHERE (c.visibility = 'public' OR a.role IS NOT NULL OR sqlc.arg('all')::boolean)
ORDER BY COALESCE(sub.last_sub_time, '1970-01-01 00:00:00+00'::timestamptz) DESC, c.created_at DESC, c.id DESC
LIMIT sqlc.arg('page_size')::int;


-- name: ListContestParticipants :many
SELECT cm.user_id, u.username
FROM contest_members cm
JOIN users u ON cm.user_id = u.id
WHERE cm.contest_id = $1 AND cm.role = 'participant';

-- name: GetContestScoreboardSubmissions :many
SELECT s.owner_id, s.problem_id,
 s.state,
 s.created_at
FROM submissions s
WHERE s.contest_id = sqlc.arg('contest_id')::uuid AND s.kind = 'contest' AND s.owner_id IS NOT NULL AND s.problem_id IS NOT NULL
ORDER BY s.owner_id, s.problem_id, s.created_at ASC;

-- name: GetUserScoreboardSubmissions :many
SELECT s.owner_id, s.problem_id,
 s.state,
 s.created_at
FROM submissions s
WHERE s.contest_id = sqlc.arg('contest_id')::uuid AND s.owner_id = sqlc.arg('owner_id')::uuid AND s.kind = 'contest' AND s.problem_id IS NOT NULL
ORDER BY s.problem_id, s.created_at ASC;

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

-- name: CreateVirtualRun :exec
INSERT INTO contest_virtual_runs (contest_id, user_id, started_at, ends_at) VALUES ($1, $2, $3, $4);

-- name: FinishVirtualRun :exec
UPDATE contest_virtual_runs SET finished_at = sqlc.arg('finished_at')::timestamptz
WHERE contest_id = $1 AND user_id = $2 AND finished_at IS NULL AND ends_at > sqlc.arg('finished_at')::timestamptz;

-- name: ListVirtualRuns :many
SELECT r.user_id, u.username, r.started_at, r.ends_at, r.finished_at
FROM contest_virtual_runs r JOIN users u ON u.id = r.user_id
WHERE r.contest_id = $1;

-- name: GetVirtualScoreboardSubmissions :many
SELECT s.owner_id, s.problem_id, s.state, s.created_at
FROM submissions s
WHERE s.contest_id = sqlc.arg('contest_id')::uuid AND s.kind = 'virtual' AND s.owner_id IS NOT NULL AND s.problem_id IS NOT NULL
ORDER BY s.owner_id, s.problem_id, s.created_at ASC;
