SELECT p.id,
       p.title,
       p.memory_limit,
       p.time_limit,
       p.created_at,
       p.updated_at
FROM problems p
WHERE (
    (
        $1::uuid IS NULL
            AND p.visibility = 'public'
        )
        OR (
        $1::uuid IS NOT NULL
            AND EXISTS (SELECT 1
                        FROM problem_members m
                        WHERE m.problem_id = p.id
                          AND m.user_id = $1
                          AND m.role in ('owner', 'moderator'))
        )
    )
  AND (
    $2::text IS NULL
        OR $2 = ''
        OR (
        CASE
            WHEN LENGTH($2) < 3 THEN p.title ILIKE '%' || $2 || '%'
            ELSE word_similarity(p.title, $2) > 0.1
            END
        )
    )

ORDER BY CASE
             WHEN $2::text IS NOT NULL
                 AND $2 != ''
                 AND LENGTH($2) >= 3 THEN word_similarity(p.title, $2)
             END DESC NULLS LAST,
         CASE
             WHEN $3::int < 0 THEN p.created_at
             END DESC,
         CASE
             WHEN $3::int >= 0 THEN p.created_at
             END
LIMIT $4 OFFSET $5
