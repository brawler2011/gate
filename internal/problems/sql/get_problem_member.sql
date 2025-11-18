select problem_id,
    user_id,
    role
from problem_members
where problem_id = $1
    and user_id = $2