package users

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
)

type Repo interface {
	CreateUser(ctx context.Context, user *models.UserCreation) (uuid.UUID, error)
	GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error)
	SearchUsers(ctx context.Context, search *models.UsersSearch) (*models.UsersList, error)
}

type UsersUseCase struct {
	usersRepo Repo
}

func NewUseCase(usersRepo Repo) *UsersUseCase {
	return &UsersUseCase{
		usersRepo: usersRepo,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, user *models.UserCreation) (uuid.UUID, error) {
	return u.usersRepo.CreateUser(ctx, user)
}

func (u *UsersUseCase) GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return u.usersRepo.GetUserById(ctx, id)
}

func (u *UsersUseCase) GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error) {
	return u.usersRepo.GetUserByKratosId(ctx, kratosId)
}

func (u *UsersUseCase) SearchUsers(ctx context.Context, search *models.UsersSearch) (*models.UsersList, error) {
	return u.usersRepo.SearchUsers(ctx, search)
}
