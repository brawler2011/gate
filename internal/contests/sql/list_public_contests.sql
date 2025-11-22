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
WHERE c.visibility = 'public'
  AND (
    $1::text IS NULL
        OR $1 = ''
        OR (
        CASE
            WHEN LENGTH($1) < 3 THEN c.title ILIKE '%' || $1 || '%'
            ELSE word_similarity(c.title, $1) > 0.1
            END
        )
    )
ORDER BY CASE
             WHEN $1::text IS NOT NULL
                 AND $1 != ''
                 AND LENGTH($1) >= 3 THEN word_similarity(c.title, $1)
             END DESC NULLS LAST,
         CASE
             WHEN $2::text = 'created_at' AND $3::text = 'desc' THEN c.created_at
             END DESC,
         CASE
             WHEN $2::text = 'created_at' AND $3::text = 'asc' THEN c.created_at
             END,
         CASE
             WHEN $2::text = 'updated_at' AND $3::text = 'desc' THEN c.updated_at
             END DESC,
         CASE
             WHEN $2::text = 'updated_at' AND $3::text = 'asc' THEN c.updated_at
             END,
         CASE
             WHEN $2::text = 'title' AND $3::text = 'desc' THEN c.title
             END DESC,
         CASE
             WHEN $2::text = 'title' AND $3::text = 'asc' THEN c.title
             END,
         CASE
             WHEN $2::text IS NULL OR $2 = '' THEN c.created_at
             END DESC
LIMIT $4 OFFSET $5

