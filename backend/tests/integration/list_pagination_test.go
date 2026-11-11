//go:build integration

package integration

import (
	"fmt"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListPaginationAndPackageHistory() {
	owner := s.createUser("pagination_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("list-pagination", "Pagination", owner.Id)
	userIDs := []uuid.UUID{owner.Id}
	// Organizations that own packages cannot be deleted by design; the suite database is disposable.
	ctx := withTestUser(s.ctx, owner.Id)
	teamID, problemID := uuid.New(), uuid.New()
	_, err := s.dbPool.Exec(s.ctx, `INSERT INTO teams(id, organization_id, name, slug) VALUES ($1,$2,'Pagination','pagination')`, teamID, org.ID)
	s.Require().NoError(err)
	err = dbfixtures.InsertProblem(s.ctx, s.dbPool, dbfixtures.ProblemSpec{ID: problemID, OrganizationID: org.ID, OwnerID: &owner.Id, Title: "Pagination", ShortName: "pagination"})
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
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var newestFirst []uuid.UUID
	for i := range 101 {
		id, err := dbfixtures.InsertPackage(s.ctx, s.dbPool, problemID, org.ID, created.Add(time.Duration(i)*time.Second))
		s.Require().NoError(err)
		newestFirst = append([]uuid.UUID{id}, newestFirst...)
	}
	listed := []uuid.UUID{}
	for page := int32(1); page <= 4; page++ {
		result, err := s.client.ListProblemPackages(ctx, corev1.ListProblemPackagesParams{ID: problemID, Page: corev1.NewOptInt32(page), PageSize: corev1.NewOptInt32(50)})
		s.Require().NoError(err)
		s.Equal(corev1.PaginationModel{Page: page, PageSize: 50, TotalItems: 101, TotalPages: 3}, result.Pagination)
		s.Require().NotNil(result.Packages)
		if page == 4 {
			s.Empty(result.Packages)
		}
		for _, pkg := range result.Packages {
			listed = append(listed, pkg.ID)
		}
	}
	s.Equal(newestFirst, listed)
	for _, size := range []int32{0, -1, 101} {
		_, err := s.client.ListProblemPackages(ctx, corev1.ListProblemPackagesParams{ID: problemID, PageSize: corev1.NewOptInt32(size)})
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	}
	s.Run("empty history", func() {
		emptyProblem := uuid.New()
		err := dbfixtures.InsertProblem(s.ctx, s.dbPool, dbfixtures.ProblemSpec{ID: emptyProblem, OrganizationID: org.ID, OwnerID: &owner.Id, Title: "Empty", ShortName: "empty-history"})
		s.Require().NoError(err)
		result, err := s.client.ListProblemPackages(ctx, corev1.ListProblemPackagesParams{ID: emptyProblem})
		s.Require().NoError(err)
		s.Equal(corev1.PaginationModel{Page: 1, PageSize: 50}, result.Pagination)
		s.NotNil(result.Packages)
		s.Empty(result.Packages)
	})
}
