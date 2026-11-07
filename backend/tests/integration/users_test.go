//go:build integration
// +build integration

package integration

import (
	"bytes"
	"io"
	"net/http"
	"net/textproto"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
)

func (s *IntegrationTestSuite) TestUserAvatar() {
	user := s.createUser("avataruser", accessmodel.UserRoleUser)

	avatarContent := []byte("fake image content")
	file := ht.MultipartFile{
		Name:   "avatar.png",
		File:   bytes.NewReader(avatarContent),
		Size:   int64(len(avatarContent)),
		Header: textproto.MIMEHeader{"Content-Type": []string{"image/png"}},
	}

	// 1. Upload Avatar
	var imgID uuid.UUID
	s.Run("UploadAvatar", func() {
		resp, err := s.client.UploadAvatar(withTestUser(s.ctx, user.Id), &corev1.UploadAvatarReq{
			Avatar: file,
		}, corev1.UploadAvatarParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		imgID = resp.ImgID
		s.NotEmpty(imgID.String())
	})

	// 2. Get User Profile and Check ImgID
	s.Run("GetUserProfileWithImgID", func() {
		resp, err := s.client.GetUser(withTestUser(s.ctx, user.Id), corev1.GetUserParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Require().True(resp.User.ImgID.IsSet())
		s.Equal(imgID, resp.User.ImgID.Value)
	})

	// 3. Get Avatar Image
	var etag string
	s.Run("GetAvatarImage", func() {
		resp, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		okResp, ok := resp.(*corev1.GetUserAvatarOKHeaders)
		s.Require().True(ok)
		s.True(okResp.ETag.IsSet())
		etag = okResp.ETag.Value
		s.NotEmpty(etag)

		data, err := io.ReadAll(okResp.Response.Data)
		s.Require().NoError(err)
		s.Equal(avatarContent, data)
	})

	// 4. Get Avatar Image with If-None-Match (304 Not Modified)
	s.Run("GetAvatarImage304", func() {
		resp, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{
			Username:    user.Username,
			IfNoneMatch: corev1.NewOptString(etag),
		})
		s.Require().NoError(err)
		_, isNotModified := resp.(*corev1.GetUserAvatarNotModified)
		s.True(isNotModified)
	})

	// 5. Delete Avatar
	s.Run("DeleteAvatar", func() {
		err := s.client.DeleteAvatar(withTestUser(s.ctx, user.Id), corev1.DeleteAvatarParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
	})

	// 6. Get User Profile and Check ImgID is nil
	s.Run("GetUserProfileWithImgIDNil", func() {
		resp, err := s.client.GetUser(withTestUser(s.ctx, user.Id), corev1.GetUserParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.False(resp.User.ImgID.IsSet())
	})

	// 7. Get Avatar Image (404 Not Found)
	s.Run("GetAvatarImage404", func() {
		resp, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
		_, isNotFound := resp.(*corev1.GetUserAvatarNotFound)
		s.True(isNotFound)
	})
}

func (s *IntegrationTestSuite) TestUsers() {
	// 1. Create Users
	user1 := s.createUser("user1", accessmodel.UserRoleUser)
	user2 := s.createUser("user2", accessmodel.UserRoleAdmin)

	// 2. GetMe
	s.Run("GetMe", func() {
		resp, err := s.client.GetMe(withTestUser(s.ctx, user1.Id))
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(user1.Id, resp.User.ID)
		s.Equal(user1.Username, resp.User.Username)
	})

	// 3. GetUser
	s.Run("GetUser", func() {
		resp, err := s.client.GetUser(withTestUser(s.ctx, user1.Id), corev1.GetUserParams{
			Username: user2.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(user2.Id, resp.User.ID)
		s.Equal(user2.Username, resp.User.Username)
	})

	// 4. ListUsers
	s.Run("ListUsers", func() {
		searchPrefix := "pagetest" + uuid.NewString()[:8]
		searchUser1 := s.createUser(searchPrefix+"alpha", accessmodel.UserRoleUser)
		searchUser2 := s.createUser(searchPrefix+"beta", accessmodel.UserRoleAdmin)
		search := searchPrefix

		resp, err := s.client.ListUsers(withTestUser(s.ctx, user1.Id), corev1.ListUsersParams{
			Page:     corev1.NewOptInt32(1),
			PageSize: corev1.NewOptInt32(10),
			Search:   corev1.NewOptString(search),
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)

		hasSearchUser1 := false
		hasSearchUser2 := false
		for _, user := range resp.Users {
			if user.Username == searchUser1.Username {
				hasSearchUser1 = true
			}
			if user.Username == searchUser2.Username {
				hasSearchUser2 = true
			}
		}

		s.True(hasSearchUser1)
		s.True(hasSearchUser2)
		s.GreaterOrEqual(len(resp.Users), 2)
		s.Equal(int32(1), resp.Pagination.Total)
	})
}

func (s *IntegrationTestSuite) TestUsersPaginationDefaultsAndBounds() {
	admin := s.createUser("pagination_admin_"+uuid.NewString()[:8], accessmodel.UserRoleAdmin)
	resp, err := s.client.ListUsers(withTestUser(s.ctx, admin.Id), corev1.ListUsersParams{})
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	for _, invalid := range []corev1.ListUsersParams{
		{Page: corev1.NewOptInt32(0)},
		{PageSize: corev1.NewOptInt32(0)},
		{PageSize: corev1.NewOptInt32(101)},
	} {
		_, err := s.client.ListUsers(withTestUser(s.ctx, admin.Id), invalid)
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	}

	valid, err := s.client.ListUsers(withTestUser(s.ctx, admin.Id), corev1.ListUsersParams{
		PageSize: corev1.NewOptInt32(100),
	})
	s.Require().NoError(err)
	s.Require().NotNil(valid)
}

func (s *IntegrationTestSuite) TestListMyClaimedAccounts() {
	owner := s.createUser("claim_owner_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	claimed := s.createUser("claimed_user_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	_, err := s.dbPool.Exec(s.ctx, "UPDATE users SET claimed_by_user_id=$1, claimed_at=now(), expires_at=now()+interval '1 day' WHERE id=$2", owner.Id, claimed.Id)
	s.Require().NoError(err)

	resp, err := s.client.ListMyClaimedAccounts(withTestUser(s.ctx, owner.Id))
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().Len(resp.Accounts, 1)
	s.Equal(claimed.Id, resp.Accounts[0].ID)
}

func (s *IntegrationTestSuite) createUser(username string, role accessmodel.UserRole) accessmodel.User {
	email := username + "@example.com"
	user := accessmodel.User{
		Id:           uuid.New(),
		Username:     username,
		Role:         role,
		PasswordHash: "$2a$10$8K1p/ae9QD.b69/j/8G5/eF/G0y.L4tG7c2G/u1w5u/c3t6T7y6m6", // dummy bcrypt hash
		Email:        &email,
	}
	err := s.usersRepo.CreateUser(s.ctx, identitymodel.CreateUserParams{
		Id:           user.Id,
		Username:     user.Username,
		Role:         user.Role,
		PasswordHash: user.PasswordHash,
		Email:        &email,
	})
	s.Require().NoError(err)
	return user
}

func (s *IntegrationTestSuite) createOrganization(login string, name string, ownerID uuid.UUID) *organizationsmodel.Organization {
	org, err := s.organizationsRepo.CreateOrganization(s.ctx, &organizationsmodel.CreateOrganizationInput{
		Login:     login,
		Name:      name,
		CreatorID: ownerID,
	})
	s.Require().NoError(err)

	err = s.organizationsRepo.AddMember(s.ctx, org.ID, ownerID, organizationsmodel.OrgRoleOwner)
	s.Require().NoError(err)

	return org
}

// createOrganizationWithID creates an organization with a specific ID (for test scenarios)
func (s *IntegrationTestSuite) createOrganizationWithID(id uuid.UUID, login string, name string) *organizationsmodel.Organization {
	// Directly insert using SQL to set a specific ID
	_, err := s.dbPool.Exec(s.ctx,
		"INSERT INTO organizations (id, login, name, description) VALUES ($1, $2, $3, $4)",
		id, login, name, "")
	s.Require().NoError(err)

	return &organizationsmodel.Organization{
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
