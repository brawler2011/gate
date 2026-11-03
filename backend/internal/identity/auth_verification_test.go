package identity

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type mockEmailService struct {
	lastVerificationEmail struct {
		ToEmail  string
		Username string
		Token    string
	}
	lastPasswordResetEmail struct {
		ToEmail  string
		Username string
		Token    string
	}
	lastEmailChangeVerification struct {
		ToNewEmail string
		Username   string
		Token      string
	}
	lastEmailChangeAlert struct {
		ToOldEmail string
		Username   string
		NewEmail   string
	}
}

func (m *mockEmailService) SendVerificationEmail(ctx context.Context, toEmail, username, token string) error {
	m.lastVerificationEmail.ToEmail = toEmail
	m.lastVerificationEmail.Username = username
	m.lastVerificationEmail.Token = token
	return nil
}

func (m *mockEmailService) SendPasswordResetEmail(ctx context.Context, toEmail, username, token string) error {
	m.lastPasswordResetEmail.ToEmail = toEmail
	m.lastPasswordResetEmail.Username = username
	m.lastPasswordResetEmail.Token = token
	return nil
}

func (m *mockEmailService) SendEmailChangeVerification(ctx context.Context, toNewEmail, username, token string) error {
	m.lastEmailChangeVerification.ToNewEmail = toNewEmail
	m.lastEmailChangeVerification.Username = username
	m.lastEmailChangeVerification.Token = token
	return nil
}

func (m *mockEmailService) SendEmailChangeAlert(ctx context.Context, toOldEmail, username, newEmail string) error {
	m.lastEmailChangeAlert.ToOldEmail = toOldEmail
	m.lastEmailChangeAlert.Username = username
	m.lastEmailChangeAlert.NewEmail = newEmail
	return nil
}

func (m *mockEmailService) SendOrgInvitationEmail(ctx context.Context, toEmail, username, inviterUsername, orgName, orgLogin, role string) error {
	return nil
}

func (m *mockEmailService) SendOrgJoinRequestEmail(ctx context.Context, toEmail, reviewerUsername, applicantUsername, orgName, orgLogin string) error {
	return nil
}

func (m *mockEmailService) SendOrgJoinRequestResolvedEmail(ctx context.Context, toEmail, username, orgName, orgLogin string, approved bool) error {
	return nil
}

func (m *mockEmailService) SendContestJoinRequestEmail(ctx context.Context, toEmail, reviewerUsername, applicantUsername, contestTitle, orgLogin, contestLogin string) error {
	return nil
}

func (m *mockEmailService) SendContestJoinRequestResolvedEmail(ctx context.Context, toEmail, username, contestTitle, orgLogin, contestLogin string, approved bool) error {
	return nil
}

type fullMockAuthRepo struct {
	sessions   map[uuid.UUID]identitymodel.Session
	tokens     map[uuid.UUID]identitymodel.AuthToken
	tokenIndex map[string]identitymodel.AuthToken // key: token_hash + ":" + token_type
}

func newFullMockAuthRepo() *fullMockAuthRepo {
	return &fullMockAuthRepo{
		sessions:   make(map[uuid.UUID]identitymodel.Session),
		tokens:     make(map[uuid.UUID]identitymodel.AuthToken),
		tokenIndex: make(map[string]identitymodel.AuthToken),
	}
}

func (m *fullMockAuthRepo) CreateSession(ctx context.Context, sessionID, userID uuid.UUID, expiresAt time.Time) error {
	m.sessions[sessionID] = identitymodel.Session{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *fullMockAuthRepo) GetSession(ctx context.Context, sessionID uuid.UUID) (identitymodel.Session, error) {
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return identitymodel.Session{}, pkg.ErrNotFound
}

func (m *fullMockAuthRepo) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *fullMockAuthRepo) DeleteSessionsByUserID(ctx context.Context, userID uuid.UUID) error {
	for id, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *fullMockAuthRepo) UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error {
	if s, ok := m.sessions[sessionID]; ok {
		s.ExpiresAt = expiresAt
		m.sessions[sessionID] = s
	}
	return nil
}

func (m *fullMockAuthRepo) CleanupExpiredSessions(ctx context.Context, hardLimitCutoff time.Time) error {
	return nil
}

