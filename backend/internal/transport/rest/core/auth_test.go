package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cookieIdentityStub struct {
	AuthUC
	user      accessmodel.User
	sessionID uuid.UUID
	err       error
	loggedOut uuid.UUID
}

func (s *cookieIdentityStub) Register(context.Context, string, string, string) (accessmodel.User, error) {
	return s.user, s.err
}

func (s *cookieIdentityStub) Login(context.Context, string, string) (accessmodel.User, uuid.UUID, error) {
	return s.user, s.sessionID, s.err
}

func (s *cookieIdentityStub) VerifyEmail(context.Context, string) (accessmodel.User, uuid.UUID, error) {
	return s.user, s.sessionID, s.err
}

func (s *cookieIdentityStub) ChangePassword(context.Context, uuid.UUID, string, string) (uuid.UUID, error) {
	return s.sessionID, s.err
}

func (s *cookieIdentityStub) Logout(_ context.Context, sessionID uuid.UUID) error {
	s.loggedOut = sessionID
	return s.err
}

func (s *cookieIdentityStub) ResetPassword(context.Context, string, string) error {
	return s.err
}

func (s *cookieIdentityStub) Authenticate(context.Context, uuid.UUID) (accessmodel.User, error) {
	return s.user, nil
}

func assertSessionCookie(t *testing.T, header, value string, maxAge int, secure bool) {
	t.Helper()
	cookie, err := http.ParseSetCookie(header)
	require.NoError(t, err)
	assert.Equal(t, "session_id", cookie.Name)
	assert.Equal(t, value, cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, secure, cookie.Secure)
	assert.Equal(t, maxAge, cookie.MaxAge)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Empty(t, cookie.Domain)
	assert.True(t, cookie.Expires.IsZero())
}

func authenticatedCookieContext(identity *cookieIdentityStub, sessionID uuid.UUID) context.Context {
	var ctx context.Context
	handler := middleware.AuthMiddleware(identity)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID.String()})
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return ctx
}

func TestAuthHandlerCookies(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "secure"}[secure], func(t *testing.T) {
			user := accessmodel.User{Id: uuid.New(), Username: "cookieuser", Role: accessmodel.UserRoleUser}
			identity := &cookieIdentityStub{user: user, sessionID: uuid.New()}
			h := NewCoreServer(Services{Auth: identity}, secure)
			ctx := context.Background()
			oldSessionID := uuid.New()
			authCtx := authenticatedCookieContext(identity, oldSessionID)

			login, err := h.Login(ctx, &corev1.LoginRequestModel{Identifier: user.Username, Password: "password123"})
			require.NoError(t, err)
			assert.Equal(t, userDTO(user), login.Response.User)
			assert.Equal(t, corev1.NewOptUUID(identity.sessionID), login.Response.SessionID)
			assertSessionCookie(t, login.SetCookie, identity.sessionID.String(), 7*24*60*60, secure)

			changed, err := h.ChangePassword(authCtx, &corev1.ChangePasswordRequestModel{OldPassword: "password123", NewPassword: "password456"})
			require.NoError(t, err)
			assert.Equal(t, userDTO(user), changed.Response.User)
			assert.Equal(t, corev1.NewOptUUID(identity.sessionID), changed.Response.SessionID)
			assert.NotEqual(t, oldSessionID, changed.Response.SessionID.Value)
			assertSessionCookie(t, changed.SetCookie, identity.sessionID.String(), 7*24*60*60, secure)

			verified, err := h.VerifyEmail(ctx, &corev1.VerifyEmailRequestModel{Token: "token"})
			require.NoError(t, err)
			require.True(t, verified.SetCookie.IsSet())
			assert.Equal(t, userDTO(user), verified.Response.User)
			assert.Equal(t, corev1.NewOptUUID(identity.sessionID), verified.Response.SessionID)
			assertSessionCookie(t, verified.SetCookie.Value, identity.sessionID.String(), 7*24*60*60, secure)

			identity.sessionID = uuid.Nil
			verified, err = h.VerifyEmail(ctx, &corev1.VerifyEmailRequestModel{Token: "token"})
			require.NoError(t, err)
			assert.Equal(t, userDTO(user), verified.Response.User)
			assert.False(t, verified.SetCookie.IsSet())
			assert.False(t, verified.Response.SessionID.IsSet())

			logout, err := h.Logout(ctx)
			require.NoError(t, err)
			assert.Equal(t, uuid.Nil, identity.loggedOut)
			assertSessionCookie(t, logout.SetCookie, "", -1, secure)

			identity.err = nil
			logout, err = h.Logout(authCtx)
			require.NoError(t, err)
			assert.Equal(t, oldSessionID, identity.loggedOut)
			assertSessionCookie(t, logout.SetCookie, "", -1, secure)

			logoutFailure := errors.New("logout failed")
			identity.err = logoutFailure
			logout, err = h.Logout(authCtx)
			require.ErrorIs(t, err, logoutFailure)
			assert.Nil(t, logout)
		})
	}
}

