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
WHERE cp.contest_id = $1
  AND cp.problem_id = $2