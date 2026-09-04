package middleware

import (
	"context"
	"log/slog"
	"net/http"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
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

const responseWriterKey contextKey = "response_writer"

// ResponseWriterMiddleware injects the ResponseWriter into the request context
func ResponseWriterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), responseWriterKey, w)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func SetSessionCookie(ctx context.Context, sessionID uuid.UUID) {
	if w, ok := ctx.Value(responseWriterKey).(http.ResponseWriter); ok && w != nil {
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID.String(),
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			MaxAge:   7 * 24 * 60 * 60,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func ClearSessionCookie(ctx context.Context) {
	if w, ok := ctx.Value(responseWriterKey).(http.ResponseWriter); ok && w != nil {
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			MaxAge:   -1,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

type SecurityHandler struct{}

func NewSecurityHandler() *SecurityHandler {
	return &SecurityHandler{}
}

func (s *SecurityHandler) HandleCookieAuth(ctx context.Context, operationName corev1.OperationName, t corev1.CookieAuth) (context.Context, error) {
	return ctx, nil
}



