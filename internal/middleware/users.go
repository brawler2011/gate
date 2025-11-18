package middleware

import (
	"context"
	"log/slog"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
)

type UsersUC interface {
	GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error)
}

const (
	userKey = "user"
)

func UsersMiddleware(usersUC UsersUC) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var ctx = c.Context()

		session, err := GetSession(ctx)
		if err != nil {
			slog.Debug("no session found in context, skipping users middleware")
			return c.Next()
		}

		user, err := usersUC.GetUserByKratosId(ctx, session.Identity.Id)
		if err != nil {
			slog.Error("failed to get user by kratos id", "error", err)
			return c.Next()
		}

		c.Locals(userKey, user)

		return c.Next()
	}
}

// GetUser extracts User from context
func GetUser(ctx context.Context) (*models.User, error) {
	u, ok := ctx.Value(userKey).(*models.User)
	if !ok {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "", "no user in ctx")
	}
	return u, nil
}
