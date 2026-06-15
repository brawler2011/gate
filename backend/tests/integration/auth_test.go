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
}
