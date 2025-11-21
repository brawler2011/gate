SELECT u.id as user_id,
    cm.contest_id,
    u.username,
    u.role role,
    cm.role contest_role,
    u.kratos_id,
    u.created_at,
    u.updated_at
FROM contest_members cm
    LEFT JOIN users u ON cm.user_id = u.id
WHERE contest_id = $1
LIMIT $2 OFFSET $3