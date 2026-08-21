package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- In-Memory Mocks for Testing ---

type mockUsersUC struct {
	interfaces.UsersUC
	users map[uuid.UUID]models.User
}

func newMockUsersUC() *mockUsersUC {
	return &mockUsersUC{users: make(map[uuid.UUID]models.User)}
}

func (m *mockUsersUC) GetUserById(ctx context.Context, id uuid.UUID) (models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return models.User{}, pkg.ErrNotFound
}

type mockContestsRepo struct {
	interfaces.ContestsRepo
	contests     map[uuid.UUID]models.Contest
	members      map[string]models.ContestMember // key: contestID:userID
	contestTeams map[uuid.UUID][]models.ContestTeam
}

func newMockContestsRepo() *mockContestsRepo {
	return &mockContestsRepo{
		contests:     make(map[uuid.UUID]models.Contest),
		members:      make(map[string]models.ContestMember),
		contestTeams: make(map[uuid.UUID][]models.ContestTeam),
	}
}

func (m *mockContestsRepo) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	if c, ok := m.contests[id]; ok {
		return c, nil
	}
	return models.Contest{}, pkg.ErrNotFound
}

func (m *mockContestsRepo) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error) {
	key := c.ContestId.String() + ":" + c.UserId.String()
	if mem, ok := m.members[key]; ok {
		return mem, nil
	}
	return models.ContestMember{}, pkg.ErrNotFound
}

func (m *mockContestsRepo) GetContestTeams(ctx context.Context, contestID uuid.UUID) ([]models.ContestTeam, error) {
	return m.contestTeams[contestID], nil
}

type mockOrgsRepo struct {
	interfaces.OrganizationsRepo
	members map[string]models.OrganizationMember // key: orgID:userID
}

func newMockOrgsRepo() *mockOrgsRepo {
	return &mockOrgsRepo{members: make(map[string]models.OrganizationMember)}
}

func (m *mockOrgsRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrganizationMember, error) {
	key := orgID.String() + ":" + userID.String()
	if mem, ok := m.members[key]; ok {
		return &mem, nil
	}
	return nil, pkg.ErrNotFound
}

type mockTeamsRepo struct {
	interfaces.TeamsRepo
	userTeams map[string][]models.Team // key: userID:orgID
}

func newMockTeamsRepo() *mockTeamsRepo {
	return &mockTeamsRepo{userTeams: make(map[string][]models.Team)}
}

func (m *mockTeamsRepo) GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]models.Team, error) {
	key := userID.String() + ":" + orgID.String()
	return m.userTeams[key], nil
}

type mockProblemsRepo struct {
	interfaces.ProblemsRepo
	problems     map[uuid.UUID]models.Problem
	members      map[string]models.ProblemMember // key: problemID:userID
	problemTeams map[uuid.UUID][]models.ProblemTeam
}

func newMockProblemsRepo() *mockProblemsRepo {
	return &mockProblemsRepo{
		problems:     make(map[uuid.UUID]models.Problem),
		members:      make(map[string]models.ProblemMember),
		problemTeams: make(map[uuid.UUID][]models.ProblemTeam),
	}
}

func (m *mockProblemsRepo) GetProblemById(ctx context.Context, id uuid.UUID) (models.Problem, error) {
	if p, ok := m.problems[id]; ok {
		return p, nil
	}
	return models.Problem{}, pkg.ErrNotFound
}

func (m *mockProblemsRepo) GetProblemMember(ctx context.Context, problemID, userID uuid.UUID) (models.ProblemMember, error) {
	key := problemID.String() + ":" + userID.String()
	if mem, ok := m.members[key]; ok {
		return mem, nil
	}
	return models.ProblemMember{}, pkg.ErrNotFound
}

func (m *mockProblemsRepo) GetProblemTeams(ctx context.Context, problemID uuid.UUID) ([]models.ProblemTeam, error) {
	return m.problemTeams[problemID], nil
}

