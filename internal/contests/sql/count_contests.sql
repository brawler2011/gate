SELECT COUNT(*)
FROM contests c
WHERE (
    (
        $1::uuid IS NULL
            AND c.visibility = 'public'
        )
        OR (
        $1::uuid IS NOT NULL
            AND EXISTS (SELECT 1
                        FROM contest_members p
                        WHERE p.contest_id = c.id
                          AND p.user_id = $1
                          AND p.role = 'owner')
        )
    )
  AND (
    $2::text IS NULL
        OR $2 = ''
        OR (
        CASE
            WHEN LENGTH($2) < 3 THEN c.title ILIKE '%' || $2 || '%'
            ELSE word_similarity(c.title, $2) > 0.1
            END
        )
    )