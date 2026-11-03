package middleware

import (
	"log/slog"
	"net/http"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
)

// UsersMiddleware loads user from session into context
func UsersMiddleware(usersUC UsersUC) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// If user is already set (e.g. by AuthMiddleware or test middleware), proceed
			if user := accessmodel.UserFromContext(ctx); !user.IsGuest() {
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

			ctx = accessmodel.WithUser(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