// --- Test Suite Setup ---

type permissionsFixture struct {
	usersUC   *mockUsersUC
	contests  *mockContestsRepo
	orgs      *mockOrgsRepo
	teams     *mockTeamsRepo
	problems  *mockProblemsRepo
	permUC    *PermissionsUseCase
	orgID     uuid.UUID
	contestID uuid.UUID
	problemID uuid.UUID
}

func setupPermissionsFixture() *permissionsFixture {
	usersUC := newMockUsersUC()
	contestsRepo := newMockContestsRepo()
	orgsRepo := newMockOrgsRepo()
	teamsRepo := newMockTeamsRepo()
	problemsRepo := newMockProblemsRepo()

	permUC := NewPermissionsUseCase(contestsRepo, usersUC, problemsRepo, teamsRepo, orgsRepo)

	return &permissionsFixture{
		usersUC:   usersUC,
		contests:  contestsRepo,
		orgs:      orgsRepo,
		teams:     teamsRepo,
		problems:  problemsRepo,
		permUC:    permUC,
		orgID:     uuid.New(),
		contestID: uuid.New(),
		problemID: uuid.New(),
	}
}

// --- Effective Role Resolution Tests (RBAC) ---

func TestEffectiveContestRole_Resolution(t *testing.T) {
	ctx := context.Background()

	t.Run("Global Admin gets RoleOwner on private contest", func(t *testing.T) {
		f := setupPermissionsFixture()
		adminID := uuid.New()
		f.usersUC.users[adminID] = models.User{Id: adminID, Role: models.UserRoleAdmin}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPrivate,
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, adminID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, models.ContestRoleOwner, *role)
	})

	for _, roleCase := range []struct {
		name string
		role models.OrganizationRole
	}{
		{"Org Owner gets RoleOwner on all org contests", models.OrgRoleOwner},
		{"Org Admin gets RoleOwner on all org contests", models.OrgRoleAdmin},
	} {
		t.Run(roleCase.name, func(t *testing.T) {
			f := setupPermissionsFixture()
			memberID := uuid.New()
			f.usersUC.users[memberID] = models.User{Id: memberID, Role: models.UserRoleUser}
			f.orgs.members[f.orgID.String()+":"+memberID.String()] = models.OrganizationMember{
				OrganizationID: f.orgID,
				UserID:         memberID,
				Role:           roleCase.role,
			}
			f.contests.contests[f.contestID] = models.Contest{
				ID:             f.contestID,
				OrganizationID: f.orgID,
				Visibility:     models.ContestVisibilityPrivate,
			}

			role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, memberID)
			require.NoError(t, err)
			require.NotNil(t, role)
			assert.Equal(t, models.ContestRoleOwner, *role)
		})
	}

	t.Run("Contest Owner gets RoleOwner", func(t *testing.T) {
		f := setupPermissionsFixture()
		contestOwnerID := uuid.New()
		f.usersUC.users[contestOwnerID] = models.User{Id: contestOwnerID, Role: models.UserRoleUser}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			OwnerID:        &contestOwnerID,
			Visibility:     models.ContestVisibilityPrivate,
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, contestOwnerID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, models.ContestRoleOwner, *role)
	})

	t.Run("Direct Contest Moderator gets RoleModerator", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = models.User{Id: userID, Role: models.UserRoleUser}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPrivate,
		}
		f.contests.members[f.contestID.String()+":"+userID.String()] = models.ContestMember{
			ContestID:   f.contestID,
			UserID:      userID,
			ContestRole: models.ContestRoleModerator,
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, models.ContestRoleModerator, *role)
	})

	t.Run("Team Contest Moderator gets RoleModerator via team", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		teamID := uuid.New()
		f.usersUC.users[userID] = models.User{Id: userID, Role: models.UserRoleUser}
		f.teams.userTeams[userID.String()+":"+f.orgID.String()] = []models.Team{
			{ID: teamID, OrganizationID: f.orgID, Name: "Dev Team"},
		}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPrivate,
		}
		f.contests.contestTeams[f.contestID] = []models.ContestTeam{
			{ContestID: f.contestID, TeamID: teamID, Role: models.ContestRoleModerator},
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, models.ContestRoleModerator, *role)
	})

	t.Run("Multiple teams resolution picks highest role (moderator > participant)", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		team1ID := uuid.New()
		team2ID := uuid.New()
		f.usersUC.users[userID] = models.User{Id: userID, Role: models.UserRoleUser}
		f.teams.userTeams[userID.String()+":"+f.orgID.String()] = []models.Team{
			{ID: team1ID, OrganizationID: f.orgID, Name: "Participants Team"},
			{ID: team2ID, OrganizationID: f.orgID, Name: "Jury Team"},
		}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPrivate,
		}
		f.contests.contestTeams[f.contestID] = []models.ContestTeam{
			{ContestID: f.contestID, TeamID: team1ID, Role: models.ContestRoleParticipant},
			{ContestID: f.contestID, TeamID: team2ID, Role: models.ContestRoleModerator},
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, models.ContestRoleModerator, *role)
	})

	t.Run("Public contest with open participation gives RoleParticipant to any user", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = models.User{Id: userID, Role: models.UserRoleUser}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPublic,
			Settings: map[string]interface{}{
				"participation_mode": models.ParticipationModeOpen,
			},
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, models.ContestRoleParticipant, *role)
	})

	t.Run("Public contest with invite_only does NOT give RoleParticipant to uninvited user", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = models.User{Id: userID, Role: models.UserRoleUser}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPublic,
			Settings: map[string]interface{}{
				"participation_mode": models.ParticipationModeInviteOnly,
			},
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		assert.Nil(t, role)
	})

	t.Run("Private contest without membership returns nil role", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = models.User{Id: userID, Role: models.UserRoleUser}
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPrivate,
		}

		role, _, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		assert.Nil(t, role)
	})
}

