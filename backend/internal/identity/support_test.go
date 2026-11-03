package identity

import (
	"context"

	notificationsmodel "github.com/brawler2011/gate/backend/internal/notifications/model"

	access "github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func newMockContestsRepo() *mockContestsRepo {
	return &mockContestsRepo{
		contests:     make(map[uuid.UUID]contestsmodel.Contest),
		members:      make(map[string]contestsmodel.ContestMember),
		contestTeams: make(map[uuid.UUID][]contestsmodel.ContestTeam),
	}
}

type organizationTestStore struct {
	*access.PermissionsUseCase
	OrganizationsRepo
	identityUsersStore
}

func setupPermissionsFixture() *permissionsFixture {
	usersUC := newMockUsersUC()
	contestsRepo := newMockContestsRepo()
	orgsRepo := newMockOrgsRepo()
	teamsRepo := newMockTeamsRepo()
	problemsRepo := newMockProblemsRepo()

	permUC := access.NewPermissionsUseCase(contestsRepo, usersUC, problemsRepo, teamsRepo, orgsRepo)

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

type mockContestsRepo struct {
	ContestsRepo
	contests     map[uuid.UUID]contestsmodel.Contest
	members      map[string]contestsmodel.ContestMember // key: contestID:userID
	contestTeams map[uuid.UUID][]contestsmodel.ContestTeam
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

func newMockOrgsRepo() *mockOrgsRepo {
	return &mockOrgsRepo{
		members:       make(map[string]organizationsmodel.OrganizationMember),
		organizations: make(map[string]organizationsmodel.Organization),
	}
}

func newMockProblemsRepo() *mockProblemsRepo {
	return &mockProblemsRepo{
		problems:     make(map[uuid.UUID]problemsmodel.Problem),
		members:      make(map[string]problemsmodel.ProblemMember),
		problemTeams: make(map[uuid.UUID][]problemsmodel.ProblemTeam),
	}
}

func newMockTeamsRepo() *mockTeamsRepo {
	return &mockTeamsRepo{userTeams: make(map[string][]organizationsmodel.Team)}
}

func newMockUsersUC() *mockUsersUC {
	return &mockUsersUC{users: make(map[uuid.UUID]accessmodel.User)}
}

type permissionsFixture struct {
	usersUC   *mockUsersUC
	contests  *mockContestsRepo
	orgs      *mockOrgsRepo
	teams     *mockTeamsRepo
	problems  *mockProblemsRepo
	permUC    *access.PermissionsUseCase
	orgID     uuid.UUID
	contestID uuid.UUID
	problemID uuid.UUID
}

type mockOrgsRepo struct {
	OrganizationsRepo
	members       map[string]organizationsmodel.OrganizationMember // key: orgID:userID
	organizations map[string]organizationsmodel.Organization       // key: login
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

type mockProblemsRepo struct {
	ProblemsRepo
	problems     map[uuid.UUID]problemsmodel.Problem
	members      map[string]problemsmodel.ProblemMember // key: problemID:userID
	problemTeams map[uuid.UUID][]problemsmodel.ProblemTeam
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

type mockTeamsRepo struct {
	TeamsRepo
	userTeams map[string][]organizationsmodel.Team // key: userID:orgID
}

func (m *mockTeamsRepo) GetUserTeamsByOrganization(ctx context.Context, userID, orgID uuid.UUID) ([]organizationsmodel.Team, error) {
	key := userID.String() + ":" + orgID.String()
	return m.userTeams[key], nil
}

type mockUsersUC struct {
	UsersUC
	users map[uuid.UUID]accessmodel.User
}

func (m *mockUsersUC) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
}

func (organizationTestStore) CreateNotification(context.Context, *notificationsmodel.CreateNotificationInput) (*notificationsmodel.Notification, error) {
	return nil, pkg.NotImplemented
}

func (organizationTestStore) LockOrganization(context.Context, uuid.UUID) error { return nil }
