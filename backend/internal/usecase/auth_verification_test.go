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
	sessions   map[uuid.UUID]models.Session
	tokens     map[uuid.UUID]models.AuthToken
	tokenIndex map[string]models.AuthToken // key: token_hash + ":" + token_type
}

func newFullMockAuthRepo() *fullMockAuthRepo {
	return &fullMockAuthRepo{
		sessions:   make(map[uuid.UUID]models.Session),
		tokens:     make(map[uuid.UUID]models.AuthToken),
		tokenIndex: make(map[string]models.AuthToken),
	}
}

func (m *fullMockAuthRepo) CreateSession(ctx context.Context, sessionID, userID uuid.UUID, expiresAt time.Time) error {
	m.sessions[sessionID] = models.Session{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *fullMockAuthRepo) GetSession(ctx context.Context, sessionID uuid.UUID) (models.Session, error) {
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return models.Session{}, pkg.ErrNotFound
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

func (m *fullMockAuthRepo) CreateAuthToken(ctx context.Context, params models.CreateAuthTokenParams) error {
	tok := models.AuthToken{
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

func (m *fullMockAuthRepo) GetAuthTokenByHash(ctx context.Context, tokenHash string, tokenType models.AuthTokenType) (models.AuthToken, error) {
	if tok, ok := m.tokenIndex[tokenHash+":"+string(tokenType)]; ok {
		return tok, nil
	}
	return models.AuthToken{}, pkg.ErrNotFound
}

func (m *fullMockAuthRepo) DeleteAuthToken(ctx context.Context, id uuid.UUID) error {
	if tok, ok := m.tokens[id]; ok {
		delete(m.tokens, id)
		delete(m.tokenIndex, tok.TokenHash+":"+string(tok.TokenType))
	}
	return nil
}

func (m *fullMockAuthRepo) DeleteAuthTokensByUserIdAndType(ctx context.Context, userID uuid.UUID, tokenType models.AuthTokenType) error {
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
	interfaces.UsersRepo
	users        map[uuid.UUID]models.User
	usersByName  map[string]models.User
	usersByEmail map[string]models.User
}

func newFullMockUsersRepo() *fullMockUsersRepo {
	return &fullMockUsersRepo{
		users:        make(map[uuid.UUID]models.User),
		usersByName:  make(map[string]models.User),
		usersByEmail: make(map[string]models.User),
	}
}

func (m *fullMockUsersRepo) CreateUser(ctx context.Context, params models.CreateUserParams) error {
	u := models.User{
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

func (m *fullMockUsersRepo) GetUserById(ctx context.Context, id uuid.UUID) (models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return models.User{}, pkg.ErrNotFound
}

func (m *fullMockUsersRepo) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	if u, ok := m.usersByName[username]; ok {
		return u, nil
	}
	return models.User{}, pkg.ErrNotFound
}

func (m *fullMockUsersRepo) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (models.User, error) {
	if u, ok := m.usersByName[identifier]; ok {
		return u, nil
	}
	if u, ok := m.usersByEmail[identifier]; ok {
		return u, nil
	}
	return models.User{}, pkg.ErrNotFound
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

func (m *fullMockUsersRepo) UpdateUser(ctx context.Context, params models.UpdateUserParams) error {
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

func (m *fullMockUsersRepo) WithTx(tx pgx.Tx) interfaces.UsersRepo {
	return m
}

func TestRegistrationAndEmailVerification(t *testing.T) {
	ctx := context.Background()
	usersRepo := newFullMockUsersRepo()
	authRepo := newFullMockAuthRepo()
	emailSvc := &mockEmailService{}
	txManager := &mockTransactorNoop{}

	authUC := NewAuthUseCase(usersRepo, authRepo, txManager, emailSvc)

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
	txManager := &mockTransactorNoop{}

	authUC := NewAuthUseCase(usersRepo, authRepo, txManager, emailSvc)

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
	txManager := &mockTransactorNoop{}

	authUC := NewAuthUseCase(usersRepo, authRepo, txManager, emailSvc)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("OldPassword123"), bcrypt.DefaultCost)
	emailStr := "user@example.com"
	userID := uuid.New()
	_ = usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:              userID,
		Username:        "resetuser",
		Role:            models.UserRoleUser,
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
	txManager := &mockTransactorNoop{}

	authUC := NewAuthUseCase(usersRepo, authRepo, txManager, emailSvc)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("MyPassword123"), bcrypt.DefaultCost)
	oldEmail := "old@example.com"
	userID := uuid.New()
	_ = usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:              userID,
		Username:        "changeemailuser",
		Role:            models.UserRoleUser,
		PasswordHash:    string(hashed),
		Email:           &oldEmail,
		IsEmailVerified: true,
	})

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
	txManager := &mockTransactorNoop{}

	usersUC := NewUsersUseCase(usersRepo, nil, nil, txManager, authRepo, emailSvc)

	hashed, _ := bcrypt.GenerateFromPassword([]byte("Pass12345"), bcrypt.DefaultCost)
	emailStr := "adminmanaged@example.com"
	userID := uuid.New()
	_ = usersRepo.CreateUser(ctx, models.CreateUserParams{
		Id:              userID,
		Username:        "targetuser",
		Role:            models.UserRoleUser,
		PasswordHash:    string(hashed),
		Email:           &emailStr,
		IsEmailVerified: false,
	})

	// 1. Admin direct email change without confirmation
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
