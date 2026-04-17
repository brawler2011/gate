package models

import "testing"

func TestContestHasRolePermission_FromStrictBooleanMatrixPolicy(t *testing.T) {
	contest := Contest{
		AccessPolicy: map[string]interface{}{
			"participant": map[string]interface{}{
				"get_contest":               true,
				"create_submission":         true,
				"list_users_submissions":    false,
				"get_other_user_submission": false,
			},
		},
	}

	if !contest.HasRolePermission(ContestRoleParticipant, ActionGetContest) {
		t.Fatalf("expected get_contest to be allowed")
	}

	if !contest.HasRolePermission(ContestRoleParticipant, ActionCreateSubmission) {
		t.Fatalf("expected create_submission to be allowed")
	}

	if contest.HasRolePermission(ContestRoleParticipant, ActionListUsersSubmissions) {
		t.Fatalf("expected list_users_submissions to be denied")
	}

	if contest.HasRolePermission(ContestRoleParticipant, ActionGetOtherUserSubmission) {
		t.Fatalf("expected get_other_user_submission to be denied")
	}
}

func TestContestHasRolePermission_RejectsLegacyPolicyShapes(t *testing.T) {
	contest := Contest{
		AccessPolicy: map[string]interface{}{
			"roles": map[string]interface{}{
				"participant": map[string]interface{}{
					"get_contest":       true,
					"create_submission": true,
				},
			},
		},
	}

	if contest.HasRolePermission(ContestRoleParticipant, ActionGetContest) {
		t.Fatalf("expected nested legacy container to be denied")
	}
}

func TestContestHasRolePermission_RejectsNonBooleanPolicyValues(t *testing.T) {
	contest := Contest{
		AccessPolicy: map[string]interface{}{
			"participant": map[string]interface{}{
				"get_contest":       "true",
				"create_submission": 1,
			},
		},
	}

	if contest.HasRolePermission(ContestRoleParticipant, ActionGetContest) {
		t.Fatalf("expected string boolean to be denied")
	}

	if contest.HasRolePermission(ContestRoleParticipant, ActionCreateSubmission) {
		t.Fatalf("expected numeric boolean to be denied")
	}
}

func TestContestHasRolePermission_NotConfigured(t *testing.T) {
	contest := Contest{AccessPolicy: map[string]interface{}{}}

	if contest.HasRolePermission(ContestRoleParticipant, ActionGetContest) {
		t.Fatalf("expected not configured policy to be denied by default")
	}
}

func TestContestPermissionMaskFromPolicy_StrictBooleanMatrix(t *testing.T) {
	contest := Contest{
		AccessPolicy: map[string]interface{}{
			"participant": map[string]interface{}{
				"get_contest":               true,
				"manage_contest":            false,
				"list_users_submissions":    false,
				"list_own_submissions":      true,
				"get_own_submission":        true,
				"get_other_user_submission": false,
				"create_submission":         true,
			},
		},
	}

	mask, ok := contest.PermissionMaskForRole(ContestRoleParticipant)
	if !ok {
		t.Fatalf("expected permission mask to be derived from policy")
	}

	if !HasContestActionPermission(mask, ActionGetContest) {
		t.Fatalf("expected get_contest bit to be set")
	}
	if HasContestActionPermission(mask, ActionManageContest) {
		t.Fatalf("expected manage_contest bit to be unset")
	}
	if HasContestActionPermission(mask, ActionListUsersSubmissions) {
		t.Fatalf("expected list_users_submissions bit to be unset")
	}
	if !HasContestActionPermission(mask, ActionListOwnSubmissions) {
		t.Fatalf("expected list_own_submissions bit to be set")
	}
	if !HasContestActionPermission(mask, ActionGetOwnSubmission) {
		t.Fatalf("expected get_own_submission bit to be set")
	}
	if HasContestActionPermission(mask, ActionGetOtherUserSubmission) {
		t.Fatalf("expected get_other_user_submission bit to be unset")
	}
	if !HasContestActionPermission(mask, ActionCreateSubmission) {
		t.Fatalf("expected create_submission bit to be set")
	}
}

func TestContestPermissionMaskFromPolicy_RejectsLegacyShape(t *testing.T) {
	contest := Contest{
		AccessPolicy: map[string]interface{}{
			"roles": map[string]interface{}{
				"participant": map[string]interface{}{
					"get_contest": true,
				},
			},
		},
	}

	_, ok := contest.PermissionMaskForRole(ContestRoleParticipant)
	if ok {
		t.Fatalf("expected legacy policy shape to be rejected")
	}
}

func TestContestRoleDefaultPermissionMask_Parity(t *testing.T) {
	ownerMask, ok := ContestRoleDefaultPermissionMask(ContestRoleOwner)
	if !ok {
		t.Fatalf("expected owner mask to exist")
	}

	moderatorMask, ok := ContestRoleDefaultPermissionMask(ContestRoleModerator)
	if !ok {
		t.Fatalf("expected moderator mask to exist")
	}

	participantMask, ok := ContestRoleDefaultPermissionMask(ContestRoleParticipant)
	if !ok {
		t.Fatalf("expected participant mask to exist")
	}

	if ownerMask != moderatorMask {
		t.Fatalf("expected owner and moderator default masks to be equal with current policy")
	}

	if !HasContestActionPermission(participantMask, ActionGetContest) {
		t.Fatalf("expected participant to be able to view contest")
	}
	if HasContestActionPermission(participantMask, ActionManageContest) {
		t.Fatalf("expected participant to be unable to manage contest")
	}
	if !HasContestActionPermission(participantMask, ActionListOwnSubmissions) {
		t.Fatalf("expected participant to list own submissions")
	}
	if HasContestActionPermission(participantMask, ActionListUsersSubmissions) {
		t.Fatalf("expected participant to be unable to list users submissions")
	}
	if !HasContestActionPermission(participantMask, ActionCreateSubmission) {
		t.Fatalf("expected participant to create submission")
	}
}
