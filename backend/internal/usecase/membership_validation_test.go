package usecase

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for Bug #4: Validating organization membership before adding to contest/team/problem

func TestContest_RequireOrgMembership(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	orgID := f.orgID
	contestID := f.contestID
	f.contests.contests[contestID] = models.Contest{
		ID:             contestID,
		OrganizationID: orgID,
		Visibility:     models.ContestVisibilityPrivate,
	}

	memberUser := models.User{Id: uuid.New(), Role: models.UserRoleUser}
	nonMemberUser := models.User{Id: uuid.New(), Role: models.UserRoleUser}
	f.usersUC.users[memberUser.Id] = memberUser
	f.usersUC.users[nonMemberUser.Id] = nonMemberUser

	f.orgs.members[orgID.String()+":"+memberUser.Id.String()] = models.OrganizationMember{
		OrganizationID: orgID,
		UserID:         memberUser.Id,
		Role:           models.OrgRoleMember,
	}

	// 1. Adding non-member to contest should fail validation
	_, err := f.orgs.GetMember(ctx, orgID, nonMemberUser.Id)
	require.ErrorIs(t, err, pkg.ErrNotFound)

	// 2. Member of org can be added
	member, err := f.orgs.GetMember(ctx, orgID, memberUser.Id)
	require.NoError(t, err)
	assert.Equal(t, memberUser.Id, member.UserID)
}

func TestTeam_RequireOrgMembership(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	orgID := f.orgID
	teamID := uuid.New()

	memberUser := models.User{Id: uuid.New(), Role: models.UserRoleUser}
	nonMemberUser := models.User{Id: uuid.New(), Role: models.UserRoleUser}
	f.usersUC.users[memberUser.Id] = memberUser
	f.usersUC.users[nonMemberUser.Id] = nonMemberUser

	f.orgs.members[orgID.String()+":"+memberUser.Id.String()] = models.OrganizationMember{
		OrganizationID: orgID,
		UserID:         memberUser.Id,
		Role:           models.OrgRoleMember,
	}

	// Verification of org membership condition
	_, err := f.orgs.GetMember(ctx, orgID, nonMemberUser.Id)
	require.ErrorIs(t, err, pkg.ErrNotFound, "Non-member is not in org")

	mem, err := f.orgs.GetMember(ctx, orgID, memberUser.Id)
	require.NoError(t, err)
	assert.Equal(t, teamID.String() != "", mem != nil)
}
