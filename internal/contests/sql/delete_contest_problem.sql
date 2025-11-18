DELETE
FROM contest_problem
WHERE contest_id = $1
  AND problem_id = $2