func TestAuthHandlerCookieErrors(t *testing.T) {
	identity := &cookieIdentityStub{err: pkg.ErrBadInput}
	h := NewCoreServer(Services{Auth: identity}, true)
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})

	login, err := h.Login(ctx, &corev1.LoginRequestModel{})
	require.ErrorIs(t, err, identity.err)
	assert.Nil(t, login)
	verified, err := h.VerifyEmail(ctx, &corev1.VerifyEmailRequestModel{})
	require.ErrorIs(t, err, identity.err)
	assert.Nil(t, verified)
	changed, err := h.ChangePassword(ctx, &corev1.ChangePasswordRequestModel{})
	require.ErrorIs(t, err, identity.err)
	assert.Nil(t, changed)
	changed, err = h.ChangePassword(context.Background(), &corev1.ChangePasswordRequestModel{})
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	assert.Nil(t, changed)
	changed, err = h.ChangePassword(ctx, nil)
	require.ErrorIs(t, err, pkg.ErrBadInput)
	assert.Nil(t, changed)
}

func TestAuthHTTPCookies(t *testing.T) {
	for _, secure := range []bool{false, true} {
		for _, transport := range []struct {
			name, scheme, forwarded string
		}{
			{"http", "http", ""},
			{"tls", "https", ""},
			{"proxy_https", "http", "https"},
			{"proxy_http", "https", "http"},
		} {
			for _, operation := range []struct {
				name, path, body string
				cookie           bool
				clear            bool
				noSession        bool
				failure          bool
			}{
				{name: "login", path: "/auth/login", body: `{"identifier":"cookieuser","password":"password123"}`, cookie: true},
				{name: "change", path: "/users/me/change-password", body: `{"old_password":"password123","new_password":"password456"}`, cookie: true},
				{name: "verify", path: "/auth/verify-email", body: `{"token":"token"}`, cookie: true},
				{name: "verify_without_session", path: "/auth/verify-email", body: `{"token":"token"}`, noSession: true},
				{name: "logout", path: "/auth/logout", cookie: true, clear: true},
				{name: "logout_without_auth", path: "/auth/logout", cookie: true, clear: true, noSession: true},
				{name: "reset", path: "/auth/reset-password", body: `{"token":"token","password":"password456"}`},
				{name: "register", path: "/auth/register", body: `{"username":"cookieuser","email":"cookie@example.com","password":"password123"}`},
				{name: "login_error", path: "/auth/login", body: `{"identifier":"cookieuser","password":"password123"}`, failure: true},
				{name: "change_error", path: "/users/me/change-password", body: `{"old_password":"password123","new_password":"password456"}`, failure: true},
				{name: "verify_error", path: "/auth/verify-email", body: `{"token":"token"}`, failure: true},
				{name: "reset_error", path: "/auth/reset-password", body: `{"token":"token","password":"password456"}`, failure: true},
			} {
				t.Run(map[bool]string{false: "local", true: "secure"}[secure]+"/"+transport.name+"/"+operation.name, func(t *testing.T) {
					identity := &cookieIdentityStub{user: accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}, sessionID: uuid.New()}
					if operation.noSession {
						identity.sessionID = uuid.Nil
					}
					if operation.failure {
						identity.err = pkg.ErrBadInput
					}
					h := NewCoreServer(Services{Auth: identity}, secure)
					server, err := corev1.NewServer(h, middleware.NewSecurityHandler(), corev1.WithErrorHandler(middleware.ResponseErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))))
					require.NoError(t, err)
					req := httptest.NewRequest(http.MethodPost, transport.scheme+"://example.com"+operation.path, strings.NewReader(operation.body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-Forwarded-Proto", transport.forwarded)
					req.Header.Set("Forwarded", "proto="+transport.forwarded)
					if !operation.noSession {
						req.AddCookie(&http.Cookie{Name: "session_id", Value: uuid.NewString()})
					}
					rr := httptest.NewRecorder()
					middleware.AuthMiddleware(identity)(server).ServeHTTP(rr, req)
					status := http.StatusOK
					if operation.failure {
						status = http.StatusBadRequest
					}
					require.Equal(t, status, rr.Code, rr.Body.String())
					if !operation.cookie {
						assert.Empty(t, rr.Header().Values("Set-Cookie"))
						return
					}
					require.Len(t, rr.Header().Values("Set-Cookie"), 1)
					value, maxAge := identity.sessionID.String(), 7*24*60*60
					if operation.clear {
						value, maxAge = "", -1
					}
					assertSessionCookie(t, rr.Header().Get("Set-Cookie"), value, maxAge, secure)
				})
			}
		}
	}
}
