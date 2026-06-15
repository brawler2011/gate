package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

type contextKey string

const sessionKey contextKey = "session"
const sessionCookieName = "session_id"

// AuthMiddleware checks for a valid session cookie and retrieves the session
func AuthMiddleware(authUC interfaces.AuthUC) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				slog.Debug("no session cookie found, skipping auth middleware")
				next.ServeHTTP(w, r)
				return
			}

			sessionID, err := uuid.Parse(cookie.Value)
			if err != nil {
				slog.Error("session cookie found, but failed to parse UUID", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			user, err := authUC.Authenticate(ctx, sessionID)
			if err != nil {
				slog.Debug("failed to authenticate session", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			session := models.Session{
				ID:     sessionID,
				UserID: user.Id,
			}

			ctx = context.WithValue(ctx, sessionKey, session)
			ctx = context.WithValue(ctx, userKey, user)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSession(ctx context.Context) (models.Session, error) {
	session, ok := ctx.Value(sessionKey).(models.Session)
	if !ok {
		return models.Session{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "no session in context")
	}
	return session, nil
}
