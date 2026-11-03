-- name: CreateContestClarification :one
INSERT INTO contest_clarifications (
    id,
    contest_id,
    problem_id,
    user_id,
    question,
    status,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    'pending',
    NOW(),
    NOW()
) RETURNING *;

-- name: GetContestClarificationByID :one
SELECT 
    cc.*,
    u.username AS username,
    au.username AS answered_by_username,
    p.title AS problem_title,
    cp.ordinal AS problem_ordinal
FROM contest_clarifications cc
JOIN users u ON cc.user_id = u.id
LEFT JOIN users au ON cc.answered_by = au.id
LEFT JOIN problems p ON cc.problem_id = p.id
LEFT JOIN contest_problems cp ON cp.contest_id = cc.contest_id AND cp.problem_id = cc.problem_id
WHERE cc.id = $1;

-- name: ListContestClarificationsForUser :many
SELECT 
    cc.*,
    u.username AS username,
    au.username AS answered_by_username,
    p.title AS problem_title,
    cp.ordinal AS problem_ordinal
FROM contest_clarifications cc
JOIN users u ON cc.user_id = u.id
LEFT JOIN users au ON cc.answered_by = au.id
LEFT JOIN problems p ON cc.problem_id = p.id
LEFT JOIN contest_problems cp ON cp.contest_id = cc.contest_id AND cp.problem_id = cc.problem_id
WHERE cc.contest_id = $1 AND cc.user_id = $2
ORDER BY cc.created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountContestClarificationsForUser :one
SELECT COUNT(*) FROM contest_clarifications
WHERE contest_id = $1 AND user_id = $2;

-- name: ListContestClarificationsForModerator :many
SELECT 
    cc.*,
    u.username AS username,
    au.username AS answered_by_username,
    p.title AS problem_title,
    cp.ordinal AS problem_ordinal
FROM contest_clarifications cc
JOIN users u ON cc.user_id = u.id
LEFT JOIN users au ON cc.answered_by = au.id
LEFT JOIN problems p ON cc.problem_id = p.id
LEFT JOIN contest_problems cp ON cp.contest_id = cc.contest_id AND cp.problem_id = cc.problem_id
WHERE cc.contest_id = $1
  AND (sqlc.narg('problem_id')::uuid IS NULL OR cc.problem_id = sqlc.narg('problem_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR sqlc.narg('status')::text = '' OR cc.status = sqlc.narg('status')::text)
ORDER BY cc.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountContestClarificationsForModerator :one
SELECT COUNT(*) FROM contest_clarifications
WHERE contest_id = $1
  AND (sqlc.narg('problem_id')::uuid IS NULL OR problem_id = sqlc.narg('problem_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR sqlc.narg('status')::text = '' OR status = sqlc.narg('status')::text);

-- name: AnswerContestClarification :one
UPDATE contest_clarifications
SET 
    answer = $1,
    answered_by = $2,
    status = 'answered',
    answered_at = NOW(),
    updated_at = NOW()
WHERE id = $3 AND contest_id = $4
RETURNING *;
