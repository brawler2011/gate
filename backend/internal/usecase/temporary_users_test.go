package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type mockUsersRepoForTemp struct {
	interfaces.UsersRepo
	users        map[uuid.UUID]models.User
	usersByName  map[string]models.User
	usersByEmail map[string]models.User
	claimedUsers map[uuid.UUID][]models.User
}

func newMockUsersRepoForTemp() *mockUsersRepoForTemp {
	return &mockUsersRepoForTemp{
		users:        make(map[uuid.UUID]models.User),
		usersByName:  make(map[string]models.User),
		usersByEmail: make(map[string]models.User),
		claimedUsers: make(map[uuid.UUID][]models.User),
	}
}

func (m *mockUsersRepoForTemp) CreateUser(ctx context.Context, params models.CreateUserParams) error {
	user := models.User{
		Id:           params.Id,
		Username:     params.Username,
		Role:         params.Role,
		PasswordHash: params.PasswordHash,
		Email:        params.Email,
		AvatarUrl:    params.AvatarUrl,
		ExpiresAt:    params.ExpiresAt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.users[user.Id] = user
	m.usersByName[user.Username] = user
	if user.Email != nil {
		m.usersByEmail[*user.Email] = user
	}
	return nil
}

func (m *mockUsersRepoForTemp) GetUserById(ctx context.Context, id uuid.UUID) (models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return models.User{}, pkg.ErrNotFound
}

func (m *mockUsersRepoForTemp) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	if u, ok := m.usersByName[username]; ok {
		return u, nil
	}
	return models.User{}, pkg.ErrNotFound
}

func (m *mockUsersRepoForTemp) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (models.User, error) {
	if u, ok := m.usersByName[identifier]; ok {
		return u, nil
	}
	if u, ok := m.usersByEmail[identifier]; ok {
		return u, nil
	}
	return models.User{}, pkg.ErrNotFound
}

func (m *mockUsersRepoForTemp) ListExistingUsernamesByPrefix(ctx context.Context, prefix string) ([]string, error) {
	var result []string
	for name := range m.usersByName {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			result = append(result, name)
		}
	}
	return result, nil
}

func (m *mockUsersRepoForTemp) ClaimTemporaryUser(ctx context.Context, id, claimedByUserID uuid.UUID, claimedAt time.Time) error {
	u, ok := m.users[id]
	if !ok {
		return pkg.ErrNotFound
	}
	u.ClaimedByUserID = &claimedByUserID
	u.ClaimedAt = &claimedAt
	m.users[id] = u
	m.usersByName[u.Username] = u
	m.claimedUsers[claimedByUserID] = append(m.claimedUsers[claimedByUserID], u)
	return nil
}

func (m *mockUsersRepoForTemp) ListClaimedAccountsByUserId(ctx context.Context, claimedByUserID uuid.UUID) ([]models.User, error) {
	return m.claimedUsers[claimedByUserID], nil
}

func (m *mockUsersRepoForTemp) WithTx(tx pgx.Tx) interfaces.UsersRepo {
	return m
}

type mockTransactorNoop struct{}

func (m *mockTransactorNoop) WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return fn(ctx, nil)
}

func TestBatchCreateOrganizationUsers_Success(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	usersRepo := newMockUsersRepoForTemp()
	txManager := &mockTransactorNoop{}

	adminUser := models.User{Id: uuid.New(), Username: "admin_user", Role: models.UserRoleUser}
	usersRepo.users[adminUser.Id] = adminUser
	f.usersUC.users[adminUser.Id] = adminUser

	// Add admin to org as owner
	f.orgs.members[f.orgID.String()+":"+adminUser.Id.String()] = models.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         adminUser.Id,
		Role:           models.OrgRoleOwner,
	}

	orgsUC := NewOrganizationsUseCase(f.orgs, usersRepo, f.permUC, txManager)

	ttl := int32(30)
	input := models.BatchCreateOrganizationUsersInput{
		OrgLogin: "test-org",
		Prefix:   "olymp2026_",
		Count:    10,
		TTLDays:  &ttl,
	}

	res, err := orgsUC.BatchCreateUsers(ctx, input, adminUser.Id)
	require.NoError(t, err)
	require.Len(t, res.Users, 10)

	// Verify sequential names prefix_01 to prefix_10
	assert.Equal(t, "olymp2026_01", res.Users[0].Username)
	assert.Equal(t, "olymp2026_10", res.Users[9].Username)

	for _, u := range res.Users {
		assert.NotEmpty(t, u.Password)
		assert.NotNil(t, u.ExpiresAt)
		assert.True(t, u.ExpiresAt.After(time.Now()))

		// Check member was created in repo
		createdUser, err := usersRepo.GetUserByUsername(ctx, u.Username)
		require.NoError(t, err)
		assert.Equal(t, u.Username, createdUser.Username)
		assert.True(t, createdUser.IsTemporary())
		assert.False(t, createdUser.IsExpired())
	}
}

