-- Visible means public or granted (see access.ProblemAllows and friends); the null empty-page sentinel preserves totals.

-- name: ScopeProblems :many
WITH visible AS (
  SELECT p.*, o.login AS org_login, o.name AS org_name FROM problem_catalog p JOIN organizations o ON o.id = p.organization_id
  LEFT JOIN problem_access a ON a.problem_id = p.id AND a.user_id = sqlc.arg('viewer_id')::uuid
  WHERE (p.visibility = 'public' OR a.role IS NOT NULL OR sqlc.arg('all')::boolean)
    AND (sqlc.narg('organization_id')::uuid IS NULL OR p.organization_id = sqlc.narg('organization_id')::uuid)
    AND (sqlc.arg('search')::text = '' OR p.title ILIKE '%' || sqlc.arg('search')::text || '%')
    AND (sqlc.arg('visibility')::text = '' OR p.visibility::text = sqlc.arg('visibility')::text)
    AND (sqlc.narg('team_id')::uuid IS NULL OR EXISTS (SELECT 1 FROM problem_teams link WHERE link.problem_id = p.id AND link.team_id = sqlc.narg('team_id')::uuid))
    AND (sqlc.narg('owner_id')::uuid IS NULL OR EXISTS (SELECT 1 FROM problem_members own WHERE own.problem_id = p.id AND own.user_id = sqlc.narg('owner_id')::uuid AND own.role = 'owner'))
    AND (sqlc.narg('is_template')::boolean IS NULL OR p.is_template = sqlc.narg('is_template')::boolean)
), total AS (SELECT COUNT(*) AS total_items FROM visible), page AS (
  SELECT * FROM visible p ORDER BY CASE WHEN sqlc.arg('descending')::boolean THEN p.created_at END DESC,
    CASE WHEN NOT sqlc.arg('descending')::boolean THEN p.created_at END ASC,
    CASE WHEN sqlc.arg('descending')::boolean THEN p.id END DESC,
    CASE WHEN NOT sqlc.arg('descending')::boolean THEN p.id END ASC
  LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint
)
SELECT total.total_items, to_jsonb(page) AS resource
FROM total LEFT JOIN page ON TRUE
ORDER BY CASE WHEN sqlc.arg('descending')::boolean THEN page.created_at END DESC,
    CASE WHEN NOT sqlc.arg('descending')::boolean THEN page.created_at END ASC,
    CASE WHEN sqlc.arg('descending')::boolean THEN page.id END DESC,
    CASE WHEN NOT sqlc.arg('descending')::boolean THEN page.id END ASC;

-- name: ScopeContests :many
WITH visible AS (
  SELECT c.*, o.login AS org_login FROM contests c JOIN organizations o ON o.id = c.organization_id
  LEFT JOIN contest_access a ON a.contest_id = c.id AND a.user_id = sqlc.arg('viewer_id')::uuid
  WHERE (c.visibility = 'public' OR a.role IS NOT NULL OR sqlc.arg('all')::boolean)
    AND (sqlc.narg('organization_id')::uuid IS NULL OR c.organization_id = sqlc.narg('organization_id')::uuid)
    AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
    AND (sqlc.arg('visibility')::text = '' OR c.visibility::text = sqlc.arg('visibility')::text)
    AND (sqlc.narg('team_id')::uuid IS NULL OR EXISTS (SELECT 1 FROM contest_teams link WHERE link.contest_id = c.id AND link.team_id = sqlc.narg('team_id')::uuid))
), total AS (SELECT COUNT(*) AS total_items FROM visible), page AS (
  SELECT * FROM visible c ORDER BY CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text = 'asc' THEN c.created_at END ASC,
    CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text <> 'asc' THEN c.created_at END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text = 'asc' THEN c.updated_at END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.updated_at END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text = 'asc' THEN c.title END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text <> 'asc' THEN c.title END DESC, c.id DESC
  LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint
)
SELECT total.total_items, to_jsonb(page) AS resource
FROM total LEFT JOIN page ON TRUE
ORDER BY CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text = 'asc' THEN page.created_at END ASC,
    CASE WHEN sqlc.arg('sort_by')::text IN ('', 'created_at') AND sqlc.arg('sort_order')::text <> 'asc' THEN page.created_at END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text = 'asc' THEN page.updated_at END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'updated_at' AND sqlc.arg('sort_order')::text <> 'asc' THEN page.updated_at END DESC,
    CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text = 'asc' THEN page.title END ASC,
    CASE WHEN sqlc.arg('sort_by')::text = 'title' AND sqlc.arg('sort_order')::text <> 'asc' THEN page.title END DESC, page.id DESC;

-- name: ScopeTeams :many
WITH visible AS (
  SELECT t.* FROM teams t
  LEFT JOIN team_access a ON a.team_id = t.id AND a.user_id = sqlc.arg('viewer_id')::uuid
  WHERE (sqlc.arg('viewer_id')::uuid <> '00000000-0000-0000-0000-000000000000'::uuid AND (t.privacy = 'closed' OR a.role IS NOT NULL) OR sqlc.arg('all')::boolean)
    AND (sqlc.narg('organization_id')::uuid IS NULL OR t.organization_id = sqlc.narg('organization_id')::uuid)
    AND (sqlc.arg('search')::text = '' OR t.name ILIKE '%' || sqlc.arg('search')::text || '%')
), total AS (SELECT COUNT(*) AS total_items FROM visible), page AS (
  SELECT * FROM visible t ORDER BY t.created_at DESC, t.id DESC
  LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint
)
SELECT total.total_items, to_jsonb(page) AS resource
FROM total LEFT JOIN page ON TRUE
ORDER BY page.created_at DESC, page.id DESC;
