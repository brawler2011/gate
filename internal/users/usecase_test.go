package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/users"
	"github.com/gate149/core/internal/users/mock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUsersUseCase_CreateUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()
	input := &models.UserCreation{
		Username: "john",
		Role:     models.RoleUser,
	}
	outID := uuid.New()

	repo.EXPECT().
		CreateUser(ctx, input).
		Return(outID, nil)

	id, err := uc.CreateUser(ctx, input)
	require.NoError(t, err)
	require.Equal(t, outID, id)
}

func TestUsersUseCase_CreateUser_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()
	input := &models.UserCreation{
		Username: "john",
		Role:     models.RoleUser,
	}

	repo.EXPECT().
		CreateUser(ctx, input).
		Return(uuid.Nil, errors.New("db error"))

	_, err := uc.CreateUser(ctx, input)
	require.Error(t, err)
}

func TestUsersUseCase_GetUserById(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()
	id := uuid.New()

	user := &models.User{
		Id:        id,
		Username:  "john",
		Role:      models.RoleUser,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	repo.EXPECT().
		GetUserById(ctx, id).
		Return(user, nil)

	res, err := uc.GetUserById(ctx, id)
	require.NoError(t, err)
	require.Equal(t, user, res)
}

func TestUsersUseCase_GetUserById_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()
	id := uuid.New()

	repo.EXPECT().
		GetUserById(ctx, id).
		Return(nil, errors.New("not found"))

	res, err := uc.GetUserById(ctx, id)
	require.Nil(t, res)
	require.Error(t, err)
}

func TestUsersUseCase_GetUserByKratosId(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()
	kratosID := "kratos-123"

	user := &models.User{
		Id:        uuid.New(),
		Username:  "alice",
		KratosId:  &kratosID,
		Role:      models.RoleUser,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	repo.EXPECT().
		GetUserByKratosId(ctx, kratosID).
		Return(user, nil)

	res, err := uc.GetUserByKratosId(ctx, kratosID)
	require.NoError(t, err)
	require.Equal(t, user, res)
}

func TestUsersUseCase_GetUserByKratosId_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()
	kratosID := "kratos-123"

	repo.EXPECT().
		GetUserByKratosId(ctx, kratosID).
		Return(nil, errors.New("not found"))

	res, err := uc.GetUserByKratosId(ctx, kratosID)
	require.Error(t, err)
	require.Nil(t, res)
}

func TestUsersUseCase_SearchUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()

	search := &models.UsersSearch{
		Page:     1,
		PageSize: 20,
		Search:   "bob",
		Role:     models.RoleUser,
	}

	result := &models.UsersList{
		Users: []*models.User{
			{
				Id:        uuid.New(),
				Username:  "bob",
				Role:      models.RoleUser,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Pagination: models.Pagination{
			Page:  1,
			Total: 1,
		},
	}

	repo.EXPECT().
		SearchUsers(ctx, search).
		Return(result, nil)

	res, err := uc.SearchUsers(ctx, search)
	require.NoError(t, err)
	require.Equal(t, result, res)
}

func TestUsersUseCase_SearchUsers_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := usersmock.NewMockRepo(ctrl)
	uc := users.NewUseCase(repo)

	ctx := context.Background()
	search := &models.UsersSearch{}

	repo.EXPECT().
		SearchUsers(ctx, search).
		Return(nil, errors.New("db error"))

	res, err := uc.SearchUsers(ctx, search)
	require.Error(t, err)
	require.Nil(t, res)
}
