package users_test

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/users"
	"github.com/gate149/core/internal/users/mock"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := usersmock.NewMockUsersUC(ctrl)
	h := users.NewHandlers(mock)

	app := fiber.New()

	id := uuid.New()

	user := &models.User{
		Id:        id,
		Username:  "john",
		Role:      models.RoleUser,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mock.EXPECT().
		GetUserById(gomock.Any(), id).
		Return(user, nil)

	app.Get("/users/:id", func(c *fiber.Ctx) error {
		return h.GetUser(c, id)
	})

	req := httptest.NewRequest("GET", "/users/"+id.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var body corev1.GetUserResponseModel
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Equal(t, user.Id, body.User.Id)
	require.Equal(t, user.Username, body.User.Username)
}

func TestGetUser_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := usersmock.NewMockUsersUC(ctrl)
	h := users.NewHandlers(mock)

	app := fiber.New()

	id := uuid.New()

	mock.EXPECT().
		GetUserById(gomock.Any(), id).
		Return(nil, errors.New("not found"))

	app.Get("/users/:id", func(c *fiber.Ctx) error {
		return h.GetUser(c, id)
	})

	req := httptest.NewRequest("GET", "/users/"+id.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, 500, resp.StatusCode)
}

func TestValidateGetUsersParams_Success(t *testing.T) {
	search := "John"
	role := models.RoleAdmin

	params := corev1.GetUsersParams{
		Page:     0,
		PageSize: 10,
		Search:   &search,
		Role:     &role,
	}

	res, err := users.ValidateGetUsersParams(params)
	require.NoError(t, err)

	require.Equal(t, search, res.Search)
	require.Equal(t, role, res.Role)
	require.Equal(t, int64(10), res.PageSize)
}

func TestValidateGetUsersParams_InvalidPageSize(t *testing.T) {
	params := corev1.GetUsersParams{PageSize: 0}

	_, err := users.ValidateGetUsersParams(params)
	require.Error(t, err)
}

func TestValidateGetUsersParams_InvalidRole(t *testing.T) {
	invalidRole := "superadmin"

	params := corev1.GetUsersParams{
		PageSize: 10,
		Role:     &invalidRole,
	}

	_, err := users.ValidateGetUsersParams(params)
	require.Error(t, err)
}

func TestGetUsers_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := usersmock.NewMockUsersUC(ctrl)
	h := users.NewHandlers(mock)
	app := fiber.New()
	
	searchStr := "john"
	params := corev1.GetUsersParams{
		Page:     0,
		PageSize: 20,
		Search:   &searchStr,
	}

	expectedSearch := &models.UsersSearch{
		Page:     0,
		PageSize: 20,
		Search:   "john",
		Role:     models.RoleUser,
	}

	user := &models.User{
		Id:        uuid.New(),
		Username:  "john",
		Role:      models.RoleUser,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	userList := &models.UsersList{
		Users: []*models.User{user},
		Pagination: models.Pagination{
			Page:  0,
			Total: 1,
		},
	}

	mock.
		EXPECT().
		SearchUsers(gomock.Any(), expectedSearch).
		Return(userList, nil)

	app.Get("/users", func(c *fiber.Ctx) error {
		return h.GetUsers(c, params)
	})

	req := httptest.NewRequest("GET", "/users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var body corev1.ListUsersResponseModel
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	require.Len(t, body.Users, 1)
	require.Equal(t, user.Username, body.Users[0].Username)
}

func TestGetUsers_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := usersmock.NewMockUsersUC(ctrl)
	h := users.NewHandlers(mock)
	app := fiber.New()

	params := corev1.GetUsersParams{
		PageSize: -1,
	}

	app.Get("/users", func(c *fiber.Ctx) error {
		return h.GetUsers(c, params)
	})

	req := httptest.NewRequest("GET", "/users", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, 500, resp.StatusCode)
}

func TestCheckLength(t *testing.T) {
	require.True(t, users.CheckLength("abc", 1, 3))
	require.False(t, users.CheckLength("abcd", 1, 3))
}

func TestUserOrAdmin(t *testing.T) {
	require.True(t, users.UserOrAdmin("user"))
	require.True(t, users.UserOrAdmin("admin"))
	require.False(t, users.UserOrAdmin("superadmin"))
}
