package handlers

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func (h *CoreServer) GetUser(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user, err := h.usersUC.GetUserById(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(corev1.GetUserResponseModel{
		User: userDTO(user),
	})
}

func (h *CoreServer) GetMe(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	return c.JSON(corev1.GetUserResponseModel{
		User: userDTO(user),
	})
}

func (h *CoreServer) ListUsers(c *fiber.Ctx, params corev1.ListUsersParams) error {
	ctx := c.UserContext()

	filter, err := validateGetUsersParams(params)
	if err != nil {
		return err
	}

	users, err := h.usersUC.ListUsers(ctx, *filter)
	if err != nil {
		return err
	}

	return c.JSON(usersListDTO(&users))
}

func (h *CoreServer) ListUserSubmissions(c *fiber.Ctx, userId uuid.UUID, params corev1.ListUserSubmissionsParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	if userId != user.Id {
		if !user.IsAdmin() {
			return pkg.Wrap(pkg.NoPermission, nil, "only admins can view other users' submissions")
		}
	} else if params.ContestId != nil {
		canView, err := h.permissionsUC.HasContestPermission(ctx, *params.ContestId, user.Id, models.ActionListOwnSubmissions)
		if err != nil {
			return err
		}
		if !canView {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
	}

	filter := listUserSubmissionsParamsToFilter(userId, params)

	submissions, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(submissionsListToDTO(submissions))
}
