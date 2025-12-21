-- Problem CRUD operations
-- name: CreateProblem :one
INSERT INTO problems (id, title, created_by)
VALUES (@id::uuid, @title, @created_by::uuid)
RETURNING id;
-- name: GetProblemById :one
SELECT *
FROM problems
WHERE id = @id::uuid
LIMIT 1;
-- name: UpdateProblem :exec
UPDATE problems
SET title = COALESCE(sqlc.narg('title'), title),
    time_limit = COALESCE(sqlc.narg('time_limit'), time_limit),
    memory_limit = COALESCE(sqlc.narg('memory_limit'), memory_limit),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    legend = COALESCE(sqlc.narg('legend'), legend),
    input_format = COALESCE(sqlc.narg('input_format'), input_format),
    output_format = COALESCE(sqlc.narg('output_format'), output_format),
    notes = COALESCE(sqlc.narg('notes'), notes),
    scoring = COALESCE(sqlc.narg('scoring'), scoring),
    legend_html = COALESCE(sqlc.narg('legend_html'), legend_html),
    input_format_html = COALESCE(
        sqlc.narg('input_format_html'),
        input_format_html
    ),
    output_format_html = COALESCE(
        sqlc.narg('output_format_html'),
        output_format_html
    ),
    notes_html = COALESCE(sqlc.narg('notes_html'), notes_html),
    scoring_html = COALESCE(sqlc.narg('scoring_html'), scoring_html)
WHERE id = @id::uuid;
-- name: DeleteProblem :exec
DELETE FROM problems
WHERE id = @id::uuid;
-- Problem listing
-- name: ListProblems :many
SELECT p.id,
    p.title,
    p.memory_limit,
    p.time_limit,
    p.created_at,
    p.updated_at
FROM problems p
WHERE (
        (
            sqlc.narg('user_id')::uuid IS NULL
            AND p.visibility = 'public'
        )
        OR (
            sqlc.narg('user_id')::uuid IS NOT NULL
            AND EXISTS (
                SELECT 1
                FROM problem_members m
                WHERE m.problem_id = p.id
                    AND m.user_id = sqlc.narg('user_id')::uuid
                    AND m.role in ('owner', 'moderator')
            )
        )
    )
    AND (
        sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
            CASE
                WHEN LENGTH(sqlc.narg('search')) < 3 THEN p.title ILIKE '%' || sqlc.narg('search') || '%'
                ELSE word_similarity(p.title, sqlc.narg('search')) > 0.1
            END
        )
    )
ORDER BY CASE
        WHEN sqlc.narg('search')::text IS NOT NULL
        AND sqlc.narg('search') != ''
        AND LENGTH(sqlc.narg('search')) >= 3 THEN word_similarity(p.title, sqlc.narg('search'))
    END DESC NULLS LAST,
    CASE
        WHEN sqlc.narg('sort_order')::int < 0 THEN p.created_at
    END DESC,
    CASE
        WHEN sqlc.narg('sort_order')::int >= 0 THEN p.created_at
    END
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
-- name: CountProblems :one
SELECT COUNT(*)
FROM problems p
WHERE (
        (
            sqlc.narg('user_id')::uuid IS NULL
            AND p.visibility = 'public'
        )
        OR (
            sqlc.narg('user_id')::uuid IS NOT NULL
            AND EXISTS (
                SELECT 1
                FROM problem_members m
                WHERE m.problem_id = p.id
                    AND m.user_id = sqlc.narg('user_id')::uuid
                    AND m.role in ('owner', 'moderator')
            )
        )
    )
    AND (
        sqlc.narg('search')::text IS NULL
        OR sqlc.narg('search') = ''
        OR (
            CASE
                WHEN LENGTH(sqlc.narg('search')) < 3 THEN p.title ILIKE '%' || sqlc.narg('search') || '%'
                ELSE word_similarity(p.title, sqlc.narg('search')) > 0.1
            END
        )
    );
-- Problem member operations
-- name: CreateProblemMember :exec
INSERT INTO problem_members (problem_id, user_id, role)
VALUES (@problem_id::uuid, @user_id::uuid, @role);
-- name: GetProblemMember :one
SELECT problem_id,
    user_id,
    role
FROM problem_members
WHERE problem_id = @problem_id::uuid
    AND user_id = @user_id::uuid;
-- Problem tests operations
-- name: CreateProblemTest :exec
INSERT INTO problem_tests (problem_id, ordinal, input, output)
VALUES (@problem_id::uuid, @ordinal, @input, @output);
-- name: GetProblemTests :many
SELECT id,
    problem_id,
    ordinal,
    input,
    output,
    created_at
FROM problem_tests
WHERE problem_id = @problem_id::uuid
ORDER BY ordinal ASC;
-- name: DeleteProblemTests :exec
DELETE FROM problem_tests
WHERE problem_id = @problem_id::uuid;