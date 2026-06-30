-- Contests queries (new schema with Organizations)

-- name: CreateContest :one
INSERT INTO contests (id, organization_id, owner_id, visibility, title, short_name, description, settings, access_policy, start_time, end_time)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetContestByID :one
SELECT * FROM contests WHERE id = $1;

-- name: GetContestByShortName :one
SELECT * FROM contests
WHERE organization_id = $1 AND short_name = $2;

-- name: ListContests :many
SELECT * FROM contests
WHERE organization_id = $1
  AND ($2::text = '' OR title ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR visibility = $3::contest_visibility)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountContests :one
SELECT COUNT(*) FROM contests
WHERE organization_id = $1
  AND ($2::text = '' OR title ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR visibility = $3::contest_visibility);

-- name: ListAllContests :many
SELECT c.* FROM contests c
WHERE ($1::text = '' OR c.title ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR c.visibility = $2::contest_visibility)
ORDER BY c.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAllContests :one
SELECT COUNT(*) FROM contests c
WHERE ($1::text = '' OR c.title ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR c.visibility = $2::contest_visibility);

-- name: UpdateContest :exec
UPDATE contests
SET title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    settings = COALESCE(sqlc.narg('settings'), settings),
    access_policy = COALESCE(sqlc.narg('access_policy'), access_policy),
    start_time = COALESCE(sqlc.narg('start_time'), start_time),
    end_time = COALESCE(sqlc.narg('end_time'), end_time),
    owner_id = COALESCE(sqlc.narg('owner_id'), owner_id)
WHERE id = $1;

-- name: DeleteContest :exec
DELETE FROM contests WHERE id = $1;

-- Contest Members (direct access control)

-- name: AddContestMember :exec
INSERT INTO contest_members (contest_id, user_id, role)
VALUES ($1, $2, $3);

-- name: GetContestMember :one
SELECT * FROM contest_members
WHERE contest_id = $1 AND user_id = $2;

-- name: ListContestMembers :many
SELECT cm.*, u.username, u.email
FROM contest_members cm
JOIN users u ON cm.user_id = u.id
WHERE cm.contest_id = $1
ORDER BY cm.created_at;

-- name: UpdateContestMemberRole :exec
UPDATE contest_members
SET role = $3
WHERE contest_id = $1 AND user_id = $2;

-- name: RemoveContestMember :exec
DELETE FROM contest_members
WHERE contest_id = $1 AND user_id = $2;

-- Contest Problems (linking to packages)

-- name: AddContestProblem :exec
INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal)
VALUES ($1, $2, $3, $4);

-- name: GetContestProblem :one
SELECT cp.*, p.title, p.short_name, p.visibility
FROM contest_problems cp
JOIN problems p ON cp.problem_id = p.id
WHERE cp.contest_id = $1 AND cp.problem_id = $2;

-- name: ListContestProblems :many
SELECT cp.*, p.title, p.short_name, p.visibility, pp.url as package_url
FROM contest_problems cp
JOIN problems p ON cp.problem_id = p.id
JOIN problem_packages pp ON cp.package_id = pp.id
WHERE cp.contest_id = $1
ORDER BY cp.ordinal;

-- name: UpdateContestProblemOrdinal :exec
UPDATE contest_problems
SET ordinal = $3
WHERE contest_id = $1 AND problem_id = $2;

-- name: RemoveContestProblem :exec
DELETE FROM contest_problems
WHERE contest_id = $1 AND problem_id = $2;

-- Access check helpers

-- name: CheckUserHasContestAccess :one
SELECT user_has_contest_access($1, $2) as has_access;

-- name: CheckUserIsContestModerator :one
SELECT user_is_contest_moderator($1, $2) as is_moderator;

-- name: ListUserAccessibleContests :many
SELECT c.* FROM contests c
WHERE user_has_contest_access($1, c.id)
ORDER BY c.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUserAccessibleContestsByOrg :many
SELECT c.* FROM contests c
WHERE user_has_contest_access($1, c.id)
  AND c.organization_id = $2
ORDER BY c.created_at DESC
LIMIT $3 OFFSET $4;