func TestBatchCreateOrganizationUsers_CollisionHandling(t *testing.T) {
	ctx := context.Background()
	f := setupPermissionsFixture()

	usersRepo := newMockUsersRepoForTemp()
	txManager := &mockTransactorNoop{}

	adminUser := models.User{Id: uuid.New(), Username: "admin_user", Role: models.UserRoleUser}
	usersRepo.users[adminUser.Id] = adminUser
	f.usersUC.users[adminUser.Id] = adminUser

	f.orgs.members[f.orgID.String()+":"+adminUser.Id.String()] = models.OrganizationMember{
		OrganizationID: f.orgID,
		UserID:         adminUser.Id,
		Role:           models.OrgRoleOwner,
	}

	// Pre-create olymp2026_01
	_ = usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:           uuid.New(),
		Username:     "olymp2026_01",
		Role:         models.UserRoleUser,
		PasswordHash: "dummy",
	})

	orgsUC := NewOrganizationsUseCase(f.orgs, usersRepo, f.permUC, txManager)

	input := models.BatchCreateOrganizationUsersInput{
		OrgLogin: "test-org",
		Prefix:   "olymp2026_",
		Count:    2,
	}

	res, err := orgsUC.BatchCreateUsers(ctx, input, adminUser.Id)
	require.NoError(t, err)
	require.Len(t, res.Users, 2)

	// Since 01 exists, it should generate 02 and 03
	assert.Equal(t, "olymp2026_02", res.Users[0].Username)
	assert.Equal(t, "olymp2026_03", res.Users[1].Username)
}

