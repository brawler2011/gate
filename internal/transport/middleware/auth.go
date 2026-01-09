package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gate149/core/pkg"
	ory "github.com/ory/client-go"
)

type contextKey string

const sessionKey contextKey = "session"
const kratosSessionCookieName = "ory_kratos_session"

// AuthMiddleware checks for a valid Kratos session cookie and retrieves the session
func AuthMiddleware(frontendAPI ory.FrontendAPI) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			cookie, err := r.Cookie(kratosSessionCookieName)
			if err != nil || cookie.Value == "" {
				slog.Debug("no kratos session cookie found, skipping auth middleware")
				next.ServeHTTP(w, r)
				return
			}

			session, _, err := frontendAPI.
				ToSession(ctx).
				Cookie(fmt.Sprintf("%s=%s", kratosSessionCookieName, cookie.Value)).
				Execute()
			if err != nil {
				slog.Error("kratos session cookie found, but failed to get session", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if session == nil {
				slog.Error("kratos session cookie found, but session is nil")
				next.ServeHTTP(w, r)
				return
			}

			if !*session.Active {
				slog.Error("kratos session cookie found, but session is not active")
				next.ServeHTTP(w, r)
				return
			}

			ctx = context.WithValue(ctx, sessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func getSession(ctx context.Context) (*ory.Session, error) {
	session, ok := ctx.Value(sessionKey).(*ory.Session)
	if !ok {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "no session in context")
	}
	return session, nil
}
