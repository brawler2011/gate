SELECT c.id,
       c.title,
       c.description,
       c.visibility,
       c.monitor_scope,
       c.submissions_list_scope,
       c.submissions_review_scope,
       c.created_by,
       c.created_at,
       c.updated_at
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
ORDER BY CASE
             WHEN $2::text IS NOT NULL
                 AND $2 != ''
                 AND LENGTH($2) >= 3 THEN word_similarity(c.title, $2)
             END DESC NULLS LAST,
         CASE
             WHEN $3::bool = true THEN c.created_at
             END DESC,
         CASE
             WHEN $3::bool = false THEN c.created_at
             END
LIMIT $4 OFFSET $5