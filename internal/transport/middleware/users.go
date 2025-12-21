package middleware

import (
	"context"
	"log/slog"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const userKey = "user"

// UsersMiddleware loads user from session into context
func UsersMiddleware(usersUC interfaces.UsersUC) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()

		session, err := GetSession(ctx)
		if err != nil {
			slog.Error("failed to get session", "error", err)
			return c.Next()
		}

		if session == nil {
			return c.Next()
		}

		kratosId, err := uuid.Parse(session.Identity.Id)
		if err != nil {
			slog.Error("failed to parse kratos id", "error", err, "kratos_id", session.Identity.Id)
			return c.Next()
		}

		user, err := usersUC.GetUserByKratosId(ctx, kratosId)
		if err != nil {
			slog.Error("failed to get user by kratos id", "error", err, "kratos_id", session.Identity.Id)
			return c.Next()
		}

		c.SetUserContext(context.WithValue(ctx, userKey, user))

		return c.Next()
	}
}

// GetUser extracts User from context
// Returns an anonymous user if no user is found in context
func GetUser(ctx context.Context) models.User {
	u, ok := ctx.Value(userKey).(models.User)
	if !ok {
		return models.Guest
	}
	return u
}
