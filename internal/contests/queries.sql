-- Contest CRUD operations

-- name: CreateContest :one
INSERT INTO contests (id, title, created_by)
VALUES (@id::uuid, @title, @created_by::uuid)
RETURNING id;

-- name: GetContest :one
SELECT
    id,
    title,
    description,
    visibility,
    monitor_scope,
    submissions_list_scope,
    submissions_review_scope,
    created_by,
    start_time,
    end_time,
    scoring_mode,
    created_at,
    updated_at
FROM contests
WHERE id = @id::uuid;

-- name: UpdateContest :exec
UPDATE contests
SET title                    = COALESCE(sqlc.narg('title'), title),
    description              = COALESCE(sqlc.narg('description'), description),
    visibility               = COALESCE(sqlc.narg('visibility'), visibility),
    monitor_scope            = COALESCE(sqlc.narg('monitor_scope'), monitor_scope),
    submissions_list_scope   = COALESCE(sqlc.narg('submissions_list_scope'), submissions_list_scope),
    submissions_review_scope = COALESCE(sqlc.narg('submissions_review_scope'), submissions_review_scope),
    start_time               = COALESCE(sqlc.narg('start_time'), start_time),
    end_time                 = COALESCE(sqlc.narg('end_time'), end_time),
    scoring_mode             = COALESCE(sqlc.narg('scoring_mode'), scoring_mode)
WHERE id = @id::uuid;

-- name: DeleteContest :exec
DELETE FROM contests
WHERE id = @id::uuid;

-- Admin contests listing

-- name: ListAdminContests :many
SELECT c.id,
       c.title,
       c.description,
       c.visibility,
       c.monitor_scope,
       c.submissions_list_scope,
       c.submissions_review_scope,
       c.created_by,
       c.start_time,
       c.end_time,
       c.scoring_mode,
       c.created_at,
       c.updated_at
FROM contests c
WHERE (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    )
  AND (
    sqlc.narg('visibility')::text IS NULL
        OR c.visibility = sqlc.narg('visibility')::contest_visibility
    )
ORDER BY CASE
             WHEN sqlc.narg('search')::text IS NOT NULL
                 AND sqlc.narg('search') != ''
                 AND LENGTH(sqlc.narg('search')) >= 3 THEN word_similarity(c.title, sqlc.narg('search'))
             END DESC NULLS LAST,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text IS NULL OR sqlc.narg('sort_by') = '' THEN c.created_at
             END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAdminContests :one
SELECT COUNT(*)
FROM contests c
WHERE (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    )
  AND (
    sqlc.narg('visibility')::text IS NULL
        OR c.visibility = sqlc.narg('visibility')::contest_visibility
    );

-- Public contests listing

-- name: ListPublicContests :many
SELECT c.id,
       c.title,
       c.description,
       c.visibility,
       c.monitor_scope,
       c.submissions_list_scope,
       c.submissions_review_scope,
       c.created_by,
       c.start_time,
       c.end_time,
       c.scoring_mode,
       c.created_at,
       c.updated_at
FROM contests c
WHERE c.visibility = 'public'
  AND (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    )
ORDER BY CASE
             WHEN sqlc.narg('search')::text IS NOT NULL
                 AND sqlc.narg('search') != ''
                 AND LENGTH(sqlc.narg('search')) >= 3 THEN word_similarity(c.title, sqlc.narg('search'))
             END DESC NULLS LAST,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text IS NULL OR sqlc.narg('sort_by') = '' THEN c.created_at
             END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPublicContests :one
SELECT COUNT(*)
FROM contests c
WHERE c.visibility = 'public'
  AND (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    );

-- User contests listing

-- name: ListUserContests :many
SELECT c.id,
       c.title,
       c.description,
       c.visibility,
       c.monitor_scope,
       c.submissions_list_scope,
       c.submissions_review_scope,
       c.created_by,
       c.start_time,
       c.end_time,
       c.scoring_mode,
       c.created_at,
       c.updated_at
FROM contests c
LEFT JOIN submissions s ON s.contest_id = c.id AND s.created_by = @created_by::uuid
WHERE (
    -- Private contests where user is member
    (c.visibility = 'private' AND EXISTS(SELECT 1 FROM contest_members cm WHERE cm.contest_id = c.id AND cm.user_id = @created_by::uuid))
    OR
    -- Public contests where user is member OR has submissions
    (c.visibility = 'public' AND (
        EXISTS(SELECT 1 FROM contest_members cm WHERE cm.contest_id = c.id AND cm.user_id = @created_by::uuid)
        OR EXISTS(SELECT 1 FROM submissions sub WHERE sub.contest_id = c.id AND sub.created_by = @created_by::uuid)
    ))
)
  AND (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    )
