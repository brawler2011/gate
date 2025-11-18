SELECT COUNT(*)
FROM solutions s
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

