-- Contest CRUD operations
-- name: CreateContest :one
INSERT INTO contests (id, title, created_by)
VALUES (@id::uuid, @title, @created_by::uuid)
RETURNING id;
-- name: GetContest :one
SELECT id,
    title,
    description,
    visibility,
    monitor_scope,
    submissions_list_scope,
    submissions_review_scope,
    created_by,
    created_at,
    updated_at
FROM contests
WHERE id = @id::uuid;
-- name: UpdateContest :exec
UPDATE contests
SET title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    monitor_scope = COALESCE(sqlc.narg('monitor_scope'), monitor_scope),
    submissions_list_scope = COALESCE(
        sqlc.narg('submissions_list_scope'),
        submissions_list_scope
    ),
    submissions_review_scope = COALESCE(
        sqlc.narg('submissions_review_scope'),
        submissions_review_scope
    )
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
    c.created_at,
    c.updated_at
FROM contests c
WHERE (
        sqlc.arg('search')::text = ''
        OR (
            CASE
                WHEN LENGTH(sqlc.arg('search')) < 3 THEN c.title ILIKE '%' || sqlc.arg('search') || '%'
                ELSE word_similarity(c.title, sqlc.arg('search')) > 0.1
            END
        )
    )
    AND (
        sqlc.narg('visibility')::text IS NULL
        OR c.visibility::text = sqlc.narg('visibility')::text
    )
ORDER BY CASE
        WHEN sqlc.arg('search')::text IS NOT NULL
        AND sqlc.arg('search') != ''
        AND LENGTH(sqlc.arg('search')) >= 3 THEN word_similarity(c.title, sqlc.arg('search'))
    END DESC NULLS LAST,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text IS NULL
        OR sqlc.narg('sort_by') = '' THEN c.created_at
    END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
-- name: CountAdminContests :one
SELECT COUNT(*)
FROM contests c
WHERE (
        sqlc.arg('search')::text = ''
        OR (
            CASE
                WHEN LENGTH(sqlc.arg('search')) < 3 THEN c.title ILIKE '%' || sqlc.arg('search') || '%'
                ELSE word_similarity(c.title, sqlc.arg('search')) > 0.1
            END
        )
    )
    AND (
        sqlc.narg('visibility')::text IS NULL
        OR c.visibility::text = sqlc.narg('visibility')::text
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
    c.created_at,
    c.updated_at
FROM contests c
WHERE c.visibility = 'public'
    AND (
        sqlc.arg('search')::text = ''
        OR (
            CASE
                WHEN LENGTH(sqlc.arg('search')) < 3 THEN c.title ILIKE '%' || sqlc.arg('search') || '%'
                ELSE word_similarity(c.title, sqlc.arg('search')) > 0.1
            END
        )
    )
ORDER BY CASE
        WHEN sqlc.arg('search')::text IS NOT NULL
        AND sqlc.arg('search') != ''
        AND LENGTH(sqlc.arg('search')) >= 3 THEN word_similarity(c.title, sqlc.arg('search'))
    END DESC NULLS LAST,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text IS NULL
        OR sqlc.narg('sort_by') = '' THEN c.created_at
    END DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
-- name: CountPublicContests :one
SELECT COUNT(*)
FROM contests c
WHERE c.visibility = 'public'
    AND (
        sqlc.arg('search')::text = ''
        OR (
            CASE
                WHEN LENGTH(sqlc.arg('search')) < 3 THEN c.title ILIKE '%' || sqlc.arg('search') || '%'
                ELSE word_similarity(c.title, sqlc.arg('search')) > 0.1
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
    c.created_at,
    c.updated_at
FROM contests c
    LEFT JOIN submissions s ON s.contest_id = c.id
    AND s.created_by = @created_by::uuid
WHERE (
        -- Private contests where user is member
        (
            c.visibility = 'private'
            AND EXISTS(
                SELECT 1
                FROM contest_members cm
                WHERE cm.contest_id = c.id
                    AND cm.user_id = @created_by::uuid
            )
        )
        OR -- Public contests where user is member OR has submissions
        (
            c.visibility = 'public'
            AND (
                EXISTS(
                    SELECT 1
                    FROM contest_members cm
                    WHERE cm.contest_id = c.id
                        AND cm.user_id = @created_by::uuid
                )
                OR EXISTS(
                    SELECT 1
                    FROM submissions sub
                    WHERE sub.contest_id = c.id
                        AND sub.created_by = @created_by::uuid
                )
            )
        )
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
        WHEN sqlc.narg('sort_by')::text = 'last_submission_time'
        AND sqlc.narg('sort_order')::text = 'desc' THEN MAX(s.created_at)
    END DESC NULLS LAST,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'last_submission_time'
        AND sqlc.narg('sort_order')::text = 'asc' THEN MAX(s.created_at)
    END NULLS LAST,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text IS NULL
        OR sqlc.narg('sort_by') = '' THEN MAX(s.created_at)
    END DESC NULLS LAST
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
-- name: CountUserContests :one
SELECT COUNT(DISTINCT c.id)
FROM contests c
WHERE (
        -- Private contests where user is member
        (
            c.visibility = 'private'
            AND EXISTS(
                SELECT 1
                FROM contest_members cm
                WHERE cm.contest_id = c.id
                    AND cm.user_id = @user_id::uuid
            )
        )
        OR -- Public contests where user is member OR has submissions
        (
            c.visibility = 'public'
            AND (
                EXISTS(
                    SELECT 1
                    FROM contest_members cm
                    WHERE cm.contest_id = c.id
                        AND cm.user_id = @user_id::uuid
                )
                OR EXISTS(
                    SELECT 1
                    FROM submissions sub
                    WHERE sub.contest_id = c.id
                        AND sub.created_by = @user_id::uuid
                )
            )
        )
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
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.created_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'created_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.created_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.updated_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'updated_at'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.updated_at
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'desc' THEN c.title
    END DESC,
    CASE
        WHEN sqlc.narg('sort_by')::text = 'title'
        AND sqlc.narg('sort_order')::text = 'asc' THEN c.title
    END,
    CASE
        WHEN sqlc.narg('sort_by')::text IS NULL
        OR sqlc.narg('sort_by') = '' THEN c.created_at
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
VALUES (
        @problem_id::uuid,
        @contest_id::uuid,
        COALESCE(
            (
                SELECT MAX(position)
                FROM contest_problem
                WHERE contest_id = @contest_id::uuid
            ),
            0
        ) + 1
    );
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
DELETE FROM contest_problem
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
WHERE user_id = @user_id::uuid
    AND contest_id = @contest_id::uuid;
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