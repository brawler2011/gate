SELECT COUNT(DISTINCT c.id)
FROM contests c
INNER JOIN contest_members cm ON cm.contest_id = c.id
WHERE cm.user_id = $1
  AND cm.role IN ('owner', 'moderator')
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

