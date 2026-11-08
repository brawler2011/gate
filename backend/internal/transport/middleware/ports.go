package middleware

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

type AuthUC interface {
	Authenticate(ctx context.Context, sessionID uuid.UUID) (accessmodel.User, error)
}

type UsersUC interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
}
