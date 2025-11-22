SELECT id, problem_id, ordinal, input, output, created_at
FROM problem_tests
WHERE problem_id = $1
ORDER BY ordinal ASC;

