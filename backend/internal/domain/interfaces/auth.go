package interfaces

import (
	"context"
	"time"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

type AuthRepo interface {
	CreateSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (models.Session, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
}

type AuthUC interface {
	Register(ctx context.Context, username, email, password string) (models.User, uuid.UUID, error)
	Login(ctx context.Context, identifier, password string) (models.User, uuid.UUID, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	Authenticate(ctx context.Context, sessionID uuid.UUID) (models.User, error)
}
