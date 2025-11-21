INSERT INTO submissions (
        contest_id,
        problem_id,
        created_by,
        submission,
        language,
        penalty
    )
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id