func (m *fullMockAuthRepo) CreateAuthToken(ctx context.Context, params identitymodel.CreateAuthTokenParams) error {
	tok := identitymodel.AuthToken{
		ID:        params.ID,
		UserID:    params.UserID,
		TokenType: params.TokenType,
		TokenHash: params.TokenHash,
		Payload:   params.Payload,
		ExpiresAt: params.ExpiresAt,
		CreatedAt: time.Now(),
	}
	m.tokens[tok.ID] = tok
	m.tokenIndex[tok.TokenHash+":"+string(tok.TokenType)] = tok
	return nil
}

func (m *fullMockAuthRepo) GetAuthTokenByHash(ctx context.Context, tokenHash string, tokenType identitymodel.AuthTokenType) (identitymodel.AuthToken, error) {
	if tok, ok := m.tokenIndex[tokenHash+":"+string(tokenType)]; ok {
		return tok, nil
	}
	return identitymodel.AuthToken{}, pkg.ErrNotFound
}

func (m *fullMockAuthRepo) DeleteAuthToken(ctx context.Context, id uuid.UUID) error {
	if tok, ok := m.tokens[id]; ok {
		delete(m.tokens, id)
		delete(m.tokenIndex, tok.TokenHash+":"+string(tok.TokenType))
	}
	return nil
}

func (m *fullMockAuthRepo) DeleteAuthTokensByUserIdAndType(ctx context.Context, userID uuid.UUID, tokenType identitymodel.AuthTokenType) error {
	for id, tok := range m.tokens {
		if tok.UserID == userID && tok.TokenType == tokenType {
			delete(m.tokens, id)
			delete(m.tokenIndex, tok.TokenHash+":"+string(tok.TokenType))
		}
	}
	return nil
}

func (m *fullMockAuthRepo) CleanupExpiredAuthTokens(ctx context.Context) error {
	return nil
}

type fullMockUsersRepo struct {
	identityUsersStore
	users        map[uuid.UUID]accessmodel.User
	usersByName  map[string]accessmodel.User
	usersByEmail map[string]accessmodel.User
}

func newFullMockUsersRepo() *fullMockUsersRepo {
	return &fullMockUsersRepo{
		users:        make(map[uuid.UUID]accessmodel.User),
		usersByName:  make(map[string]accessmodel.User),
		usersByEmail: make(map[string]accessmodel.User),
	}
}

func (m *fullMockUsersRepo) CreateUser(ctx context.Context, params identitymodel.CreateUserParams) error {
	u := accessmodel.User{
		Id:              params.Id,
		Username:        params.Username,
		Role:            params.Role,
		PasswordHash:    params.PasswordHash,
		Email:           params.Email,
		AvatarUrl:       params.AvatarUrl,
		ExpiresAt:       params.ExpiresAt,
		IsEmailVerified: params.IsEmailVerified,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	m.users[u.Id] = u
	m.usersByName[u.Username] = u
	if u.Email != nil {
		m.usersByEmail[*u.Email] = u
	}
	return nil
}

func (m *fullMockUsersRepo) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
}

func (m *fullMockUsersRepo) GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error) {
	if u, ok := m.usersByName[username]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
}

func (m *fullMockUsersRepo) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (accessmodel.User, error) {
	if u, ok := m.usersByName[identifier]; ok {
		return u, nil
	}
	if u, ok := m.usersByEmail[identifier]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
}

func (m *fullMockUsersRepo) SetUserEmailVerified(ctx context.Context, id uuid.UUID, isVerified bool) error {
	if u, ok := m.users[id]; ok {
		u.IsEmailVerified = isVerified
		m.users[id] = u
		m.usersByName[u.Username] = u
		if u.Email != nil {
			m.usersByEmail[*u.Email] = u
		}
		return nil
	}
	return pkg.ErrNotFound
}

func (m *fullMockUsersRepo) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	if u, ok := m.users[id]; ok {
		u.PasswordHash = passwordHash
		m.users[id] = u
		m.usersByName[u.Username] = u
		return nil
	}
	return pkg.ErrNotFound
}

func (m *fullMockUsersRepo) UpdateUserEmail(ctx context.Context, id uuid.UUID, email string, isVerified bool) error {
	if u, ok := m.users[id]; ok {
		if u.Email != nil {
			delete(m.usersByEmail, *u.Email)
		}
		u.Email = &email
		u.IsEmailVerified = isVerified
		m.users[id] = u
		m.usersByName[u.Username] = u
		m.usersByEmail[email] = u
		return nil
	}
	return pkg.ErrNotFound
}

