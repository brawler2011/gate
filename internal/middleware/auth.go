package middleware

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	ory "github.com/ory/client-go"
)

const (
	sessionKey              = "session"
	kratosSessionCookieName = "ory_kratos_session"
)

// AuthMiddleware checks for a valid Kratos session cookie and retrieves the session
func AuthMiddleware(frontendAPI ory.FrontendAPI) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var ctx = c.Context()

		cookie := c.Cookies(kratosSessionCookieName, "")
		if cookie == "" {
			slog.Debug("no kratos session cookie found, skipping auth middleware")
			return c.Next()
		}

		session, _, err := frontendAPI.
			ToSession(ctx).
			Cookie(fmt.Sprintf("%s=%s", kratosSessionCookieName, cookie)).
			Execute()
		if err != nil {
			slog.Error("kratos session cookie found, but failed to get session", "error", err)
			return c.Next()
		}

		c.Locals(sessionKey, session)

		return c.Next()
	}
}

// GetSession extracts Kratos session from context
func GetSession(ctx context.Context) (*ory.Session, error) {
	u, ok := ctx.Value(sessionKey).(*ory.Session)
	if !ok {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "", "no session in ctx")
	}
	return u, nil
}