GROUP BY c.id
ORDER BY CASE
             WHEN sqlc.narg('search')::text IS NOT NULL
                 AND sqlc.narg('search') != ''
                 AND LENGTH(sqlc.narg('search')) >= 3 THEN word_similarity(c.title, sqlc.narg('search'))
             END DESC NULLS LAST,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'last_submission_time' AND sqlc.narg('sort_order')::text = 'desc' THEN MAX(s.created_at)
             END DESC NULLS LAST,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'last_submission_time' AND sqlc.narg('sort_order')::text = 'asc' THEN MAX(s.created_at)
             END NULLS LAST,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text IS NULL OR sqlc.narg('sort_by') = '' THEN MAX(s.created_at)
             END DESC NULLS LAST
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountUserContests :one
SELECT COUNT(DISTINCT c.id)
FROM contests c
WHERE (
    -- Private contests where user is member
    (c.visibility = 'private' AND EXISTS(SELECT 1 FROM contest_members cm WHERE cm.contest_id = c.id AND cm.user_id = @user_id::uuid))
    OR
    -- Public contests where user is member OR has submissions
    (c.visibility = 'public' AND (
        EXISTS(SELECT 1 FROM contest_members cm WHERE cm.contest_id = c.id AND cm.user_id = @user_id::uuid)
        OR EXISTS(SELECT 1 FROM submissions sub WHERE sub.contest_id = c.id AND sub.created_by = @user_id::uuid)
    ))
)
  AND (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    );

-- Workshop contests listing

-- name: ListWorkshopContests :many
SELECT c.id,
       c.title,
       c.description,
       c.visibility,
       c.monitor_scope,
       c.submissions_list_scope,
       c.submissions_review_scope,
       c.created_by,
       c.start_time,
       c.end_time,
       c.scoring_mode,
       c.created_at,
       c.updated_at
FROM contests c
INNER JOIN contest_members cm ON cm.contest_id = c.id
WHERE cm.user_id = @user_id::uuid
  AND cm.role IN ('owner', 'moderator')
  AND (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    )
ORDER BY CASE
             WHEN sqlc.narg('search')::text IS NOT NULL
                 AND sqlc.narg('search') != ''
                 AND LENGTH(sqlc.narg('search')) >= 3 THEN word_similarity(c.title, sqlc.narg('search'))
             END DESC NULLS LAST,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'created_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'updated_at' AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
             END DESC,
         CASE
             WHEN sqlc.narg('sort_by')::text = 'title' AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
             END,
         CASE
             WHEN sqlc.narg('sort_by')::text IS NULL OR sqlc.narg('sort_by') = '' THEN c.created_at
             END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountWorkshopContests :one
SELECT COUNT(DISTINCT c.id)
FROM contests c
INNER JOIN contest_members cm ON cm.contest_id = c.id
WHERE cm.user_id = @user_id::uuid
  AND cm.role IN ('owner', 'moderator')
  AND (
    sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
        CASE
            WHEN LENGTH(sqlc.narg('search')) < 3 THEN c.title ILIKE '%' || sqlc.narg('search') || '%'
            ELSE word_similarity(c.title, sqlc.narg('search')) > 0.1
            END
        )
    );

-- Contest problem operations

-- name: CreateContestProblem :exec
INSERT INTO contest_problem (problem_id, contest_id, position)
VALUES (@problem_id::uuid,
        @contest_id::uuid,
        COALESCE(
                (SELECT MAX(position)
                 FROM contest_problem
                 WHERE contest_id = @contest_id::uuid),
                0
        ) + 1);

-- name: GetContestProblem :one
SELECT cp.problem_id,
       p.title,
       p.time_limit,
       p.memory_limit,
       cp.position,
       p.legend_html,
       p.input_format_html,
       p.output_format_html,
       p.notes_html,
       p.scoring_html,
       p.created_at,
       p.updated_at
FROM contest_problem cp
         LEFT JOIN problems p ON cp.problem_id = p.id
WHERE cp.contest_id = @contest_id::uuid
  AND cp.problem_id = @problem_id::uuid;

