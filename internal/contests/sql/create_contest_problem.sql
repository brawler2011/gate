INSERT INTO contest_problem (problem_id, contest_id, position)
VALUES ($1,
        $2,
        COALESCE(
                (SELECT MAX(position)
                 FROM contest_problem
                 WHERE contest_id = $2),
                0
        ) + 1)