// --- Effective Problem Permission Resolution Tests ---

func TestEffectiveProblemPermission_Resolution(t *testing.T) {
	ctx := context.Background()

	t.Run("Org Owner and Org Admin get Admin permission on org problem", func(t *testing.T) {
		f := setupPermissionsFixture()
		ownerID := uuid.New()
		adminID := uuid.New()
		f.usersUC.users[ownerID] = models.User{Id: ownerID, Role: models.UserRoleUser}
		f.usersUC.users[adminID] = models.User{Id: adminID, Role: models.UserRoleUser}
		f.orgs.members[f.orgID.String()+":"+ownerID.String()] = models.OrganizationMember{
			OrganizationID: f.orgID,
			UserID:         ownerID,
			Role:           models.OrgRoleOwner,
		}
		f.orgs.members[f.orgID.String()+":"+adminID.String()] = models.OrganizationMember{
			OrganizationID: f.orgID,
			UserID:         adminID,
			Role:           models.OrgRoleAdmin,
		}
		f.problems.problems[f.problemID] = models.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			Visibility:     models.ProblemVisibilityPrivate,
		}

		permsOwner, err := f.permUC.GetProblemPermissions(ctx, f.problemID, ownerID)
		require.NoError(t, err)
		assert.True(t, permsOwner.ViewProblem)
		assert.True(t, permsOwner.EditProblem)
		assert.True(t, permsOwner.AdminProblem)

		permsAdmin, err := f.permUC.GetProblemPermissions(ctx, f.problemID, adminID)
		require.NoError(t, err)
		assert.True(t, permsAdmin.ViewProblem)
		assert.True(t, permsAdmin.EditProblem)
		assert.True(t, permsAdmin.AdminProblem)
	})

	t.Run("Problem Owner gets Admin permission", func(t *testing.T) {
		f := setupPermissionsFixture()
		ownerID := uuid.New()
		f.usersUC.users[ownerID] = models.User{Id: ownerID, Role: models.UserRoleUser}
		f.problems.problems[f.problemID] = models.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			OwnerID:        &ownerID,
			Visibility:     models.ProblemVisibilityPrivate,
		}

		perms, err := f.permUC.GetProblemPermissions(ctx, f.problemID, ownerID)
		require.NoError(t, err)
		assert.True(t, perms.AdminProblem)
	})

	t.Run("Direct Problem Moderator gets Edit permission", func(t *testing.T) {
		f := setupPermissionsFixture()
		modID := uuid.New()
		f.usersUC.users[modID] = models.User{Id: modID, Role: models.UserRoleUser}
		f.problems.problems[f.problemID] = models.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			Visibility:     models.ProblemVisibilityPrivate,
		}
		f.problems.members[f.problemID.String()+":"+modID.String()] = models.ProblemMember{
			ProblemID: f.problemID,
			UserID:    modID,
			Role:      models.ProblemRoleModerator,
		}

		perms, err := f.permUC.GetProblemPermissions(ctx, f.problemID, modID)
		require.NoError(t, err)
		assert.True(t, perms.ViewProblem)
		assert.True(t, perms.EditProblem)
		assert.False(t, perms.AdminProblem)
	})

	t.Run("Public problem is viewable by anyone including Guest", func(t *testing.T) {
		f := setupPermissionsFixture()
		f.problems.problems[f.problemID] = models.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			Visibility:     models.ProblemVisibilityPublic,
		}

		permsGuest, err := f.permUC.GetProblemPermissions(ctx, f.problemID, uuid.Nil)
		require.NoError(t, err)
		assert.True(t, permsGuest.ViewProblem)
		assert.False(t, permsGuest.EditProblem)
		assert.False(t, permsGuest.AdminProblem)
	})
}

