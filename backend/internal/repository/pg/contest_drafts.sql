-- name: CreateDraft :one
INSERT INTO contest_drafts (
    contest_id,
    user_id,
    problem_id,
    language,
    code
) VALUES (
    @contest_id::uuid,
    @user_id::uuid,
    @problem_id::uuid,
    @language,
    @code
)
RETURNING id;

-- name: GetDraft :one
SELECT cd.id,
    cd.contest_id,
    cd.user_id,
    u.username,
    cd.problem_id,
    p.title AS problem_title,
    cp.ordinal AS position,
    cd.language,
    cd.code,
    cd.created_at,
    cd.updated_at
FROM contest_drafts cd
    LEFT JOIN users u ON cd.user_id = u.id
    LEFT JOIN problems p ON cd.problem_id = p.id
    LEFT JOIN contest_problems cp ON cp.contest_id = cd.contest_id AND cp.problem_id = cd.problem_id
WHERE cd.id = @id::uuid;

-- name: GetDraftsCountByProblem :one
SELECT COUNT(*)
FROM contest_drafts
WHERE contest_id = @contest_id::uuid
  AND user_id = @user_id::uuid
  AND problem_id = @problem_id::uuid;

-- name: ListDrafts :many
SELECT cd.id,
    cd.contest_id,
    cd.user_id,
    u.username,
    cd.problem_id,
    p.title AS problem_title,
    cp.ordinal AS position,
    cd.language,
    cd.code,
    cd.created_at,
    cd.updated_at
FROM contest_drafts cd
    LEFT JOIN users u ON cd.user_id = u.id
    LEFT JOIN problems p ON cd.problem_id = p.id
    LEFT JOIN contest_problems cp ON cp.contest_id = cd.contest_id AND cp.problem_id = cd.problem_id
WHERE cd.contest_id = @contest_id::uuid
  AND (sqlc.narg('user_id')::uuid IS NULL OR cd.user_id = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('problem_id')::uuid IS NULL OR cd.problem_id = sqlc.narg('problem_id')::uuid)
ORDER BY cd.created_at DESC
LIMIT @limit_val OFFSET @offset_val;

-- name: CountDrafts :one
SELECT COUNT(*)
FROM contest_drafts cd
WHERE cd.contest_id = @contest_id::uuid
  AND (sqlc.narg('user_id')::uuid IS NULL OR cd.user_id = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('problem_id')::uuid IS NULL OR cd.problem_id = sqlc.narg('problem_id')::uuid);

-- name: DeleteDraft :exec
DELETE FROM contest_drafts
WHERE id = @id::uuid;
