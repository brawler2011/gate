package models

var contestActionPolicyKeys = map[ContestAction]string{
	ActionGetContest:             "get_contest",
	ActionManageContest:          "manage_contest",
	ActionGetMonitor:             "get_monitor",
	ActionListUsersSubmissions:   "list_users_submissions",
	ActionListOwnSubmissions:     "list_own_submissions",
	ActionGetOwnSubmission:       "get_own_submission",
	ActionGetOtherUserSubmission: "get_other_user_submission",
	ActionGetSubmissionDetails:   "get_submission_details",
	ActionCreateSubmission:       "create_submission",
}

var contestActionPermissionBits = map[ContestAction]ContestPermissionMask{
	ActionGetContest:             ContestPermissionGetContest,
	ActionGetContestProblem:      ContestPermissionGetContest,
	ActionManageContest:          ContestPermissionManageContest,
	ActionGetMonitor:             ContestPermissionGetMonitor,
	ActionListUsersSubmissions:   ContestPermissionListUsersSubmissions,
	ActionListOwnSubmissions:     ContestPermissionListOwnSubmissions,
	ActionGetOwnSubmission:       ContestPermissionGetOwnSubmission,
	ActionGetOtherUserSubmission: ContestPermissionGetOtherUserSubmission,
	ActionGetSubmissionDetails:   ContestPermissionGetSubmissionDetails,
	ActionCreateSubmission:       ContestPermissionCreateSubmission,
}

func ContestPermissionBitForAction(action ContestAction) (ContestPermissionMask, bool) {
	bit, ok := contestActionPermissionBits[action]
	if !ok {
		return 0, false
	}
	return bit, true
}

func HasContestActionPermission(mask ContestPermissionMask, action ContestAction) bool {
	bit, ok := ContestPermissionBitForAction(action)
	if !ok {
		return false
	}

	return mask.Has(bit)
}

func (c *Contest) HasRolePermission(role ContestRole, action ContestAction) bool {
	if len(c.AccessPolicy) == 0 {
		return false
	}

	policyKey, ok := contestActionPolicyKeys[action]
	if !ok {
		return false
	}

	rawRolePolicy, ok := c.AccessPolicy[string(role)]
	if !ok {
		return false
	}

	rolePolicy, ok := rawRolePolicy.(map[string]interface{})
	if !ok {
		return false
	}

	rawValue, ok := rolePolicy[policyKey]
	if !ok {
		return false
	}

	allowed, ok := rawValue.(bool)
	if !ok {
		return false
	}

	return allowed
}

func (c *Contest) PermissionMaskForRole(role ContestRole) (ContestPermissionMask, bool) {
	var mask ContestPermissionMask
	hasConfiguredPermission := false

	if len(c.AccessPolicy) > 0 {
		rawRolePolicy, ok := c.AccessPolicy[string(role)]
		if !ok {
			return 0, false
		}

		rolePolicy, ok := rawRolePolicy.(map[string]interface{})
		if !ok {
			return 0, false
		}

		for action, policyKey := range contestActionPolicyKeys {
			rawValue, ok := rolePolicy[policyKey]
			if !ok {
				continue
			}

			allowed, ok := rawValue.(bool)
			if !ok {
				continue
			}

			hasConfiguredPermission = true
			if !allowed {
				continue
			}

			bit, ok := ContestPermissionBitForAction(action)
			if !ok {
				continue
			}

			mask |= bit
		}

		if !hasConfiguredPermission {
			return 0, false
		}
	} else {
		defaultMask, ok := ContestRoleDefaultPermissionMask(role)
		if !ok {
			return 0, false
		}
		mask = defaultMask
		hasConfiguredPermission = true
	}

	if len(c.Settings) > 0 {
		if rawMonitorScope, ok := c.Settings["monitor_scope"]; ok {
			if monitorScopeStr, ok := rawMonitorScope.(string); ok && monitorScopeStr != "" {
				if RoleGraterOrEquals(role, ContestRole(monitorScopeStr)) {
					mask |= ContestPermissionGetMonitor
				} else {
					mask &^= ContestPermissionGetMonitor
				}
			}
		}

		if rawSubmissionsListScope, ok := c.Settings["submissions_list_scope"]; ok {
			if submissionsScopeStr, ok := rawSubmissionsListScope.(string); ok && submissionsScopeStr != "" {
				if RoleGraterOrEquals(role, ContestRole(submissionsScopeStr)) {
					mask |= ContestPermissionListUsersSubmissions
				} else {
					mask &^= ContestPermissionListUsersSubmissions
				}
			}
		}

		if rawSubmissionDetailsScope, ok := c.Settings["submission_details_scope"]; ok {
			if detailsScopeStr, ok := rawSubmissionDetailsScope.(string); ok && detailsScopeStr != "" {
				if RoleGraterOrEquals(role, ContestRole(detailsScopeStr)) {
					mask |= ContestPermissionGetSubmissionDetails
				} else {
					mask &^= ContestPermissionGetSubmissionDetails
				}
			}
		}
	}

	return mask, hasConfiguredPermission
}

func DefaultContestAccessPolicy() map[string]interface{} {
	allPermissions := map[string]interface{}{
		"get_contest":               true,
		"manage_contest":            true,
		"get_monitor":               true,
		"list_users_submissions":    true,
		"list_own_submissions":      true,
		"get_own_submission":        true,
		"get_other_user_submission": true,
		"get_submission_details":    true,
		"create_submission":         true,
	}

	moderatorPermissions := map[string]interface{}{
		"get_contest":               true,
		"manage_contest":            true,
		"get_monitor":               true,
		"list_users_submissions":    true,
		"list_own_submissions":      true,
		"get_own_submission":        true,
		"get_other_user_submission": true,
		"get_submission_details":    true,
		"create_submission":         true,
	}

	participantPermissions := map[string]interface{}{
		"get_contest":               true,
		"manage_contest":            false,
		"get_monitor":               false,
		"list_users_submissions":    false,
		"list_own_submissions":      true,
		"get_own_submission":        true,
		"get_other_user_submission": false,
		"get_submission_details":    false,
		"create_submission":         true,
	}

	return map[string]interface{}{
		string(ContestRoleOwner):       allPermissions,
		string(ContestRoleModerator):   moderatorPermissions,
		string(ContestRoleParticipant): participantPermissions,
	}
}
