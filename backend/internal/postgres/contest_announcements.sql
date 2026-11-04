-- name: CreateContestAnnouncement :one
INSERT INTO contest_announcements (
    id,
    contest_id,
    problem_id,
    author_id,
    title,
    body,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    NOW(),
    NOW()
) RETURNING *;

-- name: GetContestAnnouncementByID :one
SELECT 
    ca.*,
    u.username AS author_username,
    p.title AS problem_title,
    cp.ordinal AS problem_ordinal
FROM contest_announcements ca
JOIN users u ON ca.author_id = u.id
LEFT JOIN problems p ON ca.problem_id = p.id
LEFT JOIN contest_problems cp ON cp.contest_id = ca.contest_id AND cp.problem_id = ca.problem_id
WHERE ca.id = $1;

-- name: ListContestAnnouncements :many
SELECT 
    ca.*,
    u.username AS author_username,
    p.title AS problem_title,
    cp.ordinal AS problem_ordinal
FROM contest_announcements ca
JOIN users u ON ca.author_id = u.id
LEFT JOIN problems p ON ca.problem_id = p.id
LEFT JOIN contest_problems cp ON cp.contest_id = ca.contest_id AND cp.problem_id = ca.problem_id
WHERE ca.contest_id = $1
ORDER BY ca.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountContestAnnouncements :one
SELECT COUNT(*) FROM contest_announcements
WHERE contest_id = $1;

-- name: DeleteContestAnnouncement :exec
DELETE FROM contest_announcements
WHERE id = $1 AND contest_id = $2;
