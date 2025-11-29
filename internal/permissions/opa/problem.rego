package authz.problem

import future.keywords.if
import data.authz.common

default allow = false

# Resolve problem and member
problem := input.problem if input.problem
else := custom.get_problem(input.problem_id)

member := input.member if input.member
else := custom.get_problem_member(input.problem_id, input.user_id)

# Context helpers
is_admin if common.is_admin
is_owner if member.role == "owner"
is_moderator if member.role == "moderator"

has_full_access if is_admin
has_full_access if is_owner

permissions = {
	"GetProblem": can_get_problem,
	"UpdateProblem": can_update_problem,
	"AdminProblem": can_admin_problem,
}

allow if permissions[input.action]

# --- Rules ---

# AdminProblem
can_admin_problem if has_full_access

# UpdateProblem
can_update_problem if has_full_access
can_update_problem if is_moderator

# GetProblem
can_get_problem if can_update_problem
can_get_problem if problem.visibility == "public"
