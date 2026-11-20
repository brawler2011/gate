package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type contextKey string

const sessionKey contextKey = "session"
const authenticationErrorKey contextKey = "authentication_error"

const sessionCookieName = "session_id"

// onError answers infrastructure failures; an invalid session only leaves the request unauthenticated.
func AuthMiddleware(authUC AuthUC, onError corev1.ErrorHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Capabilities and other authenticated projections must never enter shared caches.
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Add("Vary", "Cookie")
			ctx := accessmodel.WithReadTime(r.Context(), time.Now().UTC())
			r = r.WithContext(ctx)

			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			sessionID, err := uuid.Parse(cookie.Value)
			if err != nil {
				slog.WarnContext(ctx, "malformed session cookie")
				r = r.WithContext(context.WithValue(ctx, authenticationErrorKey, pkg.ErrUnauthenticated))
				next.ServeHTTP(w, r)
				return
			}

			user, err := authUC.Authenticate(ctx, sessionID)
			if err != nil {
				if !errors.Is(err, pkg.ErrUnauthenticated) {
					onError(ctx, w, r, fmt.Errorf("authenticate session: %w", err))
					return
				}
				r = r.WithContext(context.WithValue(ctx, authenticationErrorKey, err))
				next.ServeHTTP(w, r)
				return
			}

			session := identitymodel.Session{
				ID:     sessionID,
				UserID: user.Id,
			}

			ctx = context.WithValue(ctx, sessionKey, session)
			ctx = accessmodel.WithUser(ctx, user)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSession(ctx context.Context) (identitymodel.Session, error) {
	session, ok := ctx.Value(sessionKey).(identitymodel.Session)
	if !ok {
		return identitymodel.Session{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "no session in context")
	}
	return session, nil
}

type SecurityHandler struct{}

func NewSecurityHandler() *SecurityHandler {
	return &SecurityHandler{}
}

func (s *SecurityHandler) HandleCookieAuth(ctx context.Context, operationName corev1.OperationName, t corev1.CookieAuth) (context.Context, error) {
	return ctx, nil
}

// Public observers must not downgrade invalid or expired sessions to guests.
func AuthenticationError(ctx context.Context) error {
	err, _ := ctx.Value(authenticationErrorKey).(error)
	return err
}
