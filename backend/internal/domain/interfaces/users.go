package interfaces

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UsersRepo interface {
	CreateUser(ctx context.Context, params models.CreateUserParams) error
	GetUserById(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	GetUserByUsernameOrEmail(ctx context.Context, identifier string) (models.User, error)
	ListUsers(ctx context.Context, filter models.UsersListFilter) (models.UsersList, error)
	UpdateUser(ctx context.Context, params models.UpdateUserParams) error
	ClaimTemporaryUser(ctx context.Context, id, claimedByUserID uuid.UUID, claimedAt time.Time) error
	ListClaimedAccountsByUserId(ctx context.Context, claimedByUserID uuid.UUID) ([]models.User, error)
	ListExistingUsernamesByPrefix(ctx context.Context, prefix string) ([]string, error)
	WithTx(tx pgx.Tx) UsersRepo
}

type UsersUC interface {
	CreateUser(ctx context.Context, input models.CreateUserInput) (uuid.UUID, error)
	GetUserById(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	ListUsers(ctx context.Context, filter models.UsersListFilter) (models.UsersList, error)
	UpdateUser(ctx context.Context, input models.UpdateUserInput) error
	ClaimTemporaryUser(ctx context.Context, caller models.User, input models.ClaimTemporaryUserInput) (models.ClaimTemporaryUserResult, error)
	ListClaimedAccounts(ctx context.Context, userID uuid.UUID) ([]models.ClaimedAccountItem, error)
}

