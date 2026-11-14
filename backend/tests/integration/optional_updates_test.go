//go:build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/google/uuid"
)

// Raw JSON exercises the generated decoder as well as the use case and SQL.
func (s *IntegrationTestSuite) TestUserUpdatePresence() {
	for _, tc := range []struct {
		name, body, email string
		status            int
	}{
		{"omitted", `{}`, "", http.StatusOK},
		{"value", `{"email":"updated@example.com"}`, "updated@example.com", http.StatusOK},
		{"null_email", `{"email":null}`, "", http.StatusBadRequest},
		{"null_username", `{"username":null}`, "", http.StatusBadRequest},
		{"null_role", `{"role":null}`, "", http.StatusBadRequest},
		{"empty_email", `{"email":""}`, "", http.StatusBadRequest},
		{"empty_username", `{"username":""}`, "", http.StatusBadRequest},
		{"invalid_mixed", `{"username":"renamed_user","email":""}`, "", http.StatusBadRequest},
	} {
		s.Run(tc.name, func() {
			user := s.createUser("optional_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
			before, err := s.usersRepo.GetUserById(s.ctx, user.Id)
			s.Require().NoError(err)
			req := httptest.NewRequest(http.MethodPatch, "/users/"+user.Username, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User-ID", user.Id.String())
			rec := httptest.NewRecorder()
			s.handler.ServeHTTP(rec, req)
			s.Require().Equal(tc.status, rec.Code, rec.Body.String())
			got, err := s.usersRepo.GetUserById(s.ctx, user.Id)
			s.Require().NoError(err)
			expectedEmail := before.Email
			if tc.email != "" {
				expectedEmail = &tc.email
			}
			s.Equal(expectedEmail, got.Email)
			s.Equal(before.Username, got.Username)
			s.Equal(before.Role, got.Role)
			s.Equal(before.AvatarUrl, got.AvatarUrl)
			s.Equal(before.ExpiresAt, got.ExpiresAt)
			s.Equal(before.IsEmailVerified, got.IsEmailVerified)
			if tc.status != http.StatusOK {
				s.Equal(before, got, "rejected updates must not write")
			}
		})
	}
}

// These internal fields are not exposed by PATCH /users/{username}.
func (s *IntegrationTestSuite) TestUserRepositoryOptionalValues() {
	user := s.createUser("repo_optional_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	avatar := "avatar-key"
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	verified := true
	s.Require().NoError(s.usersRepo.UpdateUser(s.ctx, identitymodel.UpdateUserParams{
		Id: user.Id, AvatarUrl: &avatar, ExpiresAt: &expires, IsEmailVerified: &verified,
	}))
	// nil means preserve, including nullable columns; it cannot express clearing.
	s.Require().NoError(s.usersRepo.UpdateUser(s.ctx, identitymodel.UpdateUserParams{Id: user.Id}))
	got, err := s.usersRepo.GetUserById(s.ctx, user.Id)
	s.Require().NoError(err)
	s.Equal(&avatar, got.AvatarUrl)
	s.Require().NotNil(got.ExpiresAt)
	s.True(expires.Equal(*got.ExpiresAt))
	s.True(got.IsEmailVerified)
	s.Equal(user.Email, got.Email)

	empty := ""
	verified = false
	s.Require().NoError(s.usersRepo.UpdateUser(s.ctx, identitymodel.UpdateUserParams{
		Id: user.Id, AvatarUrl: &empty, IsEmailVerified: &verified,
	}))
	got, err = s.usersRepo.GetUserById(s.ctx, user.Id)
	s.Require().NoError(err)
	s.Equal(&empty, got.AvatarUrl, "avatar deletion uses an empty string sentinel, not SQL NULL")
	s.False(got.IsEmailVerified, "false must not be confused with omission")
	s.Require().NotNil(got.ExpiresAt)
	s.True(expires.Equal(*got.ExpiresAt))
	s.Equal(user.Email, got.Email)
}