-- name: GetContestProblems :many
SELECT cp.problem_id,
       p.title,
       p.time_limit,
       p.memory_limit,
       cp.position,
       p.created_at,
       p.updated_at
FROM contest_problem cp
         LEFT JOIN problems p ON cp.problem_id = p.id
WHERE cp.contest_id = @contest_id::uuid
ORDER BY cp.position;

-- name: DeleteContestProblem :exec
DELETE
FROM contest_problem
WHERE contest_id = @contest_id::uuid
  AND problem_id = @problem_id::uuid;

-- Contest member operations

-- name: CreateContestMember :exec
INSERT INTO contest_members (contest_id, user_id, role)
VALUES (@contest_id::uuid, @user_id::uuid, @role);

-- name: GetContestMember :one
SELECT contest_id,
       user_id,
       role
FROM contest_members
WHERE contest_id = @contest_id::uuid
  AND user_id = @user_id::uuid;

-- name: UpdateContestMember :exec
UPDATE contest_members
SET role = @role
WHERE contest_id = @contest_id::uuid
  AND user_id = @user_id::uuid;

-- name: DeleteContestMember :exec
DELETE FROM contest_members
WHERE user_id = @user_id::uuid AND contest_id = @contest_id::uuid;

-- name: ListContestMembers :many
SELECT u.id AS user_id,
    cm.contest_id,
    u.username,
    u.role AS role,
    cm.role AS contest_role,
    u.kratos_id,
    u.created_at,
    u.updated_at
FROM contest_members cm
    LEFT JOIN users u ON cm.user_id = u.id
WHERE contest_id = @contest_id::uuid
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountContestMembers :one
SELECT COUNT(*)
FROM contest_members
WHERE contest_id = @contest_id::uuid;

-- Contest Access Requests operations

-- name: CreateAccessRequest :one
INSERT INTO contest_access_requests (id, contest_id, user_id, status)
VALUES (@id::uuid, @contest_id::uuid, @user_id::uuid, @status)
RETURNING id;

-- name: GetAccessRequest :one
SELECT id, contest_id, user_id, status, created_at, updated_at
FROM contest_access_requests
WHERE contest_id = @contest_id::uuid
  AND user_id = @user_id::uuid;

-- name: GetAccessRequestById :one
SELECT id, contest_id, user_id, status, created_at, updated_at
FROM contest_access_requests
WHERE id = @id::uuid;

-- name: ListAccessRequests :many
SELECT car.id, car.contest_id, car.user_id, car.status, car.created_at, car.updated_at,
       u.username, u.role AS user_role
FROM contest_access_requests car
LEFT JOIN users u ON car.user_id = u.id
WHERE car.contest_id = @contest_id::uuid
  AND (sqlc.narg('status')::text IS NULL OR car.status = sqlc.narg('status')::contest_access_request_status)
ORDER BY car.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAccessRequests :one
SELECT COUNT(*)
FROM contest_access_requests
WHERE contest_id = @contest_id::uuid
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::contest_access_request_status);

-- name: UpdateAccessRequestStatus :exec
UPDATE contest_access_requests
SET status = @status
WHERE id = @id::uuid;

-- name: DeleteAccessRequest :exec
DELETE FROM contest_access_requests
WHERE id = @id::uuid;

-- Contest Invitations operations

-- name: CreateInvitation :one
INSERT INTO contest_invitations (id, contest_id, user_id, invited_by, status)
VALUES (@id::uuid, @contest_id::uuid, @user_id::uuid, @invited_by::uuid, @status)
RETURNING id;

-- name: GetInvitation :one
SELECT id, contest_id, user_id, invited_by, status, created_at, updated_at
FROM contest_invitations
WHERE id = @id::uuid;

-- name: GetInvitationByUser :one
SELECT id, contest_id, user_id, invited_by, status, created_at, updated_at
FROM contest_invitations
WHERE contest_id = @contest_id::uuid
  AND user_id = @user_id::uuid
  AND status != 'revoked'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListInvitations :many
SELECT ci.id, ci.contest_id, ci.user_id, ci.invited_by, ci.status, ci.created_at, ci.updated_at,
       u.username, u.role AS user_role,
       inv.username AS invited_by_username
FROM contest_invitations ci
LEFT JOIN users u ON ci.user_id = u.id
LEFT JOIN users inv ON ci.invited_by = inv.id
WHERE ci.contest_id = @contest_id::uuid
  AND (sqlc.narg('status')::text IS NULL OR ci.status = sqlc.narg('status')::contest_invitation_status)
