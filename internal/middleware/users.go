package middleware

import (
	"context"
	"log/slog"

	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UsersUC interface {
	GetUserByKratosId(ctx context.Context, kratosId string) (domain.User, error)
}

type UsersMiddleware struct {
	usersUC UsersUC
}

func NewUsersMiddleware(usersUC UsersUC) *UsersMiddleware {
	return &UsersMiddleware{
		usersUC: usersUC,
	}
}

const userKey = "user"

// AuthMiddleware loads user from session into context
func (m *UsersMiddleware) AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()

		session, err := GetSession(ctx)
		if err != nil {
			// Session not found or error, just continue without user
			// Or should we fail? Usually if this middleware is applied, we want to try load user.
			// But maybe user is not logged in.
			return c.Next()
		}

		if session == nil {
			return c.Next()
		}

		user, err := m.usersUC.GetUserByKratosId(ctx, session.Identity.Id)
		if err != nil {
			slog.Error("failed to get user by kratos id", "error", err, "kratos_id", session.Identity.Id)
			// Don't fail request, just no user in context
			return c.Next()
		}

		c.SetUserContext(context.WithValue(ctx, userKey, user))

		return c.Next()
	}
}

// GetUser extracts User from context
func GetUser(ctx context.Context) (domain.User, error) {
	u, ok := ctx.Value(userKey).(domain.User)
	if !ok || u.ID == uuid.Nil {
		return domain.User{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "no user in context")
	}
	return u, nil
}

// GetUserOrAnonymous extracts User from context or returns an anonymous user with empty UUID
func GetUserOrAnonymous(ctx context.Context) domain.User {
	u, err := GetUser(ctx)
	if err != nil {
		// Return anonymous user with empty UUID
		return domain.User{
			ID:       uuid.Nil,
			Username: "",
			Role:     models.RoleUser, // Anonymous is treated as normal user role but with no ID? Or should have no role?
			// Usually anonymous has restricted access.
		}
	}
	return u
}
