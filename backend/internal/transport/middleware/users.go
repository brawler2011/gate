package middleware

import (
	"errors"
	"fmt"
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
)

func UsersMiddleware(usersUC UsersUC, onError corev1.ErrorHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

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
			if errors.Is(err, pkg.ErrNotFound) {
				next.ServeHTTP(w, r)
				return
			}
			if err != nil {
				onError(ctx, w, r, fmt.Errorf("load session user %s: %w", session.UserID, err))
				return
			}

			ctx = accessmodel.WithUser(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
