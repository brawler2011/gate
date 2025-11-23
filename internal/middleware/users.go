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
	userKey contextKey = "user"
)

func UsersMiddleware(usersUC UsersUC) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()

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

		ctx = context.WithValue(ctx, userKey, user)
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// GetUser extracts User from context
func GetUser(ctx context.Context) (*models.User, error) {
	u, ok := ctx.Value(userKey).(*models.User)
	if !ok {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "no user in context")
	}
	return u, nil
}

// GetUserOrAnonymous extracts User from context or returns an anonymous user with empty UUID
// This allows unauthenticated access to public resources
func GetUserOrAnonymous(ctx context.Context) *models.User {
	u, err := GetUser(ctx)
	if err != nil {
		// Return anonymous user with empty UUID
		return &models.User{
			Id:       [16]byte{}, // Empty UUID for anonymous users
			Username: "",
			Role:     models.RoleUser,
		}
	}
	return u
}