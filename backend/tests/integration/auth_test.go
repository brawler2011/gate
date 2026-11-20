//go:build integration
// +build integration

package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	identity "github.com/brawler2011/gate/backend/internal/identity"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestAuth() {
	s.Run("Register", func() {
		username := "testauth_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "password123"

		resp, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(username, resp.User.Username)
		s.Require().True(resp.User.Email.IsSet())
		s.Equal(email, resp.User.Email.Value)

		// Re-registration of an unverified account replaces its verification token and email.
		respDup, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: "newpassword123",
		})
		s.Require().NoError(err)
		s.Require().NotNil(respDup)

		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", resp.User.ID)
		s.Require().NoError(err)

		// Verified accounts reject duplicate registration.
		_, err = s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().Error(err)
		s.Equal(http.StatusConflict, s.getStatusCode(err))

		// Registration rejects passwords below the minimum length.
		_, err = s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: "shortpassuser",
			Email:    "short@example.com",
			Password: "short",
		})
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	})

	s.Run("Login", func() {
		username := "loginuser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "securepassword"

		regResp, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().NoError(err)

		// Unverified user login fails with its own code once the password matches.
		_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().Error(err)
		s.Equal(http.StatusForbidden, s.getStatusCode(err))
		var problem *corev1.ProblemStatusCodeWithHeaders
		s.Require().ErrorAs(err, &problem)
		s.Equal(corev1.ErrorCode("email_not_verified"), problem.Response.Code)
		_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   "wrong-" + password,
		})
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err), "a wrong password must not reveal the unverified state")
		s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))

		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", regResp.User.ID)
		s.Require().NoError(err)

		loginResp, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Equal(username, loginResp.Response.User.Username)
		s.Require().True(loginResp.Response.SessionID.IsSet())
		s.NotEmpty(loginResp.Response.SessionID.Value)

		s.Equal(s.transport.lastResponse.Header.Get("Set-Cookie"), loginResp.SetCookie)
		s.requireSessionCookie(loginResp.Response.SessionID.Value.String(), 7*24*60*60)

		loginRespEmail, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: email,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Require().NotNil(loginRespEmail)

		_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   "wrongpassword",
		})
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
		s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))

		_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: "non_existent_user",
			Password:   password,
		})
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
		s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))
	})

	s.Run("Logout", func() {
		username := "logoutuser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "anotherpassword"

		regResp, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", regResp.User.ID)
		s.Require().NoError(err)

		loginResp, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Require().True(loginResp.Response.SessionID.IsSet())
		sessionID := loginResp.Response.SessionID.Value

		logoutResp, err := s.client.Logout(withTestCookie(s.ctx, sessionID))
		s.Require().NoError(err)
		s.Equal(s.transport.lastResponse.Header.Get("Set-Cookie"), logoutResp.SetCookie)
		s.requireSessionCookie("", -1)

		_, err = s.client.GetMe(withTestCookie(s.ctx, sessionID))
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))

		// An unauthenticated logout must still clear the browser cookie.
		_, err = s.client.Logout(s.ctx)
		s.Require().NoError(err)
		s.requireSessionCookie("", -1)
	})

	s.Run("Sliding Session authentication", func() {
		username := "slidinguser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "slidingpass"

		regResp, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", regResp.User.ID)
		s.Require().NoError(err)

		loginResp, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Require().True(loginResp.Response.SessionID.IsSet())
		sessionID := loginResp.Response.SessionID.Value

		var initialExpiry time.Time
		err = s.dbPool.QueryRow(s.ctx, "SELECT expires_at FROM sessions WHERE id = $1", sessionID).Scan(&initialExpiry)
		s.Require().NoError(err)

		time.Sleep(100 * time.Millisecond)

		authRepo := pgstore.NewAuthRepo(s.dbPool)
		txManager := pgstore.NewIdentityTransactions(s.dbPool)
		authUC := identity.NewAuthUseCase(s.usersRepo, authRepo, txManager, nil, nil)

		user, err := authUC.Authenticate(s.ctx, sessionID)
		s.Require().NoError(err)
		s.Equal(username, user.Username)

		var updatedExpiry time.Time
		err = s.dbPool.QueryRow(s.ctx, "SELECT expires_at FROM sessions WHERE id = $1", sessionID).Scan(&updatedExpiry)
		s.Require().NoError(err)

		// Authentication slides the session expiry.
		s.True(updatedExpiry.After(initialExpiry))
	})

	s.Run("Session lifetimes and cleanup", func() {
		username := "lifetimesuser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "lifetimespass"

		regResp, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", regResp.User.ID)
		s.Require().NoError(err)

		loginResp, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Require().True(loginResp.Response.SessionID.IsSet())
		sessionID := loginResp.Response.SessionID.Value
		userID := regResp.User.ID

		authRepo := pgstore.NewAuthRepo(s.dbPool)
		txManager := pgstore.NewIdentityTransactions(s.dbPool)
		authUC := identity.NewAuthUseCase(s.usersRepo, authRepo, txManager, nil, nil)

		// An idle session expires before its hard lifetime ends.
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET expires_at = NOW() - INTERVAL '5 minutes' WHERE id = $1", sessionID)
		s.Require().NoError(err)

		_, err = authUC.Authenticate(s.ctx, sessionID)
		s.Require().Error(err)
		s.Contains(err.Error(), "session expired (soft limit)")

		var count int
		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", sessionID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		newSessionID := uuid.New()
		err = authRepo.CreateSession(s.ctx, newSessionID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)

		// The hard lifetime expires even an active session.
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET created_at = NOW() - INTERVAL '8 days' WHERE id = $1", newSessionID)
		s.Require().NoError(err)

		_, err = authUC.Authenticate(s.ctx, newSessionID)
		s.Require().Error(err)
		s.Contains(err.Error(), "session expired (hard limit)")

		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", newSessionID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		capSessionID := uuid.New()
		err = authRepo.CreateSession(s.ctx, capSessionID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)

		hardExpiryIn := 10 * time.Minute
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET created_at = NOW() - INTERVAL '7 days' + $1::interval WHERE id = $2", hardExpiryIn.String(), capSessionID)
		s.Require().NoError(err)

		_, err = authUC.Authenticate(s.ctx, capSessionID)
		s.Require().NoError(err)

		var cappedExpiry time.Time
		err = s.dbPool.QueryRow(s.ctx, "SELECT expires_at FROM sessions WHERE id = $1", capSessionID).Scan(&cappedExpiry)
		s.Require().NoError(err)

		// Sliding expiry cannot exceed the session hard lifetime.
		expectedCapped := time.Now().Add(hardExpiryIn)
		s.WithinDuration(expectedCapped, cappedExpiry, 5*time.Second)

		softExpiredID := uuid.New()
		err = authRepo.CreateSession(s.ctx, softExpiredID, userID, time.Now().Add(-5*time.Minute))
		s.Require().NoError(err)

		hardExpiredID := uuid.New()
		err = authRepo.CreateSession(s.ctx, hardExpiredID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET created_at = NOW() - INTERVAL '8 days' WHERE id = $1", hardExpiredID)
		s.Require().NoError(err)

		validSessionID := uuid.New()
		err = authRepo.CreateSession(s.ctx, validSessionID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)

		err = authUC.CleanupExpiredSessions(s.ctx)
		s.Require().NoError(err)

		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", softExpiredID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", hardExpiredID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", validSessionID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(1, count)
	})
}

