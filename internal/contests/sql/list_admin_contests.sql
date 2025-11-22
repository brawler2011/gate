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
    $3::text IS NULL
        OR $3 = ''
        OR (
        CASE
            WHEN LENGTH($3) < 3 THEN c.title ILIKE '%' || $3 || '%'
            ELSE word_similarity(c.title, $3) > 0.1
            END
        )
    )
  AND (
    $4::text IS NULL
        OR c.visibility = $4
    )
ORDER BY CASE
             WHEN $3::text IS NOT NULL
                 AND $3 != ''
                 AND LENGTH($3) >= 3 THEN word_similarity(c.title, $3)
             END DESC NULLS LAST,
         CASE
             WHEN $5::text = 'created_at' AND $6::text = 'desc' THEN c.created_at
             END DESC,
         CASE
             WHEN $5::text = 'created_at' AND $6::text = 'asc' THEN c.created_at
             END,
         CASE
             WHEN $5::text = 'updated_at' AND $6::text = 'desc' THEN c.updated_at
             END DESC,
         CASE
             WHEN $5::text = 'updated_at' AND $6::text = 'asc' THEN c.updated_at
             END,
         CASE
             WHEN $5::text = 'title' AND $6::text = 'desc' THEN c.title
             END DESC,
         CASE
             WHEN $5::text = 'title' AND $6::text = 'asc' THEN c.title
             END,
         CASE
             WHEN $5::text IS NULL OR $5 = '' THEN c.created_at
             END DESC
LIMIT $1 OFFSET $2