func TestClaimTemporaryUser_Success(t *testing.T) {
	ctx := context.Background()
	usersRepo := newMockUsersRepoForTemp()
	contestsRepo := newMockContestsRepo()
	txManager := &mockTransactorNoop{}

	usersUC := NewUsersUseCase(usersRepo, contestsRepo, nil, txManager)

	// Permanent user
	permUser := models.User{
		Id:       uuid.New(),
		Username: "alice_permanent",
		Role:     models.UserRoleUser,
	}
	usersRepo.users[permUser.Id] = permUser
	usersRepo.usersByName[permUser.Username] = permUser

	// Temporary user with password
	tempPassword := "SecretPass123"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	expiresAt := time.Now().Add(24 * time.Hour)
	tempUserID := uuid.New()

	_ = usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:           tempUserID,
		Username:     "temp_olymp_01",
		Role:         models.UserRoleUser,
		PasswordHash: string(hashed),
		ExpiresAt:    &expiresAt,
	})

	// Temporary user was member of a contest
	contestID := uuid.New()
	contestsRepo.members[contestID.String()+":"+tempUserID.String()] = models.ContestMember{
		ContestID: contestID,
		UserID:    tempUserID,
		Role:      models.ContestRoleParticipant,
	}

	// Claim temporary user
	res, err := usersUC.ClaimTemporaryUser(ctx, permUser, models.ClaimTemporaryUserInput{
		Username: "temp_olymp_01",
		Password: tempPassword,
	})
	require.NoError(t, err)
	assert.Equal(t, tempUserID, res.ClaimedUserID)
	assert.Equal(t, "temp_olymp_01", res.ClaimedUsername)

	// Verify temporary user is now marked claimed
	updatedTemp, err := usersRepo.GetUserById(ctx, tempUserID)
	require.NoError(t, err)
	assert.True(t, updatedTemp.IsClaimed())
	assert.Equal(t, permUser.Id, *updatedTemp.ClaimedByUserID)

	// Verify cannot claim twice
	_, err = usersUC.ClaimTemporaryUser(ctx, permUser, models.ClaimTemporaryUserInput{
		Username: "temp_olymp_01",
		Password: tempPassword,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already been claimed")

	// Verify list claimed accounts
	claimedList, err := usersUC.ListClaimedAccounts(ctx, permUser.Id)
	require.NoError(t, err)
	require.Len(t, claimedList, 1)
	assert.Equal(t, "temp_olymp_01", claimedList[0].Username)
}

func TestClaimTemporaryUser_Rejections(t *testing.T) {
	ctx := context.Background()
	usersRepo := newMockUsersRepoForTemp()
	contestsRepo := newMockContestsRepo()
	txManager := &mockTransactorNoop{}
	usersUC := NewUsersUseCase(usersRepo, contestsRepo, nil, txManager)

	permUser := models.User{Id: uuid.New(), Username: "alice_perm", Role: models.UserRoleUser}
	usersRepo.users[permUser.Id] = permUser
	usersRepo.usersByName[permUser.Username] = permUser

	// 1. Permanent account cannot be claimed as temporary
	permanentOther := models.User{Id: uuid.New(), Username: "bob_perm", Role: models.UserRoleUser, PasswordHash: "x"}
	usersRepo.users[permanentOther.Id] = permanentOther
	usersRepo.usersByName[permanentOther.Username] = permanentOther

	_, err := usersUC.ClaimTemporaryUser(ctx, permUser, models.ClaimTemporaryUserInput{
		Username: "bob_perm",
		Password: "any",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a temporary account")

	// 2. Wrong password
	expiresAt := time.Now().Add(24 * time.Hour)
	tempUser := models.User{
		Id:           uuid.New(),
		Username:     "temp_02",
		Role:         models.UserRoleUser,
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuu",
		ExpiresAt:    &expiresAt,
	}
	usersRepo.users[tempUser.Id] = tempUser
	usersRepo.usersByName[tempUser.Username] = tempUser

	_, err = usersUC.ClaimTemporaryUser(ctx, permUser, models.ClaimTemporaryUserInput{
		Username: "temp_02",
		Password: "wrong_password",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestAuth_ExpiredAccount(t *testing.T) {
	ctx := context.Background()
	usersRepo := newMockUsersRepoForTemp()
	authRepo := &mockAuthRepo{sessions: make(map[uuid.UUID]models.Session)}
	txManager := &mockTransactorNoop{}
	authUC := NewAuthUseCase(usersRepo, authRepo, txManager)

	pastExpiry := time.Now().Add(-1 * time.Hour)
	password := "GoodPassword1"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	expiredUser := models.User{
		Id:           uuid.New(),
		Username:     "expired_user",
		Role:         models.UserRoleUser,
		PasswordHash: string(hashed),
		ExpiresAt:    &pastExpiry,
	}
	usersRepo.users[expiredUser.Id] = expiredUser
	usersRepo.usersByName[expiredUser.Username] = expiredUser

	// Login should fail with account expired
	_, _, err := authUC.Login(ctx, "expired_user", password)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account has expired")
}

type mockAuthRepo struct {
	interfaces.AuthRepo
	sessions map[uuid.UUID]models.Session
}

func (m *mockAuthRepo) CreateSession(ctx context.Context, sessionID, userID uuid.UUID, expiresAt time.Time) error {
	m.sessions[sessionID] = models.Session{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *mockAuthRepo) GetSession(ctx context.Context, sessionID uuid.UUID) (models.Session, error) {
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return models.Session{}, pkg.ErrNotFound
}

func (m *mockAuthRepo) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockAuthRepo) UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error {
	if s, ok := m.sessions[sessionID]; ok {
		s.ExpiresAt = expiresAt
		m.sessions[sessionID] = s
	}
	return nil
}
