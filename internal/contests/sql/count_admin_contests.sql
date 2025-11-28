SELECT COUNT(*)
FROM contests c
WHERE (
    $1::text IS NULL
        OR $1 = ''
        OR (
        CASE
            WHEN LENGTH($1) < 3 THEN c.title ILIKE '%' || $1 || '%'
            ELSE word_similarity(c.title, $1) > 0.1
            END
        )
    )
  AND (
    $2::text IS NULL
        OR c.visibility = $2::contest_visibility
    )