func (m *fullMockUsersRepo) UpdateUser(ctx context.Context, params identitymodel.UpdateUserParams) error {
	u, ok := m.users[params.Id]
	if !ok {
		return pkg.ErrNotFound
	}
	if params.Username != nil {
		delete(m.usersByName, u.Username)
		u.Username = *params.Username
		m.usersByName[u.Username] = u
	}
	if params.Email != nil {
		if u.Email != nil {
			delete(m.usersByEmail, *u.Email)
		}
		u.Email = params.Email
		m.usersByEmail[*params.Email] = u
	}
	if params.Role != nil {
		u.Role = *params.Role
	}
	if params.AvatarUrl != nil {
		u.AvatarUrl = params.AvatarUrl
	}
	if params.ExpiresAt != nil {
		u.ExpiresAt = params.ExpiresAt
	}
	if params.IsEmailVerified != nil {
		u.IsEmailVerified = *params.IsEmailVerified
	}
	m.users[u.Id] = u
	return nil
}

func TestRegistrationAndEmailVerification(t *testing.T) {
	ctx := context.Background()
	usersRepo := newFullMockUsersRepo()
	authRepo := newFullMockAuthRepo()
	emailSvc := &mockEmailService{}

	authUC := NewAuthUseCase(usersRepo, authRepo, testTransactions[IdentityTxStore]{identityTestStore{identityUsersStore: usersRepo, identityAuthStore: authRepo}}, emailSvc)

	// 1. Register new user
	user, err := authUC.Register(ctx, "testuser", "test@example.com", "SecretPass123")
	require.NoError(t, err)
	assert.False(t, user.IsEmailVerified)
	assert.Equal(t, "test@example.com", emailSvc.lastVerificationEmail.ToEmail)
	assert.NotEmpty(t, emailSvc.lastVerificationEmail.Token)

	// 2. Login before verification should fail
	_, _, err = authUC.Login(ctx, "testuser", "SecretPass123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email not verified")

	// 3. Verify email with token
	token := emailSvc.lastVerificationEmail.Token
	verifiedUser, sessionID, err := authUC.VerifyEmail(ctx, token)
	require.NoError(t, err)
	assert.True(t, verifiedUser.IsEmailVerified)
	assert.NotEqual(t, uuid.Nil, sessionID)

	// 4. Now login succeeds
	_, loginSessionID, err := authUC.Login(ctx, "testuser", "SecretPass123")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, loginSessionID)
}

func TestRegistrationRetryUnverifiedUser(t *testing.T) {
	ctx := context.Background()
	usersRepo := newFullMockUsersRepo()
	authRepo := newFullMockAuthRepo()
	emailSvc := &mockEmailService{}

	authUC := NewAuthUseCase(usersRepo, authRepo, testTransactions[IdentityTxStore]{identityTestStore{identityUsersStore: usersRepo, identityAuthStore: authRepo}}, emailSvc)

	// 1. Initial registration (email not verified yet)
	user1, err := authUC.Register(ctx, "unverifieduser", "unverified@example.com", "FirstPassword123")
	require.NoError(t, err)
	assert.False(t, user1.IsEmailVerified)
	token1 := emailSvc.lastVerificationEmail.Token
	require.NotEmpty(t, token1)

	// 2. Re-register with the same unverified username & email, updated password
	user2, err := authUC.Register(ctx, "unverifieduser", "unverified@example.com", "NewPassword456")
	require.NoError(t, err)
	assert.Equal(t, user1.Id, user2.Id)
	assert.False(t, user2.IsEmailVerified)
	token2 := emailSvc.lastVerificationEmail.Token
	require.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2)

	// 3. Old token should no longer work
	_, _, err = authUC.VerifyEmail(ctx, token1)
	require.Error(t, err)

	// 4. New token works and verifies the account
	verifiedUser, sessionID, err := authUC.VerifyEmail(ctx, token2)
	require.NoError(t, err)
	assert.True(t, verifiedUser.IsEmailVerified)
	assert.NotEqual(t, uuid.Nil, sessionID)

	// 5. Login with new password succeeds
	_, loginSessID, err := authUC.Login(ctx, "unverifieduser", "NewPassword456")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, loginSessID)

	// 6. Login with old password fails
	_, _, err = authUC.Login(ctx, "unverifieduser", "FirstPassword123")
	require.Error(t, err)

	// 7. Trying to register again after email is verified should fail with conflict
	_, err = authUC.Register(ctx, "unverifieduser", "different@example.com", "AnotherPass123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username already taken")

	_, err = authUC.Register(ctx, "otheruser", "unverified@example.com", "AnotherPass123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email already in use")
}

