SELECT s.id,
    s.user_id,
    u.username,
    s.state,
    s.score,
    s.penalty,
    s.time_stat,
    s.memory_stat,
    s.language,
    s.problem_id,
    p.title problem_title,
    cp.position,
    s.contest_id,
    c.title contest_title,
    s.updated_at,
    s.created_at
FROM solutions s
    LEFT JOIN users u ON s.user_id = u.id
    LEFT JOIN problems p ON s.problem_id = p.id
    LEFT JOIN contest_problem cp ON p.id = cp.problem_id
    AND cp.contest_id = s.contest_id
    LEFT JOIN contests c ON s.contest_id = c.id
WHERE (
        $1::uuid IS NULL
        OR s.contest_id = $1
    )
    AND (
        $2::uuid IS NULL
        OR s.user_id = $2
    )
    AND (
        $3::uuid IS NULL
        OR s.problem_id = $3
    )
    AND (
        $4::integer IS NULL
        OR s.language = $4
    )
    AND (
        $5::integer IS NULL
        OR s.state = $5
    )
ORDER BY CASE
        WHEN $6::int < 0 THEN s.id
        ELSE NULL
    END DESC,
    CASE
        WHEN $6::int >= 0 THEN s.id
        ELSE NULL
    END ASC
LIMIT $7 OFFSET $8

