//go:build integration
// +build integration

package integration

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestAuth() {
	// 1. Register a new user
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

		// Test Unverified Re-Registration (should succeed and update token/email)
		respDup, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: "newpassword123",
		})
		s.Require().NoError(err)
		s.Require().NotNil(respDup)

		// Mark email as verified
		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", resp.User.ID)
		s.Require().NoError(err)

		// Test Duplicate Registration after verification (should fail with conflict)
		_, err = s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().Error(err)
		s.Equal(http.StatusConflict, s.getStatusCode(err))

		// Test short password validation (min 8 characters)
		_, err = s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: "shortpassuser",
			Email:    "short@example.com",
			Password: "short",
		})
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	})

	// 2. Login
	s.Run("Login", func() {
		username := "loginuser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "securepassword"

		// First register the user
		regResp, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().NoError(err)

		// Unverified user login should fail with Bad Request
		_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))

		// Mark email as verified
		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", regResp.User.ID)
		s.Require().NoError(err)

		// Test successful Login via Username
		loginResp, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Equal(username, loginResp.User.Username)
		s.Require().True(loginResp.SessionID.IsSet())
		s.NotEmpty(loginResp.SessionID.Value)

		// Verify cookie is set on the response
		cookies := s.transport.lastResponse.Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "session_id" {
				sessionCookie = c
				break
			}
		}
		s.Require().NotNil(sessionCookie)
		s.Equal(loginResp.SessionID.Value.String(), sessionCookie.Value)
		s.True(sessionCookie.HttpOnly)

		// Test successful Login via Email
		loginRespEmail, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: email,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Require().NotNil(loginRespEmail)

		// Test Login with invalid password
		_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   "wrongpassword",
		})
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))

		// Test Login with non-existent user
		_, err = s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: "non_existent_user",
			Password:   password,
		})
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
	})

	// 3. Logout
	s.Run("Logout", func() {
		username := "logoutuser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "anotherpassword"

		// Register and verify email
		regResp, err := s.client.Register(s.ctx, &corev1.RegisterRequestModel{
			Username: username,
			Email:    email,
			Password: password,
		})
		s.Require().NoError(err)

		_, err = s.dbPool.Exec(s.ctx, "UPDATE users SET is_email_verified = TRUE WHERE id = $1", regResp.User.ID)
		s.Require().NoError(err)

		// Login to get a session
		loginResp, err := s.client.Login(s.ctx, &corev1.LoginRequestModel{
			Identifier: username,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Require().True(loginResp.SessionID.IsSet())
		sessionID := loginResp.SessionID.Value

		// Logout
		err = s.client.Logout(withTestCookie(s.ctx, sessionID))
		s.Require().NoError(err)

		// Verify cookie is cleared on response (MaxAge < 0 or empty value)
		cookies := s.transport.lastResponse.Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "session_id" {
				sessionCookie = c
				break
			}
		}
		s.Require().NotNil(sessionCookie)
		s.True(sessionCookie.MaxAge < 0 || sessionCookie.Value == "")
	})

	// 4. Sliding Session authentication test
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
		s.Require().True(loginResp.SessionID.IsSet())
		sessionID := loginResp.SessionID.Value

		// Directly query DB to get the initial expires_at
		var initialExpiry time.Time
		err = s.dbPool.QueryRow(s.ctx, "SELECT expires_at FROM sessions WHERE id = $1", sessionID).Scan(&initialExpiry)
		s.Require().NoError(err)

		// Wait slightly to ensure time difference
		time.Sleep(100 * time.Millisecond)

		// Perform a request utilizing the session context to trigger sliding expiry update.
		authRepo := pg.NewAuthRepo(s.dbPool)
		txManager := pg.NewTransactor(s.dbPool)
		authUC := usecase.NewAuthUseCase(s.usersRepo, authRepo, txManager)

		// Call Authenticate
		user, err := authUC.Authenticate(s.ctx, sessionID)
		s.Require().NoError(err)
		s.Equal(username, user.Username)

		// Fetch the updated expires_at
		var updatedExpiry time.Time
		err = s.dbPool.QueryRow(s.ctx, "SELECT expires_at FROM sessions WHERE id = $1", sessionID).Scan(&updatedExpiry)
		s.Require().NoError(err)

		// Expiry must be later than initial expiry
		s.True(updatedExpiry.After(initialExpiry))
	})

	// 5. Dual session lifetimes and cleanup test
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
		s.Require().True(loginResp.SessionID.IsSet())
		sessionID := loginResp.SessionID.Value
		userID := regResp.User.ID

		authRepo := pg.NewAuthRepo(s.dbPool)
		txManager := pg.NewTransactor(s.dbPool)
		authUC := usecase.NewAuthUseCase(s.usersRepo, authRepo, txManager)

		// Test Case A: Soft timeout (expires_at in past)
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET expires_at = NOW() - INTERVAL '5 minutes' WHERE id = $1", sessionID)
		s.Require().NoError(err)

		_, err = authUC.Authenticate(s.ctx, sessionID)
		s.Require().Error(err)
		s.Contains(err.Error(), "session expired (soft limit)")

		// Verify session is deleted
		var count int
		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", sessionID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		// Recreate session for Hard timeout test
		newSessionID := uuid.New()
		err = authRepo.CreateSession(s.ctx, newSessionID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)

		// Test Case B: Hard timeout (created_at older than 7 days)
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET created_at = NOW() - INTERVAL '8 days' WHERE id = $1", newSessionID)
		s.Require().NoError(err)

		_, err = authUC.Authenticate(s.ctx, newSessionID)
		s.Require().Error(err)
		s.Contains(err.Error(), "session expired (hard limit)")

		// Verify session is deleted
		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", newSessionID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		// Recreate session for Capping test
		capSessionID := uuid.New()
		err = authRepo.CreateSession(s.ctx, capSessionID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)

		// Set created_at to almost 7 days ago (6 days, 23 hours, 50 minutes ago)
		// So hard limit is in 10 minutes.
		hardExpiryIn := 10 * time.Minute
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET created_at = NOW() - INTERVAL '7 days' + $1::interval WHERE id = $2", hardExpiryIn.String(), capSessionID)
		s.Require().NoError(err)

		// Call Authenticate, which should slide but cap at hardExpiry
		_, err = authUC.Authenticate(s.ctx, capSessionID)
		s.Require().NoError(err)

		var cappedExpiry time.Time
		err = s.dbPool.QueryRow(s.ctx, "SELECT expires_at FROM sessions WHERE id = $1", capSessionID).Scan(&cappedExpiry)
		s.Require().NoError(err)

		// The expires_at should be roughly Now() + 10 minutes (hardExpiry), not Now() + 40 minutes (softExpiry)
		expectedCapped := time.Now().Add(hardExpiryIn)
		s.WithinDuration(expectedCapped, cappedExpiry, 5*time.Second)

		// Test Case C: Background Cleanup query
		// Create an expired session (soft)
		softExpiredID := uuid.New()
		err = authRepo.CreateSession(s.ctx, softExpiredID, userID, time.Now().Add(-5*time.Minute))
		s.Require().NoError(err)

		// Create a hard-expired session
		hardExpiredID := uuid.New()
		err = authRepo.CreateSession(s.ctx, hardExpiredID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, "UPDATE sessions SET created_at = NOW() - INTERVAL '8 days' WHERE id = $1", hardExpiredID)
		s.Require().NoError(err)

		// Create a valid session
		validSessionID := uuid.New()
		err = authRepo.CreateSession(s.ctx, validSessionID, userID, time.Now().Add(40*time.Minute))
		s.Require().NoError(err)

		// Run Cleanup
		err = authUC.CleanupExpiredSessions(s.ctx)
		s.Require().NoError(err)

		// Verify softExpired is deleted
		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", softExpiredID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		// Verify hardExpired is deleted
		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", hardExpiredID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(0, count)

		// Verify valid session remains
		err = s.dbPool.QueryRow(s.ctx, "SELECT COUNT(*) FROM sessions WHERE id = $1", validSessionID).Scan(&count)
		s.Require().NoError(err)
		s.Equal(1, count)
	})
}

