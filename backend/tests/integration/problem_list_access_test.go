//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestProblemListGuestAccess() {
	owner := s.createUser("list_guest_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("list-guest-org", "List Guest Org", owner.Id)
	var publicIDs []uuid.UUID
	for _, visibility := range []string{problemsmodel.ProblemVisibilityPublic, problemsmodel.ProblemVisibilityPublic, problemsmodel.ProblemVisibilityPrivate, problemsmodel.ProblemVisibilityUnlisted} {
		id := uuid.New()
		s.Require().NoError(s.problemsRepo.CreateProblem(s.ctx, &problemsmodel.CreateProblemParams{
			ID: id, OrganizationID: org.ID, OwnerID: &owner.Id,
			Visibility: visibility, Title: "List guest " + visibility, ShortName: id.String(),
		}))
		isTemplate := true
		s.Require().NoError(s.problemsRepo.UpdateProblem(s.ctx, id, &problemsmodel.ProblemUpdate{IsTemplate: &isTemplate}))
		if visibility == problemsmodel.ProblemVisibilityPublic {
			publicIDs = append(publicIDs, id)
		}
	}

	for _, tc := range []struct {
		name   string
		params corev1.ListProblemsParams
		want   []uuid.UUID
	}{
		{"without organization", corev1.ListProblemsParams{}, publicIDs},
		{"with organization", corev1.ListProblemsParams{OrganizationID: corev1.NewOptUUID(org.ID)}, publicIDs},
		{"search private", corev1.ListProblemsParams{Search: corev1.NewOptString("private")}, nil},
		{"templates", corev1.ListProblemsParams{IsTemplate: corev1.NewOptBool(true)}, publicIDs},
		{"non templates", corev1.ListProblemsParams{IsTemplate: corev1.NewOptBool(false)}, nil},
		{"owner", corev1.ListProblemsParams{Owner: corev1.NewOptBool(true)}, nil},
	} {
		s.Run(tc.name, func() {
			resp, err := s.client.ListProblems(s.ctx, tc.params)
			s.Require().NoError(err)
			ids := make([]uuid.UUID, 0, len(resp.Problems))
			for _, problem := range resp.Problems {
				ids = append(ids, problem.ID)
				s.Equal(problemsmodel.ProblemVisibilityPublic, problem.Visibility.Or(""))
			}
			s.ElementsMatch(tc.want, ids)
			s.Equal(paging.Total(int32(len(tc.want)), 50), resp.Pagination.TotalPages)
		})
	}

	var pagedIDs []uuid.UUID
	for page := int32(1); page <= 3; page++ {
		resp, err := s.client.ListProblems(s.ctx, corev1.ListProblemsParams{
			Page: corev1.NewOptInt32(page), PageSize: corev1.NewOptInt32(1),
		})
		s.Require().NoError(err)
		s.Equal(int32(2), resp.Pagination.TotalPages)
		for _, problem := range resp.Problems {
			pagedIDs = append(pagedIDs, problem.ID)
		}
	}
	s.ElementsMatch(publicIDs, pagedIDs)
}

