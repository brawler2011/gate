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
		Name:     username,
		Surname:  "Test",
	}
	err := s.usersRepo.CreateUser(s.ctx, models.CreateUserParams{
		Id:       user.Id,
		Username: user.Username,
		Role:     user.Role,
		KratosId: user.KratosID,
		Email:    user.Email,
		Name:     user.Name,
		Surname:  user.Surname,
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
	// Git commit hash must be 40 characters (SHA-1)
	gitCommitHash := "0000000000000000000000000000000000000000"
	// Package hash must be 64 characters (SHA-256)
	packageHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := s.dbPool.Exec(s.ctx,
		"INSERT INTO problem_packages (id, problem_id, organization_id, git_commit_hash, package_hash, status) VALUES ($1, $2, $3, $4, $5, $6)",
		packageID, problemID, orgID, gitCommitHash, packageHash, "ready")
	s.Require().NoError(err)
	return packageID
}
