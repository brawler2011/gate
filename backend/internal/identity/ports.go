package identity

import (
	"context"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/google/uuid"
)

type AuthRepo interface {
	GetAuthTokenByHash(context.Context, string, identitymodel.AuthTokenType) (identitymodel.AuthToken, error)
	DeleteAuthToken(context.Context, uuid.UUID) error
	CleanupExpiredAuthTokens(context.Context) error
	CreateSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (identitymodel.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	DeleteSessionsByUserID(ctx context.Context, userID uuid.UUID) error
	UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
	CleanupExpiredSessions(ctx context.Context, hardLimitCutoff time.Time) error

	CreateAuthToken(ctx context.Context, params identitymodel.CreateAuthTokenParams) error

	DeleteAuthTokensByUserIdAndType(ctx context.Context, userID uuid.UUID, tokenType identitymodel.AuthTokenType) error
}

type ContestsRepo interface {
	ListUserContestMemberships(context.Context, uuid.UUID) ([]contestsmodel.UserContestMembership, error)
	AddContestMemberIfNotExists(context.Context, uuid.UUID, uuid.UUID, string) error
}

type OutboxRepo interface {
	eventdata.Writer
}

type UsersRepo interface {
	ClaimTemporaryUser(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	SetUserEmailVerified(context.Context, uuid.UUID, bool) error
	CreateUser(ctx context.Context, params identitymodel.CreateUserParams) error
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
	GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error)
	GetUserByUsernameOrEmail(ctx context.Context, identifier string) (accessmodel.User, error)
	UpdateUser(ctx context.Context, params identitymodel.UpdateUserParams) error

	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateUserEmail(ctx context.Context, id uuid.UUID, email string, isVerified bool) error
}
