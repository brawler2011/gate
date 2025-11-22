UPDATE contest_members
SET role = $3
WHERE contest_id = $1
  AND user_id = $2

