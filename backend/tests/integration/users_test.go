//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
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
		searchPrefix := "pagetest" + uuid.NewString()[:8]
		searchUser1 := s.createUser(searchPrefix+"alpha", models.UserRoleUser)
		searchUser2 := s.createUser(searchPrefix+"beta", models.UserRoleAdmin)
		search := searchPrefix

		resp, err := s.client.ListUsersWithResponse(s.ctx, &corev1.ListUsersParams{
			Page:     1,
			PageSize: 10,
			Search:   &search,
		}, func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-Test-User-ID", user1.Id.String())
			return nil
		})
		s.Require().NoError(err)
		if resp.StatusCode() != http.StatusOK {
			s.T().Logf("ListUsers failed: %s", string(resp.Body))
		}
		s.Equal(http.StatusOK, resp.StatusCode())
		s.Require().NotNil(resp.JSON200)

		hasSearchUser1 := false
		hasSearchUser2 := false
		for _, user := range resp.JSON200.Users {
			if user.Username == searchUser1.Username {
				hasSearchUser1 = true
			}
			if user.Username == searchUser2.Username {
				hasSearchUser2 = true
			}
		}

		s.True(hasSearchUser1)
		s.True(hasSearchUser2)
		s.GreaterOrEqual(len(resp.JSON200.Users), 2)
		s.Equal(int32(1), resp.JSON200.Pagination.Total)
	})
}

func (s *IntegrationTestSuite) createUser(username string, role models.UserRole) models.User {
	user := models.User{
		Id:           uuid.New(),
		Username:     username,
		Role:         role,
		PasswordHash: "$2a$10$8K1p/ae9QD.b69/j/8G5/eF/G0y.L4tG7c2G/u1w5u/c3t6T7y6m6", // dummy bcrypt hash
		Email:        username + "@example.com",
		Name:         username,
		Surname:      "Test",
	}
	err := s.usersRepo.CreateUser(s.ctx, models.CreateUserParams{
		Id:           user.Id,
		Username:     user.Username,
		Role:         user.Role,
		PasswordHash: user.PasswordHash,
		Email:        user.Email,
		Name:         user.Name,
		Surname:      user.Surname,
	})
	s.Require().NoError(err)
	return user
}

func (s *IntegrationTestSuite) createOrganization(login string, name string, ownerID uuid.UUID) *models.Organization {
	org, err := s.organizationsRepo.CreateOrganization(s.ctx, &models.CreateOrganizationInput{
		Login:     login,
		Name:      name,
		CreatorID: ownerID,
	})
	s.Require().NoError(err)

	return org
}

// createOrganizationWithID creates an organization with a specific ID (for test scenarios)
func (s *IntegrationTestSuite) createOrganizationWithID(id uuid.UUID, login string, name string) *models.Organization {
	// Directly insert using SQL to set a specific ID
	_, err := s.dbPool.Exec(s.ctx,
		"INSERT INTO organizations (id, login, name, description) VALUES ($1, $2, $3, $4)",
		id, login, name, "")
	s.Require().NoError(err)

	return &models.Organization{
		ID:    id,
		Login: login,
		Name:  name,
	}
}

// createDummyProblemPackage creates a dummy problem package for testing
func (s *IntegrationTestSuite) createDummyProblemPackage(problemID uuid.UUID, orgID uuid.UUID) uuid.UUID {
	packageID := uuid.New()
	// Package hash must be 64 characters (SHA-256)
	packageHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := s.dbPool.Exec(s.ctx,
		"INSERT INTO problem_packages (id, problem_id, organization_id, package_hash, status, version) VALUES ($1, $2, $3, $4, $5, $6)",
		packageID, problemID, orgID, packageHash, "ready", 1)
	s.Require().NoError(err)
	return packageID
}
