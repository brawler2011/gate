package identity

import (
	"context"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/google/uuid"
)

type AuthRepo interface {
	GetSession(ctx context.Context, sessionID uuid.UUID) (identitymodel.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
	CleanupExpiredSessions(ctx context.Context, hardLimitCutoff time.Time) error
}

type UsersRepo interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
	GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error)
	GetUserByUsernameOrEmail(ctx context.Context, identifier string) (accessmodel.User, error)
}
