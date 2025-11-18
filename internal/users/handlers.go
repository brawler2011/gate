package users

import (
	"context"
	"unicode/utf8"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UsersUC interface {
	CreateUser(ctx context.Context, user *models.UserCreation) (uuid.UUID, error)
	GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error)
	SearchUsers(ctx context.Context, search *models.UsersSearch) (*models.UsersList, error)
}

type UsersHandlers struct {
	usersUC UsersUC
}

func NewHandlers(usersUC UsersUC) *UsersHandlers {
	return &UsersHandlers{
		usersUC: usersUC,
	}
}

func (h *UsersHandlers) GetUser(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.Context()

	user, err := h.usersUC.GetUserById(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(testerv1.GetUserResponseModel{
		User: userDTO(*user),
	})
}

func CheckLength(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

func UserOrAdmin(s string) bool {
	return s == models.RoleUser || s == models.RoleAdmin
}

func ValidateGetUsersParams(params testerv1.GetUsersParams) (*models.UsersSearch, error) {
	search := &models.UsersSearch{
		Page:     params.Page,
		PageSize: params.PageSize,
		Search:   "",
		Role:     models.RoleUser,
	}

	if params.PageSize <= 0 || params.PageSize > 100 {
		return nil, pkg.Wrap(pkg.ErrBadInput,
			nil,
			"",
			"page_size parameter must be between 1 and 100")
	}

	if params.Page < 0 {
		return nil, pkg.Wrap(pkg.ErrBadInput,
			nil,
			"",
			"page parameter must be >= 0")
	}

	if params.Search != nil {
		if !CheckLength(*params.Search, 0, 50) {
			return nil, pkg.Wrap(pkg.ErrBadInput,
				nil,
				"",
				"search parameter length must be between 0 and 50 characters")
		}
		search.Search = *params.Search
	}

	if params.Role != nil {
		if !UserOrAdmin(*params.Role) {
			return nil, pkg.Wrap(pkg.ErrBadInput,
				nil,
				"",
				"role parameter must be either 'user' or 'admin'")
		}
		search.Role = *params.Role
	}

	return search, nil
}

func (h *UsersHandlers) GetUsers(c *fiber.Ctx, params testerv1.GetUsersParams) error {
	ctx := c.Context()

	search, err := ValidateGetUsersParams(params)
	if err != nil {
		return err
	}

	users, err := h.usersUC.SearchUsers(ctx, search)
	if err != nil {
		return err
	}

	return c.JSON(usersListDTO(users))
}

func userDTO(u models.User) testerv1.UserModel {
	return testerv1.UserModel{
		Id:        u.Id,
		Username:  u.Username,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func usersListDTO(ul *models.UsersList) testerv1.ListUsersResponseModel {
	userDTOs := make([]testerv1.UserModel, len(ul.Users))
	for i, user := range ul.Users {
		userDTOs[i] = userDTO(*user)
	}

	return testerv1.ListUsersResponseModel{
		Users: userDTOs,
		Pagination: testerv1.PaginationModel{
			Page:  ul.Pagination.Page,
			Total: ul.Pagination.Total,
		},
	}
}
