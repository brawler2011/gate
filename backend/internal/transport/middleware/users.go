package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

const userKey = "user"

// UsersMiddleware loads user from session into context
func UsersMiddleware(usersUC interfaces.UsersUC) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			session, err := getSession(ctx)
			if err != nil {
				slog.Error("failed to get session", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if session == nil {
				next.ServeHTTP(w, r)
				return
			}

			kratosId, err := uuid.Parse(session.Identity.Id)
			if err != nil {
				slog.Error("failed to parse kratos id", "error", err, "kratos_id", session.Identity.Id)
				next.ServeHTTP(w, r)
				return
			}

			user, err := usersUC.GetUserByKratosId(ctx, kratosId)
			if err != nil {
				slog.Error("failed to get user by kratos id", "error", err, "kratos_id", session.Identity.Id)
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
