UPDATE contests
SET title                    = COALESCE($2, title),
    description              = COALESCE($3, description),
    visibility               = COALESCE($4, visibility),
    monitor_scope            = COALESCE($5, monitor_scope),
    submissions_list_scope   = COALESCE($6, submissions_list_scope),
    submissions_review_scope = COALESCE($7, submissions_review_scope)
WHERE id = $1
