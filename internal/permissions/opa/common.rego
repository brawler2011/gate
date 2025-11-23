package authz.common

import future.keywords.if
import future.keywords.in

# Check if user_id is empty (anonymous user)
is_empty_uuid(uuid_str) if {
	uuid_str == "00000000-0000-0000-0000-000000000000"
}

# Resolve user
user := input.user if input.user
else := custom.get_user(input.user_id) if {
	not is_empty_uuid(input.user_id)
}
else := null

# Check if user is admin
is_admin if {
	user
	user.Role == "admin"
}

is_user if {
	user
	user.Role == "user"
}

# Check if user is anonymous (not authenticated)
is_anonymous if not user
