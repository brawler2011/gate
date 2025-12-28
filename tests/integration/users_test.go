package integration

import (
	"context"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestUsers() {
	// 1. Create Users
	user1 := s.createUser("user1", models.UserRoleUser)
	user2 := s.createUser("user2", models.UserRoleAdmin)

	// 2. GetMe
	s.Run("GetMe", func() {
		resp, err := s.client.GetMeWithResponse(s.ctx, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", user1.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
		s.Equal(user1.Id, resp.JSON200.User.Id)
		s.Equal(user1.Username, resp.JSON200.User.Username)
	})

	// 3. GetUser
	s.Run("GetUser", func() {
		resp, err := s.client.GetUserWithResponse(s.ctx, user2.Id, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", user1.Id.String())
			return nil
		})
		s.Require().NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode())
		s.Equal(user2.Id, resp.JSON200.User.Id)
		s.Equal(user2.Username, resp.JSON200.User.Username)
	})

	// 4. ListUsers
	s.Run("ListUsers", func() {
		resp, err := s.client.ListUsersWithResponse(s.ctx, &corev1.ListUsersParams{
			Page:     1,
			PageSize: 10,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", user1.Id.String())
			return nil
		})
		s.Require().NoError(err)
		if resp.StatusCode() != http.StatusOK {
			s.T().Logf("ListUsers failed: %s", string(resp.Body))
		}
		s.Equal(http.StatusOK, resp.StatusCode())
		s.GreaterOrEqual(len(resp.JSON200.Users), 2)
	})
}

func (s *IntegrationTestSuite) createUser(username string, role models.UserRole) models.User {
	user := models.User{
		Id:       uuid.New(),
		Username: username,
		Role:     role,
		KratosID: uuid.New(),
		Email:    username + "@example.com",
	}
	err := s.usersRepo.CreateUser(s.ctx, models.CreateUserParams{
		Id:       user.Id,
		Username: user.Username,
		Role:     user.Role,
		KratosId: user.KratosID,
		Email:    user.Email,
	})
	s.Require().NoError(err)
	return user
}
