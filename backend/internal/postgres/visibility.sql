-- Metadata scopes use EXISTS grants so resources occur exactly once.
-- A nullable JSON row is the empty-page sentinel, preserving the total.

-- name: ScopeProblems :many
WITH actor AS (
  SELECT u.id, u.role FROM users u
  WHERE u.id = sqlc.narg('viewer_id')::uuid
    AND u.role IN ('user', 'admin')
    AND (u.expires_at IS NULL OR u.expires_at > sqlc.arg('now')::timestamptz)
), visible AS (
  SELECT p.*, o.login AS org_login, o.name AS org_name FROM problem_catalog p JOIN organizations o ON o.id = p.organization_id
  WHERE NOT EXISTS (SELECT 1 FROM organization_members invalid WHERE invalid.organization_id = p.organization_id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('member', 'admin', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM problem_members invalid WHERE invalid.problem_id = p.id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('viewer', 'moderator', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM problem_teams link JOIN team_members tm ON tm.team_id = link.team_id WHERE link.problem_id = p.id AND tm.user_id = sqlc.narg('viewer_id')::uuid AND (tm.role::text NOT IN ('member', 'maintainer') OR link.permission::text NOT IN ('read', 'write', 'admin')))
    AND (p.visibility = 'public' OR EXISTS (
    SELECT 1 FROM actor a WHERE a.role = 'admin' OR p.owner_id = a.id
    OR EXISTS (SELECT 1 FROM organization_members om WHERE om.organization_id = p.organization_id AND om.user_id = a.id AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM problem_members m WHERE m.problem_id = p.id AND m.user_id = a.id AND m.role IN ('viewer', 'moderator', 'owner'))
    OR EXISTS (SELECT 1 FROM problem_teams pt
      JOIN teams t ON t.id = pt.team_id JOIN team_members tm ON tm.team_id = t.id
      WHERE pt.problem_id = p.id AND tm.user_id = a.id
        AND t.organization_id = p.organization_id AND tm.role IN ('member', 'maintainer')
        AND pt.permission IN ('read', 'write', 'admin'))))
    AND (sqlc.narg('viewer_id')::uuid IS NULL OR sqlc.narg('viewer_id')::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR EXISTS (SELECT 1 FROM actor))
    AND (sqlc.narg('organization_id')::uuid IS NULL OR p.organization_id = sqlc.narg('organization_id')::uuid)
    AND (sqlc.arg('search')::text = '' OR p.title ILIKE '%' || sqlc.arg('search')::text || '%')
    AND p.visibility IN ('public', 'private')
    AND (sqlc.arg('visibility')::text = '' OR p.visibility::text = sqlc.arg('visibility')::text)
    AND (sqlc.narg('team_id')::uuid IS NULL OR EXISTS (SELECT 1 FROM problem_teams link WHERE link.problem_id = p.id AND link.team_id = sqlc.narg('team_id')::uuid))
    AND (sqlc.narg('owner_id')::uuid IS NULL OR p.owner_id = sqlc.narg('owner_id')::uuid)
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
WITH actor AS (
  SELECT u.id, u.role FROM users u
  WHERE u.id = sqlc.narg('viewer_id')::uuid
    AND u.role IN ('user', 'admin')
    AND (u.expires_at IS NULL OR u.expires_at > sqlc.arg('now')::timestamptz)
), visible AS (
  SELECT c.*, o.login AS org_login, o.name AS org_name FROM contests c JOIN organizations o ON o.id = c.organization_id
  WHERE NOT EXISTS (SELECT 1 FROM organization_members invalid WHERE invalid.organization_id = c.organization_id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('member', 'admin', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM contest_members invalid WHERE invalid.contest_id = c.id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('participant', 'moderator', 'owner'))
    AND NOT EXISTS (SELECT 1 FROM contest_teams link JOIN team_members tm ON tm.team_id = link.team_id WHERE link.contest_id = c.id AND tm.user_id = sqlc.narg('viewer_id')::uuid AND (tm.role::text NOT IN ('member', 'maintainer') OR link.role::text NOT IN ('participant', 'moderator')))
    AND COALESCE(NULLIF(c.settings->>'participation_mode', ''), 'open') IN ('open', 'by_request', 'invite_only')
    AND (c.visibility = 'public' OR EXISTS (
    SELECT 1 FROM actor a WHERE a.role = 'admin' OR c.owner_id = a.id
    OR EXISTS (SELECT 1 FROM organization_members om WHERE om.organization_id = c.organization_id AND om.user_id = a.id AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM contest_members m WHERE m.contest_id = c.id AND m.user_id = a.id AND m.role IN ('participant', 'moderator', 'owner'))
    OR EXISTS (SELECT 1 FROM contest_teams ct
      JOIN teams t ON t.id = ct.team_id JOIN team_members tm ON tm.team_id = t.id
      WHERE ct.contest_id = c.id AND tm.user_id = a.id
        AND t.organization_id = c.organization_id AND tm.role IN ('member', 'maintainer')
        AND ct.role IN ('participant', 'moderator'))))
    AND (sqlc.narg('viewer_id')::uuid IS NULL OR sqlc.narg('viewer_id')::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR EXISTS (SELECT 1 FROM actor))
    AND (sqlc.narg('organization_id')::uuid IS NULL OR c.organization_id = sqlc.narg('organization_id')::uuid)
    AND (sqlc.arg('search')::text = '' OR c.title ILIKE '%' || sqlc.arg('search')::text || '%')
    AND c.visibility IN ('public', 'private')
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
WITH actor AS (
  SELECT u.id, u.role FROM users u
  WHERE u.id = sqlc.narg('viewer_id')::uuid
    AND u.role IN ('user', 'admin')
    AND (u.expires_at IS NULL OR u.expires_at > sqlc.arg('now')::timestamptz)
), visible AS (
  SELECT t.* FROM teams t
  WHERE NOT EXISTS (SELECT 1 FROM organization_members invalid WHERE invalid.organization_id = t.organization_id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('member', 'admin', 'owner'))
    AND t.privacy IN ('closed', 'secret')
    AND NOT EXISTS (SELECT 1 FROM team_members invalid WHERE invalid.team_id = t.id AND invalid.user_id = sqlc.narg('viewer_id')::uuid AND invalid.role::text NOT IN ('member', 'maintainer'))
    AND EXISTS (SELECT 1 FROM actor a WHERE t.privacy = 'closed'
    OR a.role = 'admin'
    OR EXISTS (SELECT 1 FROM organization_members om WHERE om.organization_id = t.organization_id AND om.user_id = a.id AND om.role IN ('owner', 'admin'))
    OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = a.id AND tm.role IN ('member', 'maintainer')))
    AND (sqlc.narg('viewer_id')::uuid IS NULL OR sqlc.narg('viewer_id')::uuid = '00000000-0000-0000-0000-000000000000'::uuid OR EXISTS (SELECT 1 FROM actor))
    AND (sqlc.narg('organization_id')::uuid IS NULL OR t.organization_id = sqlc.narg('organization_id')::uuid)
    AND (sqlc.arg('search')::text = '' OR t.name ILIKE '%' || sqlc.arg('search')::text || '%')
), total AS (SELECT COUNT(*) AS total_items FROM visible), page AS (
  SELECT * FROM visible t ORDER BY t.created_at DESC, t.id DESC
  LIMIT sqlc.arg('page_size')::int OFFSET sqlc.arg('page_offset')::bigint
)
SELECT total.total_items, to_jsonb(page) AS resource
FROM total LEFT JOIN page ON TRUE
ORDER BY page.created_at DESC, page.id DESC;