// --- Contest ABAC Timing & Action Tests ---

func TestContestABAC_PreStart(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	futureStart := time.Now().Add(2 * time.Hour)
	futureEnd := time.Now().Add(5 * time.Hour)

	f.contests.contests[f.contestID] = models.Contest{
		ID:             f.contestID,
		OrganizationID: f.orgID,
		Visibility:     models.ContestVisibilityPublic,
		StartTime:      &futureStart,
		EndTime:        &futureEnd,
		Settings: map[string]interface{}{
			"participation_mode": models.ParticipationModeOpen,
			"monitor_scope":      "public",
		},
	}

	partID := uuid.New()
	modID := uuid.New()
	adminID := uuid.New()

	f.usersUC.users[partID] = models.User{Id: partID, Role: models.UserRoleUser}
	f.usersUC.users[modID] = models.User{Id: modID, Role: models.UserRoleUser}
	f.usersUC.users[adminID] = models.User{Id: adminID, Role: models.UserRoleAdmin}

	f.contests.members[f.contestID.String()+":"+modID.String()] = models.ContestMember{
		ContestID:   f.contestID,
		UserID:      modID,
		ContestRole: models.ContestRoleModerator,
	}

	// 1. Participant / Guest before start
	canViewProblem, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionGetContestProblem)
	require.NoError(t, err)
	assert.False(t, canViewProblem, "Participant should NOT view problem before contest start")

	canSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionCreateSubmission)
	require.NoError(t, err)
	assert.False(t, canSubmit, "Participant should NOT submit before contest start")

	canViewMonitor, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionGetMonitor)
	require.NoError(t, err)
	assert.False(t, canViewMonitor, "Participant should NOT view monitor before contest start")

	// 2. Moderator before start
	modCanViewProblem, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, models.ActionGetContestProblem)
	require.NoError(t, err)
	assert.True(t, modCanViewProblem, "Moderator should view problem before start")

	modCanSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, models.ActionCreateSubmission)
	require.NoError(t, err)
	assert.True(t, modCanSubmit, "Moderator should be able to submit solutions for testing before start")

	modCanViewMonitor, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, models.ActionGetMonitor)
	require.NoError(t, err)
	assert.True(t, modCanViewMonitor, "Moderator should view monitor before start")
}

