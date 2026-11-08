package access

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUsersReader struct {
	UsersReader
	users map[uuid.UUID]accessmodel.User
}

func newMockUsersReader() *mockUsersReader {
	return &mockUsersReader{users: make(map[uuid.UUID]accessmodel.User)}
}

func (m *mockUsersReader) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
}

type mockContestsRepo struct {
	ContestsRepo
	contests     map[uuid.UUID]contestsmodel.Contest
	members      map[string]contestsmodel.ContestMember // key: contestID:userID
	contestTeams map[uuid.UUID][]contestsmodel.ContestTeam
}

func newMockContestsRepo() *mockContestsRepo {
	return &mockContestsRepo{
		contests:     make(map[uuid.UUID]contestsmodel.Contest),
		members:      make(map[string]contestsmodel.ContestMember),
		contestTeams: make(map[uuid.UUID][]contestsmodel.ContestTeam),
	}
}

func (m *mockContestsRepo) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	if c, ok := m.contests[id]; ok {
		return c, nil
	}
	return contestsmodel.Contest{}, pkg.ErrNotFound
}

func (m *mockContestsRepo) GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	key := c.ContestId.String() + ":" + c.UserId.String()
	if mem, ok := m.members[key]; ok {
		return mem, nil
	}
	return contestsmodel.ContestMember{}, pkg.ErrNotFound
}

func (m *mockContestsRepo) GetContestTeams(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ContestTeam, error) {
	return m.contestTeams[contestID], nil
}

func (m *mockContestsRepo) ListUserContestMemberships(ctx context.Context, userID uuid.UUID) ([]contestsmodel.UserContestMembership, error) {
	var list []contestsmodel.UserContestMembership
	for _, mem := range m.members {
		if mem.UserID == userID {
			list = append(list, contestsmodel.UserContestMembership{
				ContestID: mem.ContestID,
				Role:      mem.Role,
			})
		}
	}
	return list, nil
}

func (m *mockContestsRepo) AddContestMemberIfNotExists(ctx context.Context, contestID, userID uuid.UUID, role string) error {
	key := contestID.String() + ":" + userID.String()
	if _, exists := m.members[key]; !exists {
		m.members[key] = contestsmodel.ContestMember{
			ContestID: contestID,
			UserID:    userID,
			Role:      role,
		}
	}
	return nil
}

type mockOrgsRepo struct {
	OrganizationsRepo
	members       map[string]organizationsmodel.OrganizationMember // key: orgID:userID
	organizations map[string]organizationsmodel.Organization       // key: login
}

func newMockOrgsRepo() *mockOrgsRepo {
	return &mockOrgsRepo{
		members:       make(map[string]organizationsmodel.OrganizationMember),
		organizations: make(map[string]organizationsmodel.Organization),
	}
}

func (m *mockOrgsRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	key := orgID.String() + ":" + userID.String()
	if mem, ok := m.members[key]; ok {
		return &mem, nil
	}
	return nil, pkg.ErrNotFound
}

func (m *mockOrgsRepo) GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error) {
	if org, ok := m.organizations[login]; ok {
		return &org, nil
	}
	return &organizationsmodel.Organization{
		ID:    uuid.New(),
		Login: login,
		Name:  "Test Org",
	}, nil
}

func (m *mockOrgsRepo) AddMember(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error {
	m.members[orgID.String()+":"+userID.String()] = organizationsmodel.OrganizationMember{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
	}
	return nil
}

type mockTeamsRepo struct {
	TeamsRepo
	userTeams map[string][]organizationsmodel.Team // key: userID:orgID
}

func newMockTeamsRepo() *mockTeamsRepo {
	return &mockTeamsRepo{userTeams: make(map[string][]organizationsmodel.Team)}
}

func (m *mockTeamsRepo) GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]organizationsmodel.Team, error) {
	key := userID.String() + ":" + orgID.String()
	return m.userTeams[key], nil
}

type mockProblemsRepo struct {
	ProblemsRepo
	problems     map[uuid.UUID]problemsmodel.Problem
	members      map[string]problemsmodel.ProblemMember // key: problemID:userID
	problemTeams map[uuid.UUID][]problemsmodel.ProblemTeam
}

