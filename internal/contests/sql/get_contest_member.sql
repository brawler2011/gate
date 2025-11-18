select contest_id,
       user_id,
       role
from contest_members
where contest_id = $1
  and user_id = $2
