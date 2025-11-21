package authz.contest

import future.keywords.if
import data.authz.common

default allow = false

# Resolve contest and member
contest := input.contest if {
	input.contest
} else := custom.get_contest(input.contest_id)

member := input.member if {
	input.member
} else := custom.get_contest_member(input.contest_id, input.user_id)

# Role weights matching Go model
role_weight(role) := 3 if role == "owner"
else := 2 if role == "moderator"
else := 1 if role == "participant"
else := 0

# Helper: r1 >= r2
role_gte(r1, r2) if {
	role_weight(r1) >= role_weight(r2)
}

# Context helpers
is_admin if common.is_admin
is_owner if {
	member
	member.Role == "owner"
}
is_moderator if {
	member
	member.Role == "moderator"
}
is_participant if {
	member
	member.Role == "participant"
}

# Map of all permissions for the current context
permissions = {
	"GetContest": can_get_contest,
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

# Default values
default can_get_contest := false
default can_update_contest := false
default can_admin_contest := false
default can_get_monitor := false
default can_list_users_submissions := false
default can_list_own_submissions := false
default can_get_other_user_submission := false
default can_get_own_submission := false
default can_create_submission := false

has_full_access if is_admin
has_full_access if is_owner

# GetContest - view contest page
can_get_contest := true if has_full_access
can_get_contest := true if is_participant
can_get_contest := true if is_moderator
can_get_contest := true if {
	contest
	contest.Visibility == "public"
}

# UpdateContest
can_update_contest := true if has_full_access
can_update_contest := true if is_moderator

# AdminContest
can_admin_contest := true if has_full_access

# GetMonitor
can_get_monitor := true if has_full_access
can_get_monitor := true if {
	member
	member.Role
	contest
	contest.MonitorScope
	role_gte(member.Role, contest.MonitorScope)
}

# ListUsersSubmissions
can_list_users_submissions := true if has_full_access
can_list_users_submissions := true if {
	member
	member.Role
	contest
	contest.SubmissionsListScope
	role_gte(member.Role, contest.SubmissionsListScope)
}

# ListOwnSubmissions
can_list_own_submissions := true if has_full_access
can_list_own_submissions := true if is_participant
can_list_own_submissions := true if is_moderator

# GetOtherUserSubmission
can_get_other_user_submission := true if has_full_access
can_get_other_user_submission := true if {
	member
	member.Role
	contest
	contest.SubmissionsReviewScope
	role_gte(member.Role, contest.SubmissionsReviewScope)
}

# GetOwnSubmission
can_get_own_submission := true if has_full_access
can_get_own_submission := true if is_participant
can_get_own_submission := true if is_moderator

# CreateSubmission
can_create_submission := true if has_full_access
can_create_submission := true if is_participant
can_create_submission := true if is_moderator