func newMockProblemsRepo() *mockProblemsRepo {
	return &mockProblemsRepo{
		problems:     make(map[uuid.UUID]problemsmodel.Problem),
		members:      make(map[string]problemsmodel.ProblemMember),
		problemTeams: make(map[uuid.UUID][]problemsmodel.ProblemTeam),
	}
}

func (m *mockProblemsRepo) GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error) {
	if p, ok := m.problems[id]; ok {
		return p, nil
	}
	return problemsmodel.Problem{}, pkg.ErrNotFound
}

func (m *mockProblemsRepo) GetProblemMember(ctx context.Context, problemID, userID uuid.UUID) (problemsmodel.ProblemMember, error) {
	key := problemID.String() + ":" + userID.String()
	if mem, ok := m.members[key]; ok {
		return mem, nil
	}
	return problemsmodel.ProblemMember{}, pkg.ErrNotFound
}

func (m *mockProblemsRepo) GetProblemTeams(ctx context.Context, problemID uuid.UUID) ([]problemsmodel.ProblemTeam, error) {
	return m.problemTeams[problemID], nil
}

type permissionsFixture struct {
	usersUC   *mockUsersReader
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
	usersUC := newMockUsersReader()
	contestsRepo := newMockContestsRepo()
	orgsRepo := newMockOrgsRepo()
	teamsRepo := newMockTeamsRepo()
	problemsRepo := newMockProblemsRepo()

	permUC := NewPermissionsUseCase(contestsRepo, usersUC, problemsRepo, teamsRepo, orgsRepo)

	f := &permissionsFixture{
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
	orgsRepo.organizations["test-org"] = organizationsmodel.Organization{
		ID:    f.orgID,
		Login: "test-org",
		Name:  "Test Org",
	}
	return f
}

func TestEffectiveContestRole_Resolution(t *testing.T) {
	ctx := context.Background()

	t.Run("Global Admin gets RoleOwner on private contest", func(t *testing.T) {
		f := setupPermissionsFixture()
		adminID := uuid.New()
		f.usersUC.users[adminID] = accessmodel.User{Id: adminID, Role: accessmodel.UserRoleAdmin}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPrivate,
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, adminID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, contestsmodel.ContestRoleOwner, *role)
	})

	for _, roleCase := range []struct {
		name string
		role organizationsmodel.OrganizationRole
	}{
		{"Org Owner gets RoleOwner on all org contests", organizationsmodel.OrgRoleOwner},
		{"Org Admin gets RoleOwner on all org contests", organizationsmodel.OrgRoleAdmin},
	} {
		t.Run(roleCase.name, func(t *testing.T) {
			f := setupPermissionsFixture()
			memberID := uuid.New()
			f.usersUC.users[memberID] = accessmodel.User{Id: memberID, Role: accessmodel.UserRoleUser}
			f.orgs.members[f.orgID.String()+":"+memberID.String()] = organizationsmodel.OrganizationMember{
				OrganizationID: f.orgID,
				UserID:         memberID,
				Role:           roleCase.role,
			}
			f.contests.contests[f.contestID] = contestsmodel.Contest{
				ID:             f.contestID,
				OrganizationID: f.orgID,
				Visibility:     contestsmodel.ContestVisibilityPrivate,
			}

			role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, memberID)
			require.NoError(t, err)
			require.NotNil(t, role)
			assert.Equal(t, contestsmodel.ContestRoleOwner, *role)
		})
	}

	t.Run("Contest Owner gets RoleOwner", func(t *testing.T) {
		f := setupPermissionsFixture()
		contestOwnerID := uuid.New()
		f.usersUC.users[contestOwnerID] = accessmodel.User{Id: contestOwnerID, Role: accessmodel.UserRoleUser}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			OwnerID:        &contestOwnerID,
			Visibility:     contestsmodel.ContestVisibilityPrivate,
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, contestOwnerID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, contestsmodel.ContestRoleOwner, *role)
	})

	t.Run("Direct Contest Moderator gets RoleModerator", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPrivate,
		}
		f.contests.members[f.contestID.String()+":"+userID.String()] = contestsmodel.ContestMember{
			ContestID:   f.contestID,
			UserID:      userID,
			ContestRole: contestsmodel.ContestRoleModerator,
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, contestsmodel.ContestRoleModerator, *role)
	})

	t.Run("Team Contest Moderator gets RoleModerator via team", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		teamID := uuid.New()
		f.usersUC.users[userID] = accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}
		f.teams.userTeams[userID.String()+":"+f.orgID.String()] = []organizationsmodel.Team{
			{ID: teamID, OrganizationID: f.orgID, Name: "Dev Team"},
		}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPrivate,
		}
		f.contests.contestTeams[f.contestID] = []contestsmodel.ContestTeam{
			{ContestID: f.contestID, TeamID: teamID, Role: contestsmodel.ContestRoleModerator},
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, contestsmodel.ContestRoleModerator, *role)
	})

	t.Run("Multiple teams resolution picks highest role (moderator > participant)", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		team1ID := uuid.New()
		team2ID := uuid.New()
		f.usersUC.users[userID] = accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}
		f.teams.userTeams[userID.String()+":"+f.orgID.String()] = []organizationsmodel.Team{
			{ID: team1ID, OrganizationID: f.orgID, Name: "Participants Team"},
			{ID: team2ID, OrganizationID: f.orgID, Name: "Jury Team"},
		}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPrivate,
		}
		f.contests.contestTeams[f.contestID] = []contestsmodel.ContestTeam{
			{ContestID: f.contestID, TeamID: team1ID, Role: contestsmodel.ContestRoleParticipant},
			{ContestID: f.contestID, TeamID: team2ID, Role: contestsmodel.ContestRoleModerator},
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, contestsmodel.ContestRoleModerator, *role)
	})

	t.Run("Public contest with open participation gives RoleParticipant to any user", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPublic,
			Settings: map[string]interface{}{
				"participation_mode": contestsmodel.ParticipationModeOpen,
			},
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		require.NotNil(t, role)
		assert.Equal(t, contestsmodel.ContestRoleParticipant, *role)
	})

	t.Run("Public contest with invite_only does NOT give RoleParticipant to uninvited user", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPublic,
			Settings: map[string]interface{}{
				"participation_mode": contestsmodel.ParticipationModeInviteOnly,
			},
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		assert.Nil(t, role)
	})

	t.Run("Private contest without membership returns nil role", func(t *testing.T) {
		f := setupPermissionsFixture()
		userID := uuid.New()
		f.usersUC.users[userID] = accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPrivate,
		}

		role, err := f.permUC.GetEffectiveContestRole(ctx, f.contestID, userID)
		require.NoError(t, err)
		assert.Nil(t, role)
	})
}

