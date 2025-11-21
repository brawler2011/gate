package authz.common

import future.keywords.if
import future.keywords.in

# Resolve user
user := input.user if input.user
else := custom.get_user(input.user_id)

# Check if user is admin
is_admin if user.Role == "admin"
is_user if user.Role == "user"
