SELECT s.id,
    s.user_id,
    u.username,
    s.solution,
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
WHERE s.id = $1