func TestEffectiveProblemPermission_Resolution(t *testing.T) {
	ctx := context.Background()

	t.Run("Org Owner and Org Admin get Admin permission on org problem", func(t *testing.T) {
		f := setupPermissionsFixture()
		ownerID := uuid.New()
		adminID := uuid.New()
		f.usersUC.users[ownerID] = accessmodel.User{Id: ownerID, Role: accessmodel.UserRoleUser}
		f.usersUC.users[adminID] = accessmodel.User{Id: adminID, Role: accessmodel.UserRoleUser}
		f.orgs.members[f.orgID.String()+":"+ownerID.String()] = organizationsmodel.OrganizationMember{
			OrganizationID: f.orgID,
			UserID:         ownerID,
			Role:           organizationsmodel.OrgRoleOwner,
		}
		f.orgs.members[f.orgID.String()+":"+adminID.String()] = organizationsmodel.OrganizationMember{
			OrganizationID: f.orgID,
			UserID:         adminID,
			Role:           organizationsmodel.OrgRoleAdmin,
		}
		f.problems.problems[f.problemID] = problemsmodel.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			Visibility:     problemsmodel.ProblemVisibilityPrivate,
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
		f.usersUC.users[ownerID] = accessmodel.User{Id: ownerID, Role: accessmodel.UserRoleUser}
		f.problems.problems[f.problemID] = problemsmodel.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			OwnerID:        &ownerID,
			Visibility:     problemsmodel.ProblemVisibilityPrivate,
		}

		perms, err := f.permUC.GetProblemPermissions(ctx, f.problemID, ownerID)
		require.NoError(t, err)
		assert.True(t, perms.AdminProblem)
	})

	t.Run("Direct Problem Moderator gets Edit permission", func(t *testing.T) {
		f := setupPermissionsFixture()
		modID := uuid.New()
		f.usersUC.users[modID] = accessmodel.User{Id: modID, Role: accessmodel.UserRoleUser}
		f.problems.problems[f.problemID] = problemsmodel.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			Visibility:     problemsmodel.ProblemVisibilityPrivate,
		}
		f.problems.members[f.problemID.String()+":"+modID.String()] = problemsmodel.ProblemMember{
			ProblemID: f.problemID,
			UserID:    modID,
			Role:      problemsmodel.ProblemRoleModerator,
		}

		perms, err := f.permUC.GetProblemPermissions(ctx, f.problemID, modID)
		require.NoError(t, err)
		assert.True(t, perms.ViewProblem)
		assert.True(t, perms.EditProblem)
		assert.False(t, perms.AdminProblem)
	})

	t.Run("Public problem is viewable by anyone including Guest", func(t *testing.T) {
		f := setupPermissionsFixture()
		f.problems.problems[f.problemID] = problemsmodel.Problem{
			ID:             f.problemID,
			OrganizationID: f.orgID,
			Visibility:     problemsmodel.ProblemVisibilityPublic,
		}

		permsGuest, err := f.permUC.GetProblemPermissions(ctx, f.problemID, uuid.Nil)
		require.NoError(t, err)
		assert.True(t, permsGuest.ViewProblem)
		assert.False(t, permsGuest.EditProblem)
		assert.False(t, permsGuest.AdminProblem)
	})
}

func TestContestABAC_PreStart(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	futureStart := time.Now().Add(2 * time.Hour)
	futureEnd := time.Now().Add(5 * time.Hour)

	f.contests.contests[f.contestID] = contestsmodel.Contest{
		ID:             f.contestID,
		OrganizationID: f.orgID,
		Visibility:     contestsmodel.ContestVisibilityPublic,
		StartTime:      &futureStart,
		EndTime:        &futureEnd,
		Settings: map[string]interface{}{
			"participation_mode": contestsmodel.ParticipationModeOpen,
			"monitor_scope":      "public",
		},
	}

	partID := uuid.New()
	modID := uuid.New()
	adminID := uuid.New()

	f.usersUC.users[partID] = accessmodel.User{Id: partID, Role: accessmodel.UserRoleUser}
	f.usersUC.users[modID] = accessmodel.User{Id: modID, Role: accessmodel.UserRoleUser}
	f.usersUC.users[adminID] = accessmodel.User{Id: adminID, Role: accessmodel.UserRoleAdmin}

	f.contests.members[f.contestID.String()+":"+modID.String()] = contestsmodel.ContestMember{
		ContestID:   f.contestID,
		UserID:      modID,
		ContestRole: contestsmodel.ContestRoleModerator,
	}

	// 1. Participant / Guest before start
	canViewProblem, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionGetContestProblem)
	require.NoError(t, err)
	assert.False(t, canViewProblem, "Participant should NOT view problem before contest start")

	canSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionCreateSubmission)
	require.NoError(t, err)
	assert.False(t, canSubmit, "Participant should NOT submit before contest start")

	canViewMonitor, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionGetMonitor)
	require.NoError(t, err)
	assert.False(t, canViewMonitor, "Participant should NOT view monitor before contest start")

	// 2. Moderator before start
	modCanViewProblem, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, accessmodel.ActionGetContestProblem)
	require.NoError(t, err)
	assert.True(t, modCanViewProblem, "Moderator should view problem before start")

	modCanSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, accessmodel.ActionCreateSubmission)
	require.NoError(t, err)
	assert.True(t, modCanSubmit, "Moderator should be able to submit solutions for testing before start")

	modCanViewMonitor, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, accessmodel.ActionGetMonitor)
	require.NoError(t, err)
	assert.True(t, modCanViewMonitor, "Moderator should view monitor before start")
}

