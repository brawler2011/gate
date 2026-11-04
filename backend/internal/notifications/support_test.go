package notifications

import (
	"context"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
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

func newMockUsersRepoForTemp() *mockUsersRepoForTemp {
	return &mockUsersRepoForTemp{
		users:        make(map[uuid.UUID]accessmodel.User),
		usersByName:  make(map[string]accessmodel.User),
		usersByEmail: make(map[string]accessmodel.User),
		claimedUsers: make(map[uuid.UUID][]accessmodel.User),
	}
}

type mockUsersRepoForTemp struct {
	UsersRepo
	users        map[uuid.UUID]accessmodel.User
	usersByName  map[string]accessmodel.User
	usersByEmail map[string]accessmodel.User
	claimedUsers map[uuid.UUID][]accessmodel.User
}

func (m *mockUsersRepoForTemp) CreateUser(ctx context.Context, params identitymodel.CreateUserParams) error {
	user := accessmodel.User{
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

func (m *mockUsersRepoForTemp) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
}

func (m *mockUsersRepoForTemp) GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error) {
	if u, ok := m.usersByName[username]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
}

func (m *mockUsersRepoForTemp) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (accessmodel.User, error) {
	if u, ok := m.usersByName[identifier]; ok {
		return u, nil
	}
	if u, ok := m.usersByEmail[identifier]; ok {
		return u, nil
	}
	return accessmodel.User{}, pkg.ErrNotFound
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
