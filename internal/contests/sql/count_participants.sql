SELECT COUNT(*)
FROM contest_members
WHERE contest_id = $1 AND role = 'participant'