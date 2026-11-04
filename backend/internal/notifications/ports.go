package notifications

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/google/uuid"
)

type Transactions interface {
	Within(context.Context, func(TxStore) error) error
}

type TxStore interface {
	UsersRepo
	MarkAsRead(ctx context.Context, id, userID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error
}

type UsersRepo interface {
	GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error)
}
