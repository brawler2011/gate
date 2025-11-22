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
ORDER BY CASE
             WHEN $2::text IS NOT NULL
                 AND $2 != ''
                 AND LENGTH($2) >= 3 THEN word_similarity(c.title, $2)
             END DESC NULLS LAST,
         CASE
             WHEN $3::text = 'created_at' AND $4::text = 'desc' THEN c.created_at
             END DESC,
         CASE
             WHEN $3::text = 'created_at' AND $4::text = 'asc' THEN c.created_at
             END,
         CASE
             WHEN $3::text = 'updated_at' AND $4::text = 'desc' THEN c.updated_at
             END DESC,
         CASE
             WHEN $3::text = 'updated_at' AND $4::text = 'asc' THEN c.updated_at
             END,
         CASE
             WHEN $3::text = 'title' AND $4::text = 'desc' THEN c.title
             END DESC,
         CASE
             WHEN $3::text = 'title' AND $4::text = 'asc' THEN c.title
             END,
         CASE
             WHEN $3::text IS NULL OR $3 = '' THEN c.created_at
             END DESC
LIMIT $5 OFFSET $6

