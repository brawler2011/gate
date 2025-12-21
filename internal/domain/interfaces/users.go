package interfaces

import (
	"context"

	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UsersRepo interface {
	CreateUser(ctx context.Context, params models.CreateUserParams) error
	GetUserById(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserByKratosId(ctx context.Context, kratosId uuid.UUID) (models.User, error)
	ListUsers(ctx context.Context, filter models.UsersListFilter) (models.UsersList, error)
	UpdateUser(ctx context.Context, params models.UpdateUserParams) error
	WithTx(tx pgx.Tx) UsersRepo
}

type UsersUC interface {
	CreateUser(ctx context.Context, input models.CreateUserInput) (uuid.UUID, error)
	GetUserById(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserByKratosId(ctx context.Context, kratosId uuid.UUID) (models.User, error)
	ListUsers(ctx context.Context, filter models.UsersListFilter) (models.UsersList, error)
	UpdateUser(ctx context.Context, input models.UpdateUserInput) error
}
