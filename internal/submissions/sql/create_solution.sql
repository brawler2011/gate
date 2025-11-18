INSERT INTO solutions (
        contest_id,
        problem_id,
        user_id,
        solution,
        language,
        penalty
    )
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id