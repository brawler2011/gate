package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
)

const userKey = "user"

// UsersMiddleware loads user from session into context
func UsersMiddleware(usersUC interfaces.UsersUC) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// If user is already set (e.g. by AuthMiddleware or test middleware), proceed
			if _, ok := ctx.Value(userKey).(models.User); ok {
				next.ServeHTTP(w, r)
				return
			}

			session, err := GetSession(ctx)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			user, err := usersUC.GetUserById(ctx, session.UserID)
			if err != nil {
				slog.Error("failed to get user by id from session", "error", err, "user_id", session.UserID)
				next.ServeHTTP(w, r)
				return
			}

			ctx = context.WithValue(ctx, userKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
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
