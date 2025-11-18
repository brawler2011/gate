SELECT cp.problem_id,
       p.title,
       p.time_limit,
       p.memory_limit,
       cp.position,
       p.created_at,
       p.updated_at
FROM contest_problem cp
         LEFT JOIN problems p ON cp.problem_id = p.id
WHERE cp.contest_id = $1
ORDER BY cp.position