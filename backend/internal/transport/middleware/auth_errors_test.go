package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type authFunc func(context.Context, uuid.UUID) (accessmodel.User, error)

func (f authFunc) Authenticate(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	return f(ctx, id)
}

func TestAuthFailureKinds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		reached bool
	}{
		{"expired session continues unauthenticated", pkg.Wrap(pkg.ErrUnauthenticated, nil, "expired"), http.StatusNoContent, true},
		{"session store outage fails the request", errors.New("connection refused"), http.StatusInternalServerError, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			auth := authFunc(func(context.Context, uuid.UUID) (accessmodel.User, error) { return accessmodel.User{}, tc.err })
			h := AuthMiddleware(auth, ResponseErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil))))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				assert.ErrorIs(t, AuthenticationError(r.Context()), pkg.ErrUnauthenticated)
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: uuid.NewString()})
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			require.Equal(t, tc.status, rr.Code)
			require.Equal(t, tc.reached, reached)
		})
	}
}
