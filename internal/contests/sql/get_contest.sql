SELECT
    id,
    title,
    description,
    visibility,
    monitor_scope,
    submissions_list_scope,
    submissions_review_scope,
    created_by,
    created_at,
    updated_at
from contests
WHERE id = $1
