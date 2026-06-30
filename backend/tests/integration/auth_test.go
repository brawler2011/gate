//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (s *IntegrationTestSuite) TestAuth() {
	// 1. Register a new user
	s.Run("Register", func() {
		username := "testauth_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "password123"

		resp, err := s.client.RegisterWithResponse(s.ctx, corev1.RegisterJSONRequestBody{
			Username: username,
			Email:    types.Email(email),
			Password: password,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
		s.Require().NotNil(resp.JSON200)
		s.Equal(username, resp.JSON200.User.Username)
		s.Require().NotNil(resp.JSON200.User.Email)
		s.Equal(email, *resp.JSON200.User.Email)
		s.NotEmpty(resp.JSON200.SessionId)

		// Verify cookie is set on the response
		cookies := resp.HTTPResponse.Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "session_id" {
				sessionCookie = c
				break
			}
		}
		s.Require().NotNil(sessionCookie)
		s.Equal(resp.JSON200.SessionId.String(), sessionCookie.Value)
		s.True(sessionCookie.HttpOnly)

		// Test Duplicate Registration (should fail)
		respDup, err := s.client.RegisterWithResponse(s.ctx, corev1.RegisterJSONRequestBody{
			Username: username,
			Email:    types.Email(email),
			Password: password,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, respDup.StatusCode())

		// Test short password validation (min 8 characters)
		respShort, err := s.client.RegisterWithResponse(s.ctx, corev1.RegisterJSONRequestBody{
			Username: "shortpassuser",
			Email:    types.Email("short@example.com"),
			Password: "short",
		})
		s.Require().NoError(err)
		s.Equal(http.StatusBadRequest, respShort.StatusCode())
	})

	// 2. Login
	s.Run("Login", func() {
		username := "loginuser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "securepassword"

		// First register the user
		regResp, err := s.client.RegisterWithResponse(s.ctx, corev1.RegisterJSONRequestBody{
			Username: username,
			Email:    types.Email(email),
			Password: password,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, regResp.StatusCode())

		// Test successful Login via Username
		loginResp, err := s.client.LoginWithResponse(s.ctx, corev1.LoginJSONRequestBody{
			Identifier: username,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, loginResp.StatusCode())
		s.Equal(username, loginResp.JSON200.User.Username)
		s.NotEmpty(loginResp.JSON200.SessionId)

		// Test successful Login via Email
		loginRespEmail, err := s.client.LoginWithResponse(s.ctx, corev1.LoginJSONRequestBody{
			Identifier: email,
			Password:   password,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, loginRespEmail.StatusCode())

		// Test Login with invalid password
		loginRespFail, err := s.client.LoginWithResponse(s.ctx, corev1.LoginJSONRequestBody{
			Identifier: username,
			Password:   "wrongpassword",
		})
		s.Require().NoError(err)
		s.Equal(http.StatusUnauthorized, loginRespFail.StatusCode())

		// Test Login with non-existent user
		loginRespMissing, err := s.client.LoginWithResponse(s.ctx, corev1.LoginJSONRequestBody{
			Identifier: "non_existent_user",
			Password:   password,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusUnauthorized, loginRespMissing.StatusCode())
	})

	// 3. Logout
	s.Run("Logout", func() {
		username := "logoutuser_" + uuid.NewString()[:8]
		email := username + "@example.com"
		password := "anotherpassword"

		// Register and login to get a session
		regResp, err := s.client.RegisterWithResponse(s.ctx, corev1.RegisterJSONRequestBody{
			Username: username,
			Email:    types.Email(email),
			Password: password,
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, regResp.StatusCode())
		sessionID := regResp.JSON200.SessionId

		// Logout
		logoutResp, err := s.client.LogoutWithResponse(s.ctx, func(ctx context.Context, req *http.Request) error {
			// Attach session cookie
			req.AddCookie(&http.Cookie{
				Name:  "session_id",
				Value: sessionID.String(),
			})
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, logoutResp.StatusCode())

		// Verify cookie is cleared on response (MaxAge < 0 or empty value)
		cookies := logoutResp.HTTPResponse.Cookies()
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

		regResp, err := s.client.RegisterWithResponse(s.ctx, corev1.RegisterJSONRequestBody{
			Username: username,
			Email:    types.Email(email),
			Password: password,
		})
		s.Require().NoError(err)
		sessionID := regResp.JSON200.SessionId

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

		regResp, err := s.client.RegisterWithResponse(s.ctx, corev1.RegisterJSONRequestBody{
			Username: username,
			Email:    types.Email(email),
			Password: password,
		})
		s.Require().NoError(err)
		sessionID := regResp.JSON200.SessionId
		userID := regResp.JSON200.User.Id

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