func TestContestABAC_Running(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	pastStart := time.Now().Add(-1 * time.Hour)
	futureEnd := time.Now().Add(2 * time.Hour)

	f.contests.contests[f.contestID] = models.Contest{
		ID:             f.contestID,
		OrganizationID: f.orgID,
		Visibility:     models.ContestVisibilityPublic,
		StartTime:      &pastStart,
		EndTime:        &futureEnd,
		Settings: map[string]interface{}{
			"participation_mode":       models.ParticipationModeOpen,
			"monitor_scope":            "participant",
			"submissions_list_scope":   "moderator",
			"submission_details_scope": "moderator",
		},
	}

	partID := uuid.New()
	modID := uuid.New()
	f.usersUC.users[partID] = models.User{Id: partID, Role: models.UserRoleUser}
	f.usersUC.users[modID] = models.User{Id: modID, Role: models.UserRoleUser}
	f.contests.members[f.contestID.String()+":"+modID.String()] = models.ContestMember{
		ContestID:   f.contestID,
		UserID:      modID,
		ContestRole: models.ContestRoleModerator,
	}

	// Participant checks
	canViewProb, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionGetContestProblem)
	require.NoError(t, err)
	assert.True(t, canViewProb)

	canSub, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionCreateSubmission)
	require.NoError(t, err)
	assert.True(t, canSub)

	canMon, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionGetMonitor)
	require.NoError(t, err)
	assert.True(t, canMon, "Participant meets monitor_scope='participant'")

	canListAll, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionListUsersSubmissions)
	require.NoError(t, err)
	assert.False(t, canListAll, "Participant cannot list all submissions when submissions_list_scope='moderator'")

	canListOwn, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionListOwnSubmissions)
	require.NoError(t, err)
	assert.True(t, canListOwn, "Participant can list own submissions")

	// Guest checks (uuid.Nil)
	guestCanMon, err := f.permUC.HasContestPermission(ctx, f.contestID, uuid.Nil, models.ActionGetMonitor)
	require.NoError(t, err)
	assert.False(t, guestCanMon, "Guest does NOT meet monitor_scope='participant'")

	guestCanSub, err := f.permUC.HasContestPermission(ctx, f.contestID, uuid.Nil, models.ActionCreateSubmission)
	require.NoError(t, err)
	assert.False(t, guestCanSub, "Guest cannot submit")
}

func TestContestABAC_Finished_Upsolving(t *testing.T) {
	ctx := context.Background()

	t.Run("Upsolving enabled allows submissions after contest end for viewable users", func(t *testing.T) {
		f := setupPermissionsFixture()
		pastStart := time.Now().Add(-3 * time.Hour)
		pastEnd := time.Now().Add(-1 * time.Hour)

		enableUpsolving := true
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPublic,
			StartTime:      &pastStart,
			EndTime:        &pastEnd,
			Settings: map[string]interface{}{
				"enable_upsolving":   enableUpsolving,
				"participation_mode": models.ParticipationModeOpen,
			},
		}

		userID := uuid.New()
		f.usersUC.users[userID] = models.User{Id: userID, Role: models.UserRoleUser}

		canSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, userID, models.ActionCreateSubmission)
		require.NoError(t, err)
		assert.True(t, canSubmit, "Logged-in user can submit in upsolving mode after contest end")

		guestCanSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, uuid.Nil, models.ActionCreateSubmission)
		require.NoError(t, err)
		assert.False(t, guestCanSubmit, "Guest cannot submit in upsolving mode")
	})

	t.Run("Upsolving disabled blocks submissions after contest end for regular participants", func(t *testing.T) {
		f := setupPermissionsFixture()
		pastStart := time.Now().Add(-3 * time.Hour)
		pastEnd := time.Now().Add(-1 * time.Hour)

		enableUpsolving := false
		f.contests.contests[f.contestID] = models.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     models.ContestVisibilityPublic,
			StartTime:      &pastStart,
			EndTime:        &pastEnd,
			Settings: map[string]interface{}{
				"enable_upsolving": enableUpsolving,
			},
		}

		partID := uuid.New()
		modID := uuid.New()
		f.usersUC.users[partID] = models.User{Id: partID, Role: models.UserRoleUser}
		f.usersUC.users[modID] = models.User{Id: modID, Role: models.UserRoleUser}
		f.contests.members[f.contestID.String()+":"+modID.String()] = models.ContestMember{
			ContestID:   f.contestID,
			UserID:      modID,
			ContestRole: models.ContestRoleModerator,
		}

		canSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, models.ActionCreateSubmission)
		require.NoError(t, err)
		assert.False(t, canSubmit, "Participant cannot submit when upsolving is disabled")

		modCanSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, models.ActionCreateSubmission)
		require.NoError(t, err)
		assert.True(t, modCanSubmit, "Moderator can always submit for testing even when upsolving is disabled")
	})
}

