package users

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
)

type Repo interface {
	CreateUser(ctx context.Context, params *models.CreateUserParams) error
	GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error)
	ListUsers(ctx context.Context, filter *models.UsersFilter) (*models.UsersList, error)
}

type UsersUseCase struct {
	repo Repo
}

func NewUseCase(repo Repo) *UsersUseCase {
	return &UsersUseCase{
		repo: repo,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, input *models.CreateUserInput) (uuid.UUID, error) {
	id := uuid.New()

	params := &models.CreateUserParams{
		Id:       id,
		Username: input.Username,
		Role:     input.Role,
		KratosId: input.KratosId,
	}

	if err := u.repo.CreateUser(ctx, params); err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (u *UsersUseCase) GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return u.repo.GetUserById(ctx, id)
}

func (u *UsersUseCase) GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error) {
	return u.repo.GetUserByKratosId(ctx, kratosId)
}

func (u *UsersUseCase) ListUsers(ctx context.Context, filter *models.UsersFilter) (*models.UsersList, error) {
	return u.repo.ListUsers(ctx, filter)
}
