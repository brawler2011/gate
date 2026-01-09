package usecase

import (
	"context"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UsersUseCase struct {
	usersRepo  interfaces.UsersRepo
	outboxRepo interfaces.OutboxRepo
	imagesRepo interfaces.ImagesRepo
	txManager  interfaces.Transactor
}

func NewUsersUseCase(
	repo interfaces.UsersRepo,
	outboxRepo interfaces.OutboxRepo,
	imagesRepo interfaces.ImagesRepo,
	txManager interfaces.Transactor,
) *UsersUseCase {
	return &UsersUseCase{
		usersRepo:  repo,
		outboxRepo: outboxRepo,
		imagesRepo: imagesRepo,
		txManager:  txManager,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, input models.CreateUserInput) (uuid.UUID, error) {
	id := uuid.New()

	// Prepare image Id if image data is provided
	var imgId *uuid.UUID
	var imageParams *models.CreateImageParams

	if input.Image != nil && *input.Image != "" {
		imageId := uuid.New()
		imgId = &imageId
		imageParams = &models.CreateImageParams{
			Id:    imageId,
			Image: *input.Image,
		}
	}

	params := models.CreateUserParams{
		Id:       id,
		Username: input.Username,
		Role:     models.UserRole(input.Role),
		KratosId: input.KratosId,
		Email:    input.Email,
		Name:     input.Name,
		Surname:  input.Surname,
		Bio:      input.Bio,
		ImgId:    imgId,
	}

	// Use transaction to save both user and image atomically
	err := u.txManager.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Save image first if provided
		if imageParams != nil {
			if err := u.imagesRepo.WithTx(tx).CreateImage(ctx, *imageParams); err != nil {
				return err
			}
		}

		// Save user with reference to image
		if err := u.usersRepo.WithTx(tx).CreateUser(ctx, params); err != nil {
			return err
		}

		return nil
	})

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
		Id:       input.Id,
		Username: input.Username,
		Role:     role,
		Email:    input.Email,
		Name:     input.Name,
		Surname:  input.Surname,
		Bio:      input.Bio,
		ImgId:    input.ImgId,
	}

	return u.usersRepo.UpdateUser(ctx, params)
}
