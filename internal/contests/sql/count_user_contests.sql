SELECT COUNT(DISTINCT c.id)
FROM contests c
WHERE (
    -- Private contests where user is member
    (c.visibility = 'private' AND EXISTS(SELECT 1 FROM contest_members WHERE contest_id = c.id AND user_id = $1))
    OR
    -- Public contests where user is member OR has submissions
    (c.visibility = 'public' AND (
        EXISTS(SELECT 1 FROM contest_members WHERE contest_id = c.id AND user_id = $1)
        OR EXISTS(SELECT 1 FROM submissions WHERE contest_id = c.id AND created_by = $1)
    ))
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

