package authz.contest

import future.keywords.if
import data.authz.common

default allow = false

# Resolve contest and member
contest := input.contest if input.contest
else := custom.get_contest(input.contest_id)

member := input.member if input.member
else := custom.get_contest_member(input.contest_id, input.user_id)

# Role weights matching Go model
role_weight("owner") = 3
role_weight("moderator") = 2
role_weight("participant") = 1
role_weight(_) = 0

# Helper: r1 >= r2
role_gte(r1, r2) if {
	role_weight(r1) >= role_weight(r2)
}

# Context helpers
is_admin if common.is_admin
is_owner if member.Role == "owner"
is_moderator if member.Role == "moderator"
is_participant if member.Role == "participant"

# Map of all permissions for the current context
permissions = {
	"UpdateContest": can_update_contest,
	"AdminContest": can_admin_contest,
	"GetMonitor": can_get_monitor,
	"ListUsersSubmissions": can_list_users_submissions,
	"ListOwnSubmissions": can_list_own_submissions,
	"GetOtherUserSubmission": can_get_other_user_submission,
	"GetOwnSubmission": can_get_own_submission,
	"CreateSubmission": can_create_submission,
}

# Single action check
allow if permissions[input.action]

# --- Rules ---

has_full_access if is_admin
has_full_access if is_owner

# UpdateContest
can_update_contest if has_full_access
can_update_contest if is_moderator

# AdminContest
can_admin_contest if has_full_access

# GetMonitor
can_get_monitor if has_full_access
can_get_monitor if role_gte(member.Role, contest.MonitorScope)

# ListUsersSubmissions
can_list_users_submissions if has_full_access
can_list_users_submissions if role_gte(member.Role, contest.SubmissionsListScope)

# ListOwnSubmissions
can_list_own_submissions if has_full_access
can_list_own_submissions if member # Any member

# GetOtherUserSubmission
can_get_other_user_submission if has_full_access
can_get_other_user_submission if role_gte(member.Role, contest.SubmissionsReviewScope)

# GetOwnSubmission
can_get_own_submission if has_full_access
can_get_own_submission if member

# CreateSubmission
can_create_submission if has_full_access
can_create_submission if member
