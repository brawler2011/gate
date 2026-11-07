//go:build integration

package integration

import (
	"fmt"
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListPaginationAndPackageHistory() {
	owner := s.createUser("pagination_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("list-pagination", "Pagination", owner.Id)
	userIDs := []uuid.UUID{owner.Id}
	defer func() {
		_, err := s.dbPool.Exec(s.ctx, "DELETE FROM organizations WHERE id=$1", org.ID)
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, "DELETE FROM users WHERE id=ANY($1::uuid[])", userIDs)
		s.Require().NoError(err)
	}()
	ctx := withTestUser(s.ctx, owner.Id)
	teamID, problemID := uuid.New(), uuid.New()
	_, err := s.dbPool.Exec(s.ctx, `INSERT INTO teams(id, organization_id, name, slug) VALUES ($1,$2,'Pagination','pagination')`, teamID, org.ID)
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, `INSERT INTO problems(id, organization_id, owner_id, title, short_name, visibility) VALUES ($1,$2,$3,'Pagination','pagination','private')`, problemID, org.ID, owner.Id)
	s.Require().NoError(err)
	for i := range 3 {
		user := owner
		if i > 0 {
			user = s.createUser(fmt.Sprintf("pagination_member_%d", i), accessmodel.UserRoleUser)
			userIDs = append(userIDs, user.Id)
			s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, user.Id, organizationsmodel.OrgRoleMember))
		}
		_, err = s.dbPool.Exec(s.ctx, `INSERT INTO team_members(team_id,user_id,role,created_at) VALUES ($1,$2,'member','2026-01-01')`, teamID, user.Id)
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, `INSERT INTO problem_members(problem_id,user_id,role,created_at) VALUES ($1,$2,'viewer','2026-01-01')`, problemID, user.Id)
		s.Require().NoError(err)
	}
	type listPage struct {
		ids        []uuid.UUID
		pagination corev1.PaginationModel
	}
	lists := map[string]func(int32) listPage{
		"organization members": func(page int32) listPage {
			r, err := s.client.ListOrganizationMembers(ctx, corev1.ListOrganizationMembersParams{Login: org.Login, Page: corev1.NewOptInt32(page), PageSize: corev1.NewOptInt32(1)})
			s.Require().NoError(err)
			ids := make([]uuid.UUID, len(r.Members))
			for i, m := range r.Members {
				ids[i] = m.UserID
			}
			return listPage{ids, r.Pagination}
		},
		"team members": func(page int32) listPage {
			r, err := s.client.ListTeamMembers(ctx, corev1.ListTeamMembersParams{ID: teamID, Page: corev1.NewOptInt32(page), PageSize: corev1.NewOptInt32(1)})
			s.Require().NoError(err)
			ids := make([]uuid.UUID, len(r.Members))
			for i, m := range r.Members {
				ids[i] = m.UserID
			}
			return listPage{ids, r.Pagination}
		},
		"problem members": func(page int32) listPage {
			r, err := s.client.ListProblemMembers(ctx, corev1.ListProblemMembersParams{ID: problemID, Page: corev1.NewOptInt32(page), PageSize: corev1.NewOptInt32(1)})
			s.Require().NoError(err)
			ids := make([]uuid.UUID, len(r.Members))
			for i, m := range r.Members {
				ids[i] = m.UserID
			}
			return listPage{ids, r.Pagination}
		},
	}
	for name, read := range lists {
		s.Run(name, func() {
			seen := map[uuid.UUID]bool{}
			for page := int32(1); page <= 4; page++ {
				result := read(page)
				s.Equal(corev1.PaginationModel{Page: page, PageSize: 1, TotalItems: 3, TotalPages: 3}, result.pagination)
				if page == 4 {
					s.Empty(result.ids)
					continue
				}
				s.Require().Len(result.ids, 1)
				s.False(seen[result.ids[0]], "adjacent pages must not overlap")
				seen[result.ids[0]] = true
			}
			s.Len(seen, 3)
		})
	}
	_, err = s.dbPool.Exec(s.ctx, `INSERT INTO problem_packages (id,problem_id,organization_id,package_hash,status,version,created_at)
 SELECT gen_random_uuid(),$1,$2,md5($1::uuid::text || version::text) || md5($2::uuid::text || version::text),'ready',version,'2026-01-01' FROM generate_series(1,101) version`, problemID, org.ID)
	s.Require().NoError(err)
	versions := []int32{}
	for page := int32(1); page <= 4; page++ {
		result, err := s.client.ListProblemPackages(ctx, corev1.ListProblemPackagesParams{ID: problemID, Page: corev1.NewOptInt32(page), PageSize: corev1.NewOptInt32(50)})
		s.Require().NoError(err)
		s.Equal(corev1.PaginationModel{Page: page, PageSize: 50, TotalItems: 101, TotalPages: 3}, result.Pagination)
		s.Require().NotNil(result.Packages)
		if page == 4 {
			s.Empty(result.Packages)
		}
		for _, pkg := range result.Packages {
			versions = append(versions, pkg.Version.Value)
		}
	}
	s.Require().Len(versions, 101)
	for i, version := range versions {
		s.EqualValues(101-i, version)
	}
	for _, size := range []int32{0, -1, 101} {
		_, err := s.client.ListProblemPackages(ctx, corev1.ListProblemPackagesParams{ID: problemID, PageSize: corev1.NewOptInt32(size)})
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	}
	s.Run("empty history", func() {
		_, err := s.dbPool.Exec(s.ctx, `DELETE FROM problem_packages WHERE problem_id=$1`, problemID)
		s.Require().NoError(err)
		result, err := s.client.ListProblemPackages(ctx, corev1.ListProblemPackagesParams{ID: problemID})
		s.Require().NoError(err)
		s.Equal(corev1.PaginationModel{Page: 1, PageSize: 50}, result.Pagination)
		s.NotNil(result.Packages)
		s.Empty(result.Packages)
	})
}