// --- Organization Permissions Tests ---

func TestOrganizationPermissions(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	ownerID := uuid.New()
	adminID := uuid.New()
	memberID := uuid.New()
	outsiderID := uuid.New()
	globalAdminID := uuid.New()

	f.usersUC.users[ownerID] = models.User{Id: ownerID, Role: models.UserRoleUser}
	f.usersUC.users[adminID] = models.User{Id: adminID, Role: models.UserRoleUser}
	f.usersUC.users[memberID] = models.User{Id: memberID, Role: models.UserRoleUser}
	f.usersUC.users[outsiderID] = models.User{Id: outsiderID, Role: models.UserRoleUser}
	f.usersUC.users[globalAdminID] = models.User{Id: globalAdminID, Role: models.UserRoleAdmin}

	f.orgs.members[f.orgID.String()+":"+ownerID.String()] = models.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         ownerID,
		Role:           models.OrgRoleOwner,
	}
	f.orgs.members[f.orgID.String()+":"+adminID.String()] = models.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         adminID,
		Role:           models.OrgRoleAdmin,
	}
	f.orgs.members[f.orgID.String()+":"+memberID.String()] = models.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         memberID,
		Role:           models.OrgRoleMember,
	}

	// 1. ActionViewOrganization
	for _, uid := range []uuid.UUID{ownerID, adminID, memberID, outsiderID, globalAdminID} {
		canView, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, uid, models.ActionViewOrganization)
		require.NoError(t, err)
		assert.True(t, canView)
	}

	// 2. ActionManageOrganization
	orgAdminCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, adminID, models.ActionManageOrganization)
	require.NoError(t, err)
	assert.True(t, orgAdminCanManage)

	orgOwnerCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, ownerID, models.ActionManageOrganization)
	require.NoError(t, err)
	assert.True(t, orgOwnerCanManage)

	memberCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, memberID, models.ActionManageOrganization)
	require.NoError(t, err)
	assert.False(t, memberCanManage)

	outsiderCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, outsiderID, models.ActionManageOrganization)
	require.NoError(t, err)
	assert.False(t, outsiderCanManage)

	globalAdminCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, globalAdminID, models.ActionManageOrganization)
	require.NoError(t, err)
	assert.True(t, globalAdminCanManage)

	// 3. ActionDeleteOrganization
	orgOwnerCanDelete, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, ownerID, models.ActionDeleteOrganization)
	require.NoError(t, err)
	assert.True(t, orgOwnerCanDelete)

	orgAdminCanDelete, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, adminID, models.ActionDeleteOrganization)
	require.NoError(t, err)
	assert.False(t, orgAdminCanDelete, "Org Admin cannot delete organization; only Owner or Global Admin can")

	globalAdminCanDelete, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, globalAdminID, models.ActionDeleteOrganization)
	require.NoError(t, err)
	assert.True(t, globalAdminCanDelete)
}