func TestPasswordResetFlow(t *testing.T) {
	ctx := context.Background()
	usersRepo := newFullMockUsersRepo()
	authRepo := newFullMockAuthRepo()
	emailSvc := &mockEmailService{}

	authUC := NewAuthUseCase(usersRepo, authRepo, testTransactions[IdentityTxStore]{identityTestStore{identityUsersStore: usersRepo, identityAuthStore: authRepo}}, emailSvc)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("OldPassword123"), bcrypt.DefaultCost)
	emailStr := "user@example.com"
	userID := uuid.New()
	_ = usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{
		Id:              userID,
		Username:        "resetuser",
		Role:            accessmodel.UserRoleUser,
		PasswordHash:    string(hashed),
		Email:           &emailStr,
		IsEmailVerified: true,
	})

	// Create an active session
	sessID := uuid.New()
	_ = authRepo.CreateSession(ctx, sessID, userID, time.Now().Add(time.Hour))

	// 1. Request password reset
	err := authUC.ForgotPassword(ctx, "resetuser")
	require.NoError(t, err)
	assert.Equal(t, emailStr, emailSvc.lastPasswordResetEmail.ToEmail)
	assert.NotEmpty(t, emailSvc.lastPasswordResetEmail.Token)

	// 2. Reset password with token
	token := emailSvc.lastPasswordResetEmail.Token
	err = authUC.ResetPassword(ctx, token, "NewPassword123")
	require.NoError(t, err)

	// 3. Old session must be deleted
	_, err = authRepo.GetSession(ctx, sessID)
	require.Error(t, err)

	// 4. Login with old password fails
	_, _, err = authUC.Login(ctx, "resetuser", "OldPassword123")
	require.Error(t, err)

	// 5. Login with new password succeeds
	_, newSessID, err := authUC.Login(ctx, "resetuser", "NewPassword123")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, newSessID)
}

func TestEmailChangeFlow(t *testing.T) {
	ctx := context.Background()
	usersRepo := newFullMockUsersRepo()
	authRepo := newFullMockAuthRepo()
	emailSvc := &mockEmailService{}

	authUC := NewAuthUseCase(usersRepo, authRepo, testTransactions[IdentityTxStore]{identityTestStore{identityUsersStore: usersRepo, identityAuthStore: authRepo}}, emailSvc)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("MyPassword123"), bcrypt.DefaultCost)
	oldEmail := "old@example.com"
	userID := uuid.New()
	_ = usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{
		Id:              userID,
		Username:        "changeemailuser",
		Role:            accessmodel.UserRoleUser,
		PasswordHash:    string(hashed),
		Email:           &oldEmail,
		IsEmailVerified: true,
	})

	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser})

	// 1. Request email change
	newEmail := "new@example.com"
	err := authUC.RequestEmailChange(ctx, userID, "MyPassword123", newEmail)
	require.NoError(t, err)
	assert.Equal(t, newEmail, emailSvc.lastEmailChangeVerification.ToNewEmail)
	assert.Equal(t, oldEmail, emailSvc.lastEmailChangeAlert.ToOldEmail)
	assert.NotEmpty(t, emailSvc.lastEmailChangeVerification.Token)

	// 2. Before confirmation, old email is still active
	u, err := usersRepo.GetUserById(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, oldEmail, *u.Email)

	// 3. Confirm email change
	token := emailSvc.lastEmailChangeVerification.Token
	err = authUC.ConfirmEmailChange(ctx, token)
	require.NoError(t, err)

	// 4. Email is now updated
	u, err = usersRepo.GetUserById(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, newEmail, *u.Email)
}

