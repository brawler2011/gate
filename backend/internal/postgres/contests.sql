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


-- Contest Problems (linking to packages)

-- name: AddContestProblem :exec
INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal)
VALUES ($1, $2, $3, (SELECT COALESCE(MAX(ordinal), 0) + 1 FROM contest_problems WHERE contest_id = $1))
ON CONFLICT (contest_id, problem_id) DO UPDATE SET package_id = EXCLUDED.package_id;

-- name: GetContestProblem :one
SELECT cp.*, pp.title, pp.short_name, 'private'::text AS visibility, pp.time_limit_ms, pp.memory_limit_mb, pp.created_at AS problem_updated_at
FROM contest_problems cp
JOIN problem_packages pp ON cp.package_id = pp.id
WHERE cp.contest_id = $1 AND cp.problem_id = $2;

-- name: ListContestProblems :many
SELECT cp.*, pp.title, pp.short_name, 'private'::text AS visibility, pp.time_limit_ms, pp.memory_limit_mb, pp.created_at AS problem_updated_at, pp.url as package_url
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

-- Access check helpers



-- name: ListDashboardContests :many
WITH actor AS (
  SELECT u.id, u.role FROM users u
  WHERE u.id = sqlc.narg('viewer_id')::uuid
    AND u.role IN ('user', 'admin')
    AND (u.expires_at IS NULL OR u.expires_at > sqlc.arg('now')::timestamptz)
)
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
        (SELECT cm.role::text FROM contest_members cm WHERE cm.contest_id = c.id AND cm.user_id = sqlc.arg('user_id')::uuid),
        CASE WHEN EXISTS(
            SELECT 1 FROM organization_members om 
            WHERE om.organization_id = c.organization_id AND om.user_id = sqlc.arg('user_id')::uuid AND om.role IN ('owner', 'admin')
        ) THEN 'moderator' ELSE 'participant' END
    )::text as user_role,
    sub.last_sub_time
FROM contests c
JOIN organizations o ON c.organization_id = o.id
LEFT JOIN (
    SELECT submissions.contest_id, MAX(submissions.created_at) as last_sub_time
    FROM submissions
    WHERE submissions.owner_id = sqlc.arg('user_id')::uuid
    GROUP BY submissions.contest_id
) sub ON c.id = sub.contest_id
WHERE (sqlc.narg('viewer_id')::uuid IS NULL OR sqlc.narg('viewer_id')::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR EXISTS (SELECT 1 FROM actor))
AND NOT EXISTS (SELECT 1 FROM organization_members invalid WHERE invalid.organization_id = c.organization_id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('member', 'admin', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM contest_members invalid WHERE invalid.contest_id = c.id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('participant', 'moderator', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM contest_teams link JOIN team_members tm ON tm.team_id = link.team_id WHERE link.contest_id = c.id AND tm.user_id = sqlc.narg('viewer_id')::uuid AND (tm.role::text NOT IN ('member', 'maintainer') OR link.role::text NOT IN ('participant', 'moderator')))
    AND COALESCE(NULLIF(c.settings->>'participation_mode', ''), 'open') IN ('open', 'by_request', 'invite_only')
AND c.visibility IN ('public', 'private') AND (c.visibility = 'public' OR EXISTS (
    SELECT 1 FROM actor a WHERE a.role = 'admin' OR c.owner_id = a.id
    OR EXISTS (SELECT 1 FROM organization_members om WHERE om.organization_id = c.organization_id AND om.user_id = a.id AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM contest_members m WHERE m.contest_id = c.id AND m.user_id = a.id AND m.role IN ('participant', 'moderator', 'owner'))
    OR EXISTS (SELECT 1 FROM contest_teams ct
      JOIN teams t ON t.id = ct.team_id JOIN team_members tm ON tm.team_id = t.id
      WHERE ct.contest_id = c.id AND tm.user_id = a.id
        AND t.organization_id = c.organization_id AND tm.role IN ('member', 'maintainer')
        AND ct.role IN ('participant', 'moderator'))))
ORDER BY COALESCE(sub.last_sub_time, '1970-01-01 00:00:00+00'::timestamptz) DESC, c.created_at DESC, c.id DESC
LIMIT sqlc.arg('page_size')::int;

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

-- name: GetContestScoreboardSubmissions :many
SELECT s.owner_id, s.problem_id,
 CASE WHEN sqlc.narg('freeze_time')::timestamptz IS NULL THEN s.state ELSE COALESCE(
 (SELECT NULLIF(h.state, 1) FROM submission_state_history h WHERE h.submission_id=s.id AND h.observed_at < sqlc.narg('freeze_time')::timestamptz ORDER BY h.observed_at DESC,h.id DESC LIMIT 1),0) END::integer AS state,
 s.created_at
FROM submissions s
WHERE s.contest_id = sqlc.arg('contest_id')::uuid AND s.owner_id IS NOT NULL AND s.problem_id IS NOT NULL
ORDER BY s.owner_id, s.problem_id, s.created_at ASC;

-- name: GetUserScoreboardSubmissions :many
SELECT s.owner_id, s.problem_id,
 CASE WHEN sqlc.narg('freeze_time')::timestamptz IS NULL THEN s.state ELSE COALESCE(
 (SELECT NULLIF(h.state, 1) FROM submission_state_history h WHERE h.submission_id=s.id AND h.observed_at < sqlc.narg('freeze_time')::timestamptz ORDER BY h.observed_at DESC,h.id DESC LIMIT 1),0) END::integer AS state,
 s.created_at
FROM submissions s
WHERE s.contest_id = sqlc.arg('contest_id')::uuid AND s.owner_id = sqlc.arg('owner_id')::uuid AND s.problem_id IS NOT NULL
ORDER BY s.problem_id, s.created_at ASC;

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







-- name: GetAuthorizationContestMember :one
SELECT cm.*, u.username
FROM contest_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.contest_id = $1 AND cm.user_id = $2
FOR SHARE OF cm;