ORDER BY ci.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListUserInvitations :many
SELECT ci.id, ci.contest_id, ci.user_id, ci.invited_by, ci.status, ci.created_at, ci.updated_at,
       c.title AS contest_title,
       inv.username AS invited_by_username
FROM contest_invitations ci
LEFT JOIN contests c ON ci.contest_id = c.id
LEFT JOIN users inv ON ci.invited_by = inv.id
WHERE ci.user_id = @user_id::uuid
  AND ci.status = 'pending'
ORDER BY ci.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountInvitations :one
SELECT COUNT(*)
FROM contest_invitations
WHERE contest_id = @contest_id::uuid
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::contest_invitation_status);

-- name: UpdateInvitationStatus :exec
UPDATE contest_invitations
SET status = @status
WHERE id = @id::uuid;

-- name: DeleteInvitation :exec
DELETE FROM contest_invitations
WHERE id = @id::uuid;

-- Problem Position Management

-- name: UpdateContestProblemPosition :exec
UPDATE contest_problem
SET position = @position
WHERE contest_id = @contest_id::uuid
  AND problem_id = @problem_id::uuid;

-- name: ReorderContestProblemsAfterDelete :exec
UPDATE contest_problem
SET position = position - 1
WHERE contest_id = @contest_id::uuid
  AND position > @deleted_position;

-- name: GetMaxProblemPosition :one
SELECT COALESCE(MAX(position), -1) AS max_position
FROM contest_problem
WHERE contest_id = @contest_id::uuid;

-- Monitor Query - Calculate standings

-- name: GetContestMonitor :many
SELECT 
    u.id AS user_id,
    u.username,
    COALESCE(SUM(CASE 
        WHEN s.state = 200 THEN 
            CASE 
                WHEN c.scoring_mode = 'points' THEN s.score 
                ELSE 100  -- binary mode: full points for AC
            END
        ELSE 0 
    END), 0) AS total_score,
    COALESCE(SUM(CASE WHEN s.state = 200 THEN s.penalty ELSE 0 END), 0) AS total_penalty,
    COUNT(DISTINCT CASE WHEN s.state = 200 THEN cp.problem_id END) AS solved_count
FROM contest_members cm
LEFT JOIN users u ON cm.user_id = u.id
LEFT JOIN contests c ON cm.contest_id = c.id
LEFT JOIN contest_problem cp ON cp.contest_id = cm.contest_id
LEFT JOIN LATERAL (
    SELECT 
        s.problem_id,
        s.state,
        s.score,
        s.penalty
    FROM submissions s
    WHERE s.contest_id = cm.contest_id
      AND s.created_by = cm.user_id
      AND s.problem_id = cp.problem_id
    ORDER BY 
        CASE WHEN s.state = 200 THEN 0 ELSE 1 END,  -- AC submissions first
        s.score DESC,  -- Then by score
        s.created_at ASC  -- Then by time
    LIMIT 1
) s ON true
WHERE cm.contest_id = @contest_id::uuid
  AND cm.role = 'participant'
GROUP BY u.id, u.username
ORDER BY total_score DESC, total_penalty ASC, u.username ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountContestMonitorRows :one
SELECT COUNT(DISTINCT cm.user_id)
FROM contest_members cm
WHERE cm.contest_id = @contest_id::uuid
  AND cm.role = 'participant';

-- name: GetMonitorProblemDetails :many
SELECT 
    cp.problem_id,
    cp.position,
    COALESCE(s.score, 0) AS score,
    COALESCE(s.attempts, 0) AS attempts,
    COALESCE(s.solved, false) AS solved,
    COALESCE(s.penalty, 0) AS penalty
FROM contest_problem cp
LEFT JOIN LATERAL (
    SELECT 
        MAX(CASE WHEN sub.state = 200 THEN sub.score ELSE 0 END) AS score,
        COUNT(*) AS attempts,
        bool_or(sub.state = 200) AS solved,
        MAX(CASE WHEN sub.state = 200 THEN sub.penalty ELSE 0 END) AS penalty
    FROM submissions sub
    WHERE sub.contest_id = @contest_id::uuid
      AND sub.problem_id = cp.problem_id
      AND sub.created_by = @user_id::uuid
) s ON true
WHERE cp.contest_id = @contest_id::uuid
ORDER BY cp.position ASC;
