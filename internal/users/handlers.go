package users

import (
	"context"
	"fmt"
	"unicode/utf8"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/middleware"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UsersUC interface {
	GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error)
	ListUsers(ctx context.Context, filter *models.UsersFilter) (*models.UsersList, error)
}

type SubmissionsUC interface {
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error)
}

type PermissionsUC interface {
	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action permissions.ContestAction, opts ...permissions.PermissionOption) (bool, error)
}

type UsersHandlers struct {
	usersUC       UsersUC
	submissionsUC SubmissionsUC
	permissionsUC PermissionsUC
}

func NewHandlers(usersUC UsersUC, submissionsUC SubmissionsUC, permissionsUC PermissionsUC) *UsersHandlers {
	return &UsersHandlers{
		usersUC:       usersUC,
		submissionsUC: submissionsUC,
		permissionsUC: permissionsUC,
	}
}

const (
	minPage      = 1
	minPageSize  = 1
	maxPageSize  = 20
	maxSearchLen = 50
)

func (h *UsersHandlers) GetUser(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user, err := h.usersUC.GetUserById(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(testerv1.GetUserResponseModel{
		User: userDTO(*user),
	})
}

func isLengthBetween(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

func isUserOrAdmin(s string) bool {
	return s == models.RoleUser || s == models.RoleAdmin
}

func badInput(msg string) error {
	return pkg.Wrap(pkg.ErrBadInput, nil, msg)
}

var (
	badPageSize = badInput(
		fmt.Sprintf("page_size parameter must be between %d and %d", minPageSize, maxPageSize),
	)
	badPage = badInput(
		fmt.Sprintf("page parameter must be >= %d", minPage),
	)
	badSearch = badInput(
		fmt.Sprintf("search parameter length must be between 0 and %d characters", maxSearchLen),
	)
	badRole = badInput(
		"role parameter must be either 'user' or 'admin'",
	)
)

func validateGetUsersParams(params testerv1.ListUsersParams) (*models.UsersFilter, error) {
	filter := &models.UsersFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
		Search:   "",
		Role:     "",
	}

	if params.PageSize < minPageSize || params.PageSize > maxPageSize {
		return nil, badPageSize
	}

	if params.Page < minPage {
		return nil, badPage
	}

	if params.Search != nil {
		if !isLengthBetween(*params.Search, 0, maxSearchLen) {
			return nil, badSearch
		}
		filter.Search = *params.Search
	}

	if params.Role != nil {
		if !isUserOrAdmin(*params.Role) {
			return nil, badRole
		}
		filter.Role = *params.Role
	}

	return filter, nil
}

func (h *UsersHandlers) ListUsers(c *fiber.Ctx, params testerv1.ListUsersParams) error {
	ctx := c.UserContext()

	filter, err := validateGetUsersParams(params)
	if err != nil {
		return err
	}

	users, err := h.usersUC.ListUsers(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(usersListDTO(users))
}

func (h *UsersHandlers) ListUserSubmissions(c *fiber.Ctx, userId uuid.UUID, params testerv1.ListUserSubmissionsParams) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	// Check permissions based on whether viewing own or other's submissions
	if userId != user.Id {
		// Trying to view someone else's submissions - only admin can do this
		if !user.IsAdmin() {
			return pkg.Wrap(pkg.NoPermission, nil, "only admins can view other users' submissions")
		}
	} else if params.ContestId != nil {
		// Viewing own submissions with contestId specified - check contest permission
		canView, err := h.permissionsUC.HasContestPermission(ctx, *params.ContestId, user.Id, permissions.ActionListOwnSubmissions, permissions.WithUser(user))
		if err != nil {
			return err
		}
		if !canView {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
	}
	// If userId == user.Id and no contestId - allow (user viewing own submissions globally)

	filter := listUserSubmissionsParamsToFilter(userId, params)

	submissions, err := h.submissionsUC.ListSolutions(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(submissionsListToDTO(submissions))
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

func listUserSubmissionsParamsToFilter(userId uuid.UUID, params testerv1.ListUserSubmissionsParams) models.SolutionsFilter {
	var state *models.State = nil
	if params.State != nil {
		s := models.State(*params.State)
		state = &s
	}

	// Convert sortOrder string to integer: -1 for desc, 0 for asc
	var order *int64 = nil
	if params.SortOrder != nil {
		var orderVal int64
		if *params.SortOrder == testerv1.Desc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	return models.SolutionsFilter{
		ContestId: params.ContestId,
		Page:      params.Page,
		PageSize:  params.PageSize,
		ProblemId: params.ProblemId,
		UserId:    &userId,
		Language:  nil,
		Order:     order,
		State:     state,
	}
}

func submissionsListToDTO(solutionsList *models.SolutionsList) *testerv1.ListSubmissionsResponseModel {
	resp := testerv1.ListSubmissionsResponseModel{
		Submissions: make([]testerv1.SubmissionsListItemModel, len(solutionsList.Solutions)),
		Pagination: testerv1.PaginationModel{
			Page:  solutionsList.Pagination.Page,
			Total: solutionsList.Pagination.Total,
		},
	}

	for i, solution := range solutionsList.Solutions {
		resp.Submissions[i] = testerv1.SubmissionsListItemModel{
			Id:           solution.Id,
			UserId:       solution.CreatedBy,
			Username:     solution.Username,
			State:        int64(solution.State),
			Score:        solution.Score,
			Penalty:      solution.Penalty,
			TimeStat:     solution.TimeStat,
			MemoryStat:   solution.MemoryStat,
			Language:     int64(solution.Language),
			ProblemId:    solution.ProblemId,
			ProblemTitle: solution.ProblemTitle,
			Position:     solution.Position,
			ContestId:    solution.ContestId,
			ContestTitle: solution.ContestTitle,
			UpdatedAt:    solution.UpdatedAt,
			CreatedAt:    solution.CreatedAt,
		}
	}

	return &resp
}
