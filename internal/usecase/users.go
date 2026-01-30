package usecase

import (
	"context"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

type UsersUseCase struct {
	usersRepo  interfaces.UsersRepo
	outboxRepo interfaces.OutboxRepo
	txManager  interfaces.Transactor
}

func NewUsersUseCase(
	repo interfaces.UsersRepo,
	outboxRepo interfaces.OutboxRepo,
	txManager interfaces.Transactor,
) *UsersUseCase {
	return &UsersUseCase{
		usersRepo:  repo,
		outboxRepo: outboxRepo,
		txManager:  txManager,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, input models.CreateUserInput) (uuid.UUID, error) {
	id := uuid.New()

	params := models.CreateUserParams{
		Id:        id,
		Username:  input.Username,
		Role:      models.UserRole(input.Role),
		KratosId:  input.KratosId,
		Email:     input.Email,
		Name:      input.Name,
		Surname:   input.Surname,
		Bio:       input.Bio,
		AvatarUrl: input.AvatarUrl,
	}

	// Create user directly (no image table anymore)
	err := u.usersRepo.CreateUser(ctx, params)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (u *UsersUseCase) GetUserById(ctx context.Context, id uuid.UUID) (models.User, error) {
	return u.usersRepo.GetUserById(ctx, id)
}

func (u *UsersUseCase) GetUserByKratosId(ctx context.Context, kratosId uuid.UUID) (models.User, error) {
	return u.usersRepo.GetUserByKratosId(ctx, kratosId)
}

func (u *UsersUseCase) ListUsers(ctx context.Context, filter models.UsersListFilter) (models.UsersList, error) {
	params := models.UsersListFilter{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Search:   filter.Search,
		Role:     filter.Role,
	}

	usersList, err := u.usersRepo.ListUsers(ctx, params)
	if err != nil {
		return models.UsersList{}, err
	}

	return usersList, nil
}

func (u *UsersUseCase) UpdateUser(ctx context.Context, input models.UpdateUserInput) error {
	var role *models.UserRole
	if input.Role != nil {
		r := models.UserRole(*input.Role)
		role = &r
	}

	params := models.UpdateUserParams{
		Id:        input.Id,
		Username:  input.Username,
		Role:      role,
		Email:     input.Email,
		Name:      input.Name,
		Surname:   input.Surname,
		Bio:       input.Bio,
		AvatarUrl: input.AvatarUrl,
	}

	return u.usersRepo.UpdateUser(ctx, params)
}
