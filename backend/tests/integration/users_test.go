//go:build integration
// +build integration

package integration

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/textproto"
	"time"

	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
)

func (s *IntegrationTestSuite) TestUserAvatar() {
	user := s.createUser("avataruser", accessmodel.UserRoleUser)

	testImg := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var imgBuf bytes.Buffer
	s.Require().NoError(png.Encode(&imgBuf, testImg))
	avatarContent := imgBuf.Bytes()

	file := ht.MultipartFile{
		Name:   "avatar.png",
		File:   bytes.NewReader(avatarContent),
		Size:   int64(len(avatarContent)),
		Header: textproto.MIMEHeader{"Content-Type": []string{"image/png"}},
	}

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

	s.Run("GetUserProfileWithImgID", func() {
		resp, err := s.client.GetUser(withTestUser(s.ctx, user.Id), corev1.GetUserParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Require().True(resp.User.ImgID.IsSet())
		s.Equal(imgID, resp.User.ImgID.Value)
	})

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
		s.True(bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")), "avatar must have valid PNG signature")
		s.Equal(avatarContent, data)
	})

	s.Run("GetAvatarImage304", func() {
		resp, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{
			Username:    user.Username,
			IfNoneMatch: corev1.NewOptString(etag),
		})
		s.Require().NoError(err)
		_, isNotModified := resp.(*corev1.GetUserAvatarNotModified)
		s.True(isNotModified)
	})

	s.Run("DeleteAvatar", func() {
		err := s.client.DeleteAvatar(withTestUser(s.ctx, user.Id), corev1.DeleteAvatarParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
	})

	s.Run("GetUserProfileWithImgIDNil", func() {
		resp, err := s.client.GetUser(withTestUser(s.ctx, user.Id), corev1.GetUserParams{
			Username: user.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.False(resp.User.ImgID.IsSet())
	})

	s.Run("GetAvatarImage404", func() {
		_, err := s.client.GetUserAvatar(s.ctx, corev1.GetUserAvatarParams{
			Username: user.Username,
		})
		s.Equal(http.StatusNotFound, s.getStatusCode(err))
	})
}

func (s *IntegrationTestSuite) TestUsers() {
	user1 := s.createUser("user1", accessmodel.UserRoleUser)
	user2 := s.createUser("user2", accessmodel.UserRoleAdmin)

	s.Run("GetMe", func() {
		resp, err := s.client.GetMe(withTestUser(s.ctx, user1.Id))
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(user1.Id, resp.User.ID)
		s.Equal(user1.Username, resp.User.Username)
	})

	s.Run("GetUser", func() {
		resp, err := s.client.GetUser(withTestUser(s.ctx, user1.Id), corev1.GetUserParams{
			Username: user2.Username,
		})
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Equal(user2.Id, resp.User.ID)
		s.Equal(user2.Username, resp.User.Username)
	})

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
		s.Equal(int32(1), resp.Pagination.TotalPages)
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
	id := uuid.New()
	s.Require().NoError(dbfixtures.InsertOrganization(s.ctx, s.dbPool, id, login, name, ownerID))
	org, err := s.organizationsRepo.GetOrganizationByID(s.ctx, id)
	s.Require().NoError(err)
	return org
}

func (s *IntegrationTestSuite) createContest(p contestsmodel.CreateContestParams, owner uuid.UUID) error {
	return dbfixtures.CreateContest(s.ctx, s.dbPool, p, owner)
}

func (s *IntegrationTestSuite) createDummyProblemPackage(problemID uuid.UUID, orgID uuid.UUID) uuid.UUID {
	packageID, err := dbfixtures.InsertPackage(s.ctx, s.dbPool, problemID, orgID, time.Time{})
	s.Require().NoError(err)
	return packageID
}
