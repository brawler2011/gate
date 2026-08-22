package interfaces

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

type AuthRepo interface {
	CreateSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (models.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	DeleteSessionsByUserID(ctx context.Context, userID uuid.UUID) error
	UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
	CleanupExpiredSessions(ctx context.Context, hardLimitCutoff time.Time) error

	CreateAuthToken(ctx context.Context, params models.CreateAuthTokenParams) error
	GetAuthTokenByHash(ctx context.Context, tokenHash string, tokenType models.AuthTokenType) (models.AuthToken, error)
	DeleteAuthToken(ctx context.Context, id uuid.UUID) error
	DeleteAuthTokensByUserIdAndType(ctx context.Context, userID uuid.UUID, tokenType models.AuthTokenType) error
	CleanupExpiredAuthTokens(ctx context.Context) error
}

type AuthUC interface {
	Register(ctx context.Context, username, email, password string) (models.User, error)
	Login(ctx context.Context, identifier, password string) (models.User, uuid.UUID, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	Authenticate(ctx context.Context, sessionID uuid.UUID) (models.User, error)
	CleanupExpiredSessions(ctx context.Context) error

	VerifyEmail(ctx context.Context, token string) (models.User, uuid.UUID, error)
	ResendVerificationEmail(ctx context.Context, identifier string) error
	ForgotPassword(ctx context.Context, identifier string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) (uuid.UUID, error)
	RequestEmailChange(ctx context.Context, userID uuid.UUID, password, newEmail string) error
	ConfirmEmailChange(ctx context.Context, token string) error
}

