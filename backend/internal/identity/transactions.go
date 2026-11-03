package identity

import (
	"context"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/google/uuid"
)

type IdentityTxStore interface {
	CreateUser(ctx context.Context, params identitymodel.CreateUserParams) error
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
	GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error)
	GetUserByUsernameOrEmail(ctx context.Context, identifier string) (accessmodel.User, error)
	UpdateUser(ctx context.Context, params identitymodel.UpdateUserParams) error
	SetUserEmailVerified(ctx context.Context, id uuid.UUID, isVerified bool) error
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateUserEmail(ctx context.Context, id uuid.UUID, email string, isVerified bool) error
	ClaimTemporaryUser(ctx context.Context, id, claimedByUserID uuid.UUID, claimedAt time.Time) error
	ListUserContestMemberships(ctx context.Context, userID uuid.UUID) ([]contestsmodel.UserContestMembership, error)
	AddContestMemberIfNotExists(ctx context.Context, contestID, userID uuid.UUID, role string) error
	CreateSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (identitymodel.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	DeleteSessionsByUserID(ctx context.Context, userID uuid.UUID) error
	UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
	CleanupExpiredSessions(ctx context.Context, hardLimitCutoff time.Time) error
	CreateAuthToken(ctx context.Context, params identitymodel.CreateAuthTokenParams) error
	GetAuthTokenByHash(ctx context.Context, tokenHash string, tokenType identitymodel.AuthTokenType) (identitymodel.AuthToken, error)
	DeleteAuthToken(ctx context.Context, id uuid.UUID) error
	DeleteAuthTokensByUserIdAndType(ctx context.Context, userID uuid.UUID, tokenType identitymodel.AuthTokenType) error
	CleanupExpiredAuthTokens(ctx context.Context) error
}

type IdentityTransactions interface {
	Within(context.Context, func(IdentityTxStore) error) error
}