func TestContestABAC_Running(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	pastStart := time.Now().Add(-1 * time.Hour)
	futureEnd := time.Now().Add(2 * time.Hour)

	f.contests.contests[f.contestID] = contestsmodel.Contest{
		ID:             f.contestID,
		OrganizationID: f.orgID,
		Visibility:     contestsmodel.ContestVisibilityPublic,
		StartTime:      &pastStart,
		EndTime:        &futureEnd,
		Settings: map[string]interface{}{
			"participation_mode":       contestsmodel.ParticipationModeOpen,
			"monitor_scope":            "participant",
			"submissions_list_scope":   "moderator",
			"submission_details_scope": "moderator",
		},
	}

	partID := uuid.New()
	modID := uuid.New()
	f.usersUC.users[partID] = accessmodel.User{Id: partID, Role: accessmodel.UserRoleUser}
	f.usersUC.users[modID] = accessmodel.User{Id: modID, Role: accessmodel.UserRoleUser}
	f.contests.members[f.contestID.String()+":"+modID.String()] = contestsmodel.ContestMember{
		ContestID:   f.contestID,
		UserID:      modID,
		ContestRole: contestsmodel.ContestRoleModerator,
	}

	// Participant checks
	canViewProb, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionGetContestProblem)
	require.NoError(t, err)
	assert.True(t, canViewProb)

	canSub, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionCreateSubmission)
	require.NoError(t, err)
	assert.True(t, canSub)

	canMon, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionGetMonitor)
	require.NoError(t, err)
	assert.True(t, canMon, "Participant meets monitor_scope='participant'")

	canListAll, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionListUsersSubmissions)
	require.NoError(t, err)
	assert.False(t, canListAll, "Participant cannot list all submissions when submissions_list_scope='moderator'")

	canListOwn, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionListOwnSubmissions)
	require.NoError(t, err)
	assert.True(t, canListOwn, "Participant can list own submissions")

	// Guest checks (uuid.Nil)
	guestCanMon, err := f.permUC.HasContestPermission(ctx, f.contestID, uuid.Nil, accessmodel.ActionGetMonitor)
	require.NoError(t, err)
	assert.False(t, guestCanMon, "Guest does NOT meet monitor_scope='participant'")

	guestCanSub, err := f.permUC.HasContestPermission(ctx, f.contestID, uuid.Nil, accessmodel.ActionCreateSubmission)
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
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPublic,
			StartTime:      &pastStart,
			EndTime:        &pastEnd,
			Settings: map[string]interface{}{
				"enable_upsolving":   enableUpsolving,
				"participation_mode": contestsmodel.ParticipationModeOpen,
			},
		}

		userID := uuid.New()
		f.usersUC.users[userID] = accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser}

		canSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, userID, accessmodel.ActionCreateSubmission)
		require.NoError(t, err)
		assert.True(t, canSubmit, "Logged-in user can submit in upsolving mode after contest end")

		guestCanSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, uuid.Nil, accessmodel.ActionCreateSubmission)
		require.NoError(t, err)
		assert.False(t, guestCanSubmit, "Guest cannot submit in upsolving mode")
	})

	t.Run("Upsolving disabled blocks submissions after contest end for regular participants", func(t *testing.T) {
		f := setupPermissionsFixture()
		pastStart := time.Now().Add(-3 * time.Hour)
		pastEnd := time.Now().Add(-1 * time.Hour)

		enableUpsolving := false
		f.contests.contests[f.contestID] = contestsmodel.Contest{
			ID:             f.contestID,
			OrganizationID: f.orgID,
			Visibility:     contestsmodel.ContestVisibilityPublic,
			StartTime:      &pastStart,
			EndTime:        &pastEnd,
			Settings: map[string]interface{}{
				"enable_upsolving": enableUpsolving,
			},
		}

		partID := uuid.New()
		modID := uuid.New()
		f.usersUC.users[partID] = accessmodel.User{Id: partID, Role: accessmodel.UserRoleUser}
		f.usersUC.users[modID] = accessmodel.User{Id: modID, Role: accessmodel.UserRoleUser}
		f.contests.members[f.contestID.String()+":"+modID.String()] = contestsmodel.ContestMember{
			ContestID:   f.contestID,
			UserID:      modID,
			ContestRole: contestsmodel.ContestRoleModerator,
		}

		canSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, partID, accessmodel.ActionCreateSubmission)
		require.NoError(t, err)
		assert.False(t, canSubmit, "Participant cannot submit when upsolving is disabled")

		modCanSubmit, err := f.permUC.HasContestPermission(ctx, f.contestID, modID, accessmodel.ActionCreateSubmission)
		require.NoError(t, err)
		assert.True(t, modCanSubmit, "Moderator can always submit for testing even when upsolving is disabled")
	})
}