func (s *IntegrationTestSuite) requireSessionCookie(value string, maxAge int) {
	s.T().Helper()
	s.Require().Len(s.transport.lastResponse.Header.Values("Set-Cookie"), 1)
	cookies := s.transport.lastResponse.Cookies()
	s.Require().Len(cookies, 1)
	cookie := cookies[0]
	s.Equal("session_id", cookie.Name)
	s.Equal(value, cookie.Value)
	s.Equal("/", cookie.Path)
	s.True(cookie.HttpOnly)
	s.False(cookie.Secure)
	s.Equal(maxAge, cookie.MaxAge)
	s.Equal(http.SameSiteLaxMode, cookie.SameSite)
	s.Empty(cookie.Domain)
	s.True(cookie.Expires.IsZero())
}

func (s *IntegrationTestSuite) TestSessionCookieLifecycle() {
	username := "cookie_" + uuid.NewString()[:8]
	registered, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
		Username: username, Email: username + "@example.com", Password: "Password123!",
	})
	s.Require().NoError(err)
	s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))

	repo := pgstore.NewAuthRepo(s.dbPool)
	createToken := func(tokenType identitymodel.AuthTokenType) string {
		token := uuid.NewString()
		digest := sha256.Sum256([]byte(token))
		s.Require().NoError(repo.CreateAuthToken(s.ctx, identitymodel.CreateAuthTokenParams{
			ID: uuid.New(), UserID: registered.User.ID, TokenType: tokenType,
			TokenHash: hex.EncodeToString(digest[:]), ExpiresAt: time.Now().Add(time.Hour),
		}))
		return token
	}

	token := createToken(identitymodel.AuthTokenTypeEmailVerification)
	verified, err := s.client.VerifyEmail(s.ctx, &corev1.VerifyEmailRequestModel{Token: token})
	s.Require().NoError(err)
	s.Equal(registered.User.ID, verified.Response.User.ID)
	s.Require().True(verified.Response.SessionID.IsSet())
	s.Require().True(verified.SetCookie.IsSet())
	s.Equal(s.transport.lastResponse.Header.Get("Set-Cookie"), verified.SetCookie.Value)
	oldSessionID := verified.Response.SessionID.Value
	s.requireSessionCookie(oldSessionID.String(), 7*24*60*60)

	_, err = s.client.VerifyEmail(s.ctx, &corev1.VerifyEmailRequestModel{Token: token})
	s.Require().Error(err)
	s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))

	_, err = s.client.ChangePassword(withTestCookie(s.ctx, oldSessionID), &corev1.ChangePasswordRequestModel{
		OldPassword: "wrongpassword", NewPassword: "Password456!",
	})
	s.Require().Error(err)
	s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))

	changed, err := s.client.ChangePassword(withTestCookie(s.ctx, oldSessionID), &corev1.ChangePasswordRequestModel{
		OldPassword: "Password123!", NewPassword: "Password456!",
	})
	s.Require().NoError(err)
	s.Equal(registered.User.ID, changed.Response.User.ID)
	s.Require().True(changed.Response.SessionID.IsSet())
	newSessionID := changed.Response.SessionID.Value
	s.NotEqual(oldSessionID, newSessionID)
	s.Equal(s.transport.lastResponse.Header.Get("Set-Cookie"), changed.SetCookie)
	s.requireSessionCookie(newSessionID.String(), 7*24*60*60)

	_, err = s.client.GetMe(withTestCookie(s.ctx, oldSessionID))
	s.Require().Error(err)
	s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
	_, err = s.client.GetMe(withTestCookie(s.ctx, newSessionID))
	s.Require().NoError(err)

	err = s.client.ResetPassword(s.ctx, &corev1.ResetPasswordRequestModel{
		Token: createToken(identitymodel.AuthTokenTypePasswordReset), Password: "Password789!",
	})
	s.Require().NoError(err)
	s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))
	_, err = s.client.GetMe(withTestCookie(s.ctx, newSessionID))
	s.Require().Error(err)
	s.Equal(http.StatusUnauthorized, s.getStatusCode(err))

	err = s.client.ResetPassword(s.ctx, &corev1.ResetPasswordRequestModel{Token: "invalid", Password: "Password789!"})
	s.Require().Error(err)
	s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	s.Empty(s.transport.lastResponse.Header.Values("Set-Cookie"))
}