func (s *IntegrationTestSuite) TestProblemListAuthenticatedAccess() {
	orgOwner := s.createUser("list_org_owner", accessmodel.UserRoleUser)
	orgAdmin := s.createUser("list_org_admin", accessmodel.UserRoleUser)
	orgMember := s.createUser("list_org_member", accessmodel.UserRoleUser)
	outsider := s.createUser("list_outsider", accessmodel.UserRoleUser)
	admin := s.createUser("list_global_admin", accessmodel.UserRoleAdmin)
	problemOwner := s.createUser("list_problem_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("list-access-org", "List Access Org", orgOwner.Id)
	otherOrg := s.createOrganization("list-other-org", "List Other Org", outsider.Id)
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, orgAdmin.Id, organizationsmodel.OrgRoleAdmin))
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, orgMember.Id, organizationsmodel.OrgRoleMember))

	createProblem := func(orgID uuid.UUID, visibility string, ownerID *uuid.UUID, template bool) uuid.UUID {
		id := uuid.New()
		s.Require().NoError(s.problemsRepo.CreateProblem(s.ctx, &problemsmodel.CreateProblemParams{
			ID: id, OrganizationID: orgID, OwnerID: ownerID,
			Visibility: visibility, Title: "List access " + visibility, ShortName: id.String(),
		}))
		s.Require().NoError(s.problemsRepo.UpdateProblem(s.ctx, id, &problemsmodel.ProblemUpdate{IsTemplate: &template}))
		return id
	}
	publicID := createProblem(org.ID, problemsmodel.ProblemVisibilityPublic, &orgOwner.Id, false)
	privateID := createProblem(org.ID, problemsmodel.ProblemVisibilityPrivate, nil, false)
	unlistedID := createProblem(org.ID, problemsmodel.ProblemVisibilityUnlisted, nil, false)
	templateID := createProblem(org.ID, problemsmodel.ProblemVisibilityPrivate, nil, true)
	ownedID := createProblem(org.ID, problemsmodel.ProblemVisibilityPrivate, &problemOwner.Id, false)
	foreignID := createProblem(otherOrg.ID, problemsmodel.ProblemVisibilityPrivate, nil, false)
	allIDs := []uuid.UUID{publicID, privateID, unlistedID, templateID, ownedID, foreignID}
	// Equal timestamps exercise the stable ordering used between pages.
	_, err := s.dbPool.Exec(s.ctx, "UPDATE problems SET created_at = '2026-01-01'::timestamptz")
	s.Require().NoError(err)

	type accessCase struct {
		name string
		user accessmodel.User
		want []uuid.UUID
	}
	cases := []accessCase{
		{"global admin", admin, []uuid.UUID{publicID, privateID, templateID, ownedID, foreignID}},
		{"organization owner", orgOwner, []uuid.UUID{publicID, privateID, templateID, ownedID}},
		{"organization admin", orgAdmin, []uuid.UUID{publicID, privateID, templateID, ownedID}},
		{"organization member", orgMember, []uuid.UUID{publicID}},
		{"outsider", outsider, []uuid.UUID{publicID, foreignID}},
		{"problem owner without organization membership", problemOwner, []uuid.UUID{publicID, ownedID}},
	}
	var directViewer accessmodel.User
	for _, role := range []problemsmodel.ProblemRole{problemsmodel.ProblemRoleViewer, problemsmodel.ProblemRoleModerator, problemsmodel.ProblemRoleOwner} {
		user := s.createUser("list_direct_"+string(role), accessmodel.UserRoleUser)
		for _, id := range []uuid.UUID{privateID, unlistedID} {
			s.Require().NoError(s.problemsRepo.CreateProblemMember(s.ctx, &problemsmodel.CreateProblemMemberParams{
				ProblemID: id, UserID: user.Id, Role: role,
			}))
		}
		cases = append(cases, accessCase{"direct " + string(role), user, []uuid.UUID{publicID, privateID}})
		if role == problemsmodel.ProblemRoleViewer {
			directViewer = user
		}
	}
	var readTeamID uuid.UUID
	var teamReader accessmodel.User
	for _, permission := range []problemsmodel.ProblemPermission{problemsmodel.ProblemPermissionRead, problemsmodel.ProblemPermissionWrite, problemsmodel.ProblemPermissionAdmin} {
		user := s.createUser("list_team_"+string(permission), accessmodel.UserRoleUser)
		s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, user.Id, organizationsmodel.OrgRoleMember))
		team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{
			OrganizationID: org.ID, Name: string(permission), Slug: string(permission), Privacy: organizationsmodel.TeamPrivacySecret,
		})
		s.Require().NoError(err)
		s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, user.Id, organizationsmodel.TeamRoleMember))
		for _, id := range []uuid.UUID{privateID, unlistedID} {
			s.Require().NoError(s.problemsRepo.AddProblemTeam(s.ctx, id, team.ID, permission))
		}
		cases = append(cases, accessCase{"team " + string(permission), user, []uuid.UUID{publicID, privateID}})
		if permission == problemsmodel.ProblemPermissionRead {
			readTeamID, teamReader = team.ID, user
		}
	}
	// Multiple grants must not duplicate problems or inflate pagination.
	s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, readTeamID, directViewer.Id, organizationsmodel.TeamRoleMember))
	// A team in another organization must not grant access (same as GetProblem).
	s.Require().NoError(s.problemsRepo.AddProblemTeam(s.ctx, foreignID, readTeamID, problemsmodel.ProblemPermissionRead))

	assertList := func(ctx context.Context, params corev1.ListProblemsParams, want []uuid.UUID) {
		params.PageSize = corev1.NewOptInt32(2)
		totalPages := paging.Total(int32(len(want)), 2)
		var ids []uuid.UUID
		for page := int32(1); page <= totalPages+1; page++ {
			params.Page = corev1.NewOptInt32(page)
			resp, err := s.client.ListProblems(ctx, params)
			s.Require().NoError(err)
			s.Equal(totalPages, resp.Pagination.TotalPages)
			s.Equal(page, resp.Pagination.Page)
			if page > totalPages {
				s.Empty(resp.Problems)
			}
			for _, problem := range resp.Problems {
				ids = append(ids, problem.ID)
			}
		}
		s.ElementsMatch(want, ids)
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			ctx := withTestUser(s.ctx, tc.user.Id)
			assertList(ctx, corev1.ListProblemsParams{}, tc.want)
			var orgIDs, privateIDs, templateIDs []uuid.UUID
			for _, id := range tc.want {
				if id != foreignID {
					orgIDs = append(orgIDs, id)
				}
				if id != publicID && id != unlistedID {
					privateIDs = append(privateIDs, id)
				}
				if id == templateID {
					templateIDs = append(templateIDs, id)
				}
			}
			assertList(ctx, corev1.ListProblemsParams{OrganizationID: corev1.NewOptUUID(org.ID)}, orgIDs)
			assertList(ctx, corev1.ListProblemsParams{Search: corev1.NewOptString("private")}, privateIDs)
			assertList(ctx, corev1.ListProblemsParams{IsTemplate: corev1.NewOptBool(true)}, templateIDs)
			for _, id := range allIDs {
				_, err := s.client.GetProblem(ctx, corev1.GetProblemParams{ID: id})
				if slices.Contains(tc.want, id) {
					s.NoError(err, "listed problem must be readable")
				} else {
					s.Equal(http.StatusForbidden, s.getStatusCode(err))
				}
			}
		})
	}
	// Exercise the production authentication middleware, without the test user header.
	authRepo := pgstore.NewAuthRepo(s.dbPool)
	sessionID := uuid.New()
	s.Require().NoError(authRepo.CreateSession(s.ctx, sessionID, problemOwner.Id, time.Now().Add(time.Hour)))
	assertList(withTestCookie(s.ctx, sessionID), corev1.ListProblemsParams{}, []uuid.UUID{publicID, ownedID})
	s.Require().NoError(authRepo.DeleteSession(s.ctx, sessionID))
	assertList(withTestCookie(s.ctx, sessionID), corev1.ListProblemsParams{}, []uuid.UUID{publicID})
	assertList(context.WithValue(s.ctx, testCookieKey, "invalid-session"), corev1.ListProblemsParams{}, []uuid.UUID{publicID})
	expiredID := uuid.New()
	s.Require().NoError(authRepo.CreateSession(s.ctx, expiredID, admin.Id, time.Now().Add(-time.Hour)))
	assertList(withTestCookie(s.ctx, expiredID), corev1.ListProblemsParams{}, []uuid.UUID{publicID})

	assertList(withTestUser(s.ctx, problemOwner.Id), corev1.ListProblemsParams{Owner: corev1.NewOptBool(true)}, []uuid.UUID{ownedID})
	assertList(withTestUser(s.ctx, admin.Id), corev1.ListProblemsParams{Owner: corev1.NewOptBool(true)}, nil)

	// Revoke both grants for the viewer, then organization inheritance and ownership.
	for _, id := range []uuid.UUID{privateID, unlistedID} {
		s.Require().NoError(s.problemsRepo.RemoveProblemMember(s.ctx, id, directViewer.Id))
		s.Require().NoError(s.problemsRepo.RemoveProblemTeam(s.ctx, id, readTeamID))
	}
	assertList(withTestUser(s.ctx, directViewer.Id), corev1.ListProblemsParams{}, []uuid.UUID{publicID})
	assertList(withTestUser(s.ctx, teamReader.Id), corev1.ListProblemsParams{}, []uuid.UUID{publicID})
	s.Require().NoError(s.organizationsRepo.RemoveMember(s.ctx, org.ID, orgAdmin.Id))
	assertList(withTestUser(s.ctx, orgAdmin.Id), corev1.ListProblemsParams{}, []uuid.UUID{publicID})
	_, err = s.dbPool.Exec(s.ctx, "UPDATE problems SET owner_id = NULL WHERE id = $1", ownedID)
	s.Require().NoError(err)
	assertList(withTestUser(s.ctx, problemOwner.Id), corev1.ListProblemsParams{}, []uuid.UUID{publicID})
}
