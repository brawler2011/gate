SELECT *
FROM users
WHERE (
        $1::text IS NULL
        OR $1 = ''
        OR word_similarity(username, $1) > 0.1
    )
    AND (
        $2::text IS NULL
        OR $2 = ''
        OR role::text = $2
    )
ORDER BY CASE
        WHEN $1::text IS NOT NULL
        AND $1 != '' THEN word_similarity(username, $1)
    END DESC NULLS LAST,
    created_at DESC
LIMIT $3 OFFSET $4