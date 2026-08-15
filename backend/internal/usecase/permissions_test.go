package usecase

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func TestPickHigherContestRoleWithMask(t *testing.T) {
	participantRole := models.ContestRoleParticipant
	moderatorRole := models.ContestRoleModerator

	mask1 := models.ContestPermissionGetContest | models.ContestPermissionCreateSubmission
	mask2 := models.ContestPermissionGetMonitor

	// Test case 1: Initial role assignment
	role, mask := pickHigherContestRoleWithMask(nil, nil, 0, participantRole, &mask1)
	if role == nil || *role != participantRole {
		t.Errorf("expected role participant, got %v", role)
	}
	if mask != mask1 {
		t.Errorf("expected mask %d, got %d", mask1, mask)
	}

	// Test case 2: Same role level combining masks (OR)
	role, mask = pickHigherContestRoleWithMask(nil, role, mask, participantRole, &mask2)
	if role == nil || *role != participantRole {
		t.Errorf("expected role participant, got %v", role)
	}
	expectedMask := mask1 | mask2
	if mask != expectedMask {
		t.Errorf("expected combined mask %d, got %d", expectedMask, mask)
	}

	// Test case 3: Upgrade to higher role, preserving combined permissions
	role, mask = pickHigherContestRoleWithMask(nil, role, mask, moderatorRole, nil)
	if role == nil || *role != moderatorRole {
		t.Errorf("expected role moderator, got %v", role)
	}
	defaultModMask, _ := models.ContestRoleDefaultPermissionMask(moderatorRole)
	if (mask & defaultModMask) != defaultModMask {
		t.Errorf("expected moderator permissions mask to contain default moderator mask, got %d", mask)
	}
}

func TestResolveContestRoleAndMask_OwnerAndPublic(t *testing.T) {
	ownerID := uuid.New()
	contest := &models.Contest{
		OwnerID:    &ownerID,
		Visibility: models.ContestVisibilityPrivate,
	}

	uc := &PermissionsUseCase{}

	// Test 1: Owner gets Owner role
	role, mask, err := uc.resolveContestRoleAndMask(context.Background(), contest, ownerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role == nil || *role != models.ContestRoleOwner {
		t.Errorf("expected ContestRoleOwner, got %v", role)
	}
	if mask == 0 {
		t.Errorf("expected non-zero mask for owner, got 0")
	}

	// Test 2: Non-owner in public contest gets Participant role
	publicContest := &models.Contest{
		Visibility: models.ContestVisibilityPublic,
	}
	role, mask, err = uc.resolveContestRoleAndMask(context.Background(), publicContest, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role == nil || *role != models.ContestRoleParticipant {
		t.Errorf("expected ContestRoleParticipant for public contest, got %v", role)
	}
	if mask == 0 {
		t.Errorf("expected non-zero mask for public contest participant, got 0")
	}
}

func TestResolveProblemPermission_Guest(t *testing.T) {
	uc := &PermissionsUseCase{}
	perm, err := uc.resolveProblemPermission(context.Background(), uuid.New(), uuid.New(), uuid.Nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perm != nil {
		t.Errorf("expected nil permission for guest user, got %v", perm)
	}
}