func TestOrganizationPermissions(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	ownerID := uuid.New()
	adminID := uuid.New()
	memberID := uuid.New()
	outsiderID := uuid.New()
	globalAdminID := uuid.New()

	f.usersUC.users[ownerID] = accessmodel.User{Id: ownerID, Role: accessmodel.UserRoleUser}
	f.usersUC.users[adminID] = accessmodel.User{Id: adminID, Role: accessmodel.UserRoleUser}
	f.usersUC.users[memberID] = accessmodel.User{Id: memberID, Role: accessmodel.UserRoleUser}
	f.usersUC.users[outsiderID] = accessmodel.User{Id: outsiderID, Role: accessmodel.UserRoleUser}
	f.usersUC.users[globalAdminID] = accessmodel.User{Id: globalAdminID, Role: accessmodel.UserRoleAdmin}

	f.orgs.members[f.orgID.String()+":"+ownerID.String()] = organizationsmodel.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         ownerID,
		Role:           organizationsmodel.OrgRoleOwner,
	}
	f.orgs.members[f.orgID.String()+":"+adminID.String()] = organizationsmodel.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         adminID,
		Role:           organizationsmodel.OrgRoleAdmin,
	}
	f.orgs.members[f.orgID.String()+":"+memberID.String()] = organizationsmodel.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         memberID,
		Role:           organizationsmodel.OrgRoleMember,
	}

	// 1. ActionViewOrganization
	for _, uid := range []uuid.UUID{ownerID, adminID, memberID, outsiderID, globalAdminID} {
		canView, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, uid, accessmodel.ActionViewOrganization)
		require.NoError(t, err)
		assert.True(t, canView)
	}

	// 2. ActionManageOrganization
	orgAdminCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, adminID, accessmodel.ActionManageOrganization)
	require.NoError(t, err)
	assert.True(t, orgAdminCanManage)

	orgOwnerCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, ownerID, accessmodel.ActionManageOrganization)
	require.NoError(t, err)
	assert.True(t, orgOwnerCanManage)

	memberCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, memberID, accessmodel.ActionManageOrganization)
	require.NoError(t, err)
	assert.False(t, memberCanManage)

	outsiderCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, outsiderID, accessmodel.ActionManageOrganization)
	require.NoError(t, err)
	assert.False(t, outsiderCanManage)

	globalAdminCanManage, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, globalAdminID, accessmodel.ActionManageOrganization)
	require.NoError(t, err)
	assert.True(t, globalAdminCanManage)

	// 3. ActionDeleteOrganization
	orgOwnerCanDelete, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, ownerID, accessmodel.ActionDeleteOrganization)
	require.NoError(t, err)
	assert.True(t, orgOwnerCanDelete)

	orgAdminCanDelete, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, adminID, accessmodel.ActionDeleteOrganization)
	require.NoError(t, err)
	assert.False(t, orgAdminCanDelete, "Org Admin cannot delete organization; only Owner or Global Admin can")

	globalAdminCanDelete, err := f.permUC.HasOrganizationPermission(ctx, f.orgID, globalAdminID, accessmodel.ActionDeleteOrganization)
	require.NoError(t, err)
	assert.True(t, globalAdminCanDelete)
}
