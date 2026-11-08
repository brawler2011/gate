-- Bounded batch reads for metadata policies and future capability projections.

-- name: LoadProblemFacts :many
SELECT p.id, jsonb_build_object('ID', p.id, 'Organization', jsonb_build_object('ID', o.id, 'Role', COALESCE(om.role::text, ''), 'JoinPolicy', o.join_policy), 'OwnerID', COALESCE(p.owner_id, '00000000-0000-0000-0000-000000000000'::uuid),
    'Visibility', p.visibility, 'DirectRole', COALESCE(m.role::text, ''),
    'Teams', COALESCE((SELECT jsonb_agg(jsonb_build_object(
      'OrganizationID', t.organization_id, 'Membership', tm.role, 'Active', true, 'Role', link.permission))
      FROM problem_teams link JOIN teams t ON t.id = link.team_id
      JOIN team_members tm ON tm.team_id = t.id
      WHERE link.problem_id = p.id AND tm.user_id = sqlc.narg('viewer_id')::uuid), '[]'::jsonb)) AS facts
FROM problems p JOIN organizations o ON o.id = p.organization_id
LEFT JOIN organization_members om ON om.organization_id = o.id AND om.user_id = sqlc.narg('viewer_id')::uuid
LEFT JOIN problem_members m ON m.problem_id = p.id AND m.user_id = sqlc.narg('viewer_id')::uuid
WHERE p.id = ANY(sqlc.arg('ids')::uuid[]);

-- name: LoadContestFacts :many
SELECT c.id, jsonb_build_object('ID', c.id, 'StartTime', c.start_time, 'EndTime', c.end_time, 'Settings', c.settings, 'Organization', jsonb_build_object('ID', o.id, 'Role', COALESCE(om.role::text, ''), 'JoinPolicy', o.join_policy), 'OwnerID', COALESCE(c.owner_id, '00000000-0000-0000-0000-000000000000'::uuid),
    'Visibility', c.visibility, 'DirectRole', COALESCE(m.role::text, ''),
    'Teams', COALESCE((SELECT jsonb_agg(jsonb_build_object(
      'OrganizationID', t.organization_id, 'Membership', tm.role, 'Active', true, 'Role', link.role))
      FROM contest_teams link JOIN teams t ON t.id = link.team_id
      JOIN team_members tm ON tm.team_id = t.id
      WHERE link.contest_id = c.id AND tm.user_id = sqlc.narg('viewer_id')::uuid), '[]'::jsonb), 'ParticipationMode', COALESCE(NULLIF(c.settings->>'participation_mode', ''), 'open')) AS facts
FROM contests c JOIN organizations o ON o.id = c.organization_id
LEFT JOIN organization_members om ON om.organization_id = o.id AND om.user_id = sqlc.narg('viewer_id')::uuid
LEFT JOIN contest_members m ON m.contest_id = c.id AND m.user_id = sqlc.narg('viewer_id')::uuid
WHERE c.id = ANY(sqlc.arg('ids')::uuid[]);

-- name: LoadTeamFacts :many
SELECT t.id, jsonb_build_object('ID', t.id, 'Organization', jsonb_build_object('ID', o.id, 'Role', COALESCE(om.role::text, ''), 'JoinPolicy', o.join_policy), 'Role', COALESCE(tm.role::text, ''), 'Privacy', t.privacy) AS facts
FROM teams t JOIN organizations o ON o.id = t.organization_id
LEFT JOIN organization_members om ON om.organization_id = o.id AND om.user_id = sqlc.narg('viewer_id')::uuid
LEFT JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = sqlc.narg('viewer_id')::uuid
WHERE t.id = ANY(sqlc.arg('ids')::uuid[]);