func TestAdminUserManagement(t *testing.T) {
	ctx := context.Background()
	usersRepo := newFullMockUsersRepo()
	authRepo := newFullMockAuthRepo()
	emailSvc := &mockEmailService{}

	usersUC := NewUsersUseCase(usersRepo, testTransactions[IdentityTxStore]{identityTestStore{identityUsersStore: usersRepo, identityAuthStore: authRepo}}, emailSvc)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("Pass12345"), bcrypt.DefaultCost)
	emailStr := "adminmanaged@example.com"
	userID := uuid.New()
	_ = usersRepo.CreateUser(ctx, identitymodel.CreateUserParams{
		Id:              userID,
		Username:        "targetuser",
		Role:            accessmodel.UserRoleUser,
		PasswordHash:    string(hashed),
		Email:           &emailStr,
		IsEmailVerified: false,
	})

	// 1. Admin direct email change without confirmation
	require.ErrorIs(t, usersUC.AdminSetPassword(ctx, "targetuser", "DeniedPassword99"), pkg.ErrUnauthenticated)
	ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: userID, Role: accessmodel.UserRoleUser})
	require.ErrorIs(t, usersUC.AdminSetPassword(ctx, "targetuser", "DeniedPassword99"), pkg.NoPermission)
	admin := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleAdmin}
	usersRepo.users[admin.Id] = admin
	ctx = accessmodel.WithUser(ctx, admin)

	err := usersUC.AdminChangeEmail(ctx, "targetuser", "direct@example.com", false)
	require.NoError(t, err)
	u, err := usersRepo.GetUserById(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "direct@example.com", *u.Email)
	assert.True(t, u.IsEmailVerified)

	// 2. Admin set password directly
	sessID := uuid.New()
	_ = authRepo.CreateSession(ctx, sessID, userID, time.Now().Add(time.Hour))
	err = usersUC.AdminSetPassword(ctx, "targetuser", "DirectNewPassword99")
	require.NoError(t, err)

	// Session was invalidated
	_, err = authRepo.GetSession(ctx, sessID)
	require.Error(t, err)

	// 3. Admin send password reset link
	err = usersUC.AdminSendPasswordReset(ctx, "targetuser")
	require.NoError(t, err)
	assert.Equal(t, "direct@example.com", emailSvc.lastPasswordResetEmail.ToEmail)
}

// The password can change while bcrypt runs outside the transaction.
type changedPasswordStore struct{ identityTestStore }

func (s changedPasswordStore) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	user, err := s.identityUsersStore.GetUserById(ctx, id)
	user.PasswordHash = "new password hash"
	return user, err
}
func TestLoginRejectsPasswordChangedBeforeSessionCommit(t *testing.T) {
	users, sessions := newFullMockUsersRepo(), newFullMockAuthRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("OldPassword123"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, users.CreateUser(context.Background(), identitymodel.CreateUserParams{Id: uuid.New(), Username: "login_race", Role: accessmodel.UserRoleUser, PasswordHash: string(hash), IsEmailVerified: true}))
	store := changedPasswordStore{identityTestStore{identityUsersStore: users, identityAuthStore: sessions}}
	uc := NewAuthUseCase(users, sessions, testTransactions[IdentityTxStore]{store}, nil)
	_, id, err := uc.Login(context.Background(), "login_race", "OldPassword123")
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	require.Equal(t, uuid.Nil, id)
	require.Empty(t, sessions.sessions)
}

func TestAccountCommandsRejectStaleAdministratorAndSelfPromotion(t *testing.T) {
	users := newFullMockUsersRepo()
	user := accessmodel.User{Id: uuid.New(), Username: "ordinary", Role: accessmodel.UserRoleUser}
	users.users[user.Id] = user
	uc := NewUsersUseCase(users, testTransactions[IdentityTxStore]{identityTestStore{identityUsersStore: users}}, nil)
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: user.Id, Role: accessmodel.UserRoleAdmin})
	_, err := uc.CreateUser(ctx, identitymodel.CreateUserInput{Username: "denied", Password: "Password123!", Role: accessmodel.UserRoleUser})
	require.ErrorIs(t, err, pkg.NoPermission)
	name := "renamed"
	other := uuid.New()
	require.ErrorIs(t, uc.UpdateUser(ctx, identitymodel.UpdateUserInput{Id: other, Username: &name}), pkg.NoPermission)
	role := accessmodel.UserRoleAdmin
	ctx = accessmodel.WithUser(ctx, user)
	require.ErrorIs(t, uc.UpdateUser(ctx, identitymodel.UpdateUserInput{Id: user.Id, Role: &role}), pkg.NoPermission)
	require.Equal(t, accessmodel.UserRoleUser, users.users[user.Id].Role)
}
