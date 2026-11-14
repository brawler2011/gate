//go:build integration

package integration

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestOrganizationContestReadVisibilityAndPagination() {
	owner := s.createUser("org_read_owner", accessmodel.UserRoleUser)
	outsider := s.createUser("org_read_outsider", accessmodel.UserRoleUser)
	member := s.createUser("org_read_member", accessmodel.UserRoleUser)
	admin := s.createUser("org_read_admin", accessmodel.UserRoleAdmin)
	org := s.createOrganization("org-reads", "Reads", owner.Id)
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, member.Id, organizationsmodel.OrgRoleMember))
	publicIDs := []uuid.UUID{uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002")}
	for i, id := range append(publicIDs, uuid.New()) {
		visibility := contestsmodel.ContestVisibilityPublic
		if i == 2 {
			visibility = contestsmodel.ContestVisibilityPrivate
		}
		s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{ID: id, OrganizationID: org.ID, Title: "Matching contest", Login: id.String(), Visibility: visibility}))
	}
	_, err := s.dbPool.Exec(s.ctx, "UPDATE contests SET created_at=$1 WHERE organization_id=$2", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), org.ID)
	s.Require().NoError(err)
	params := corev1.ListOrganizationContestsParams{OrgLogin: org.Login, Search: corev1.NewOptString("Matching"), PageSize: corev1.NewOptInt32(1)}
	first, err := s.client.ListOrganizationContests(withTestUser(s.ctx, outsider.Id), params)
	s.Require().NoError(err)
	s.Require().Len(first.Contests, 1)
	s.Equal(publicIDs[1], first.Contests[0].ID)
	s.EqualValues(2, first.Pagination.TotalPages)
	params.Page = corev1.NewOptInt32(2)
	second, err := s.client.ListOrganizationContests(withTestUser(s.ctx, outsider.Id), params)
	s.Require().NoError(err)
	s.Require().Len(second.Contests, 1)
	s.Equal(publicIDs[0], second.Contests[0].ID)
	for _, actor := range []accessmodel.User{owner, member, admin} {
		response, err := s.client.ListOrganizationContests(withTestUser(s.ctx, actor.Id), params)
		s.Require().NoError(err)
		expected := 3
		if actor.Id == member.Id {
			expected = 2
		}
		s.EqualValues(expected, response.Pagination.TotalPages)
	}
	params.Page = corev1.NewOptInt32(3)
	empty, err := s.client.ListOrganizationContests(withTestUser(s.ctx, outsider.Id), params)
	s.Require().NoError(err)
	s.Empty(empty.Contests)
	s.EqualValues(2, empty.Pagination.TotalPages)
	params.Search = corev1.NewOptString("absent")
	empty, err = s.client.ListOrganizationContests(withTestUser(s.ctx, owner.Id), params)
	s.Require().NoError(err)
	s.Empty(empty.Contests)
	s.Zero(empty.Pagination.TotalPages)
}

func (s *IntegrationTestSuite) TestContestReadFiltersOrderingAndRosterPages() {
	owner := s.createUser("contest_read_owner", accessmodel.UserRoleUser)
	other := s.createUser("contest_read_other", accessmodel.UserRoleUser)
	org := s.createOrganization("contest-read-filters", "Reads", owner.Id)
	ids := make([]uuid.UUID, 3)
	for i, title := range []string{"Needle B", "Other", "Needle A"} {
		ids[i] = uuid.New()
		s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{ID: ids[i], OrganizationID: org.ID, OwnerID: &owner.Id, Title: title, Login: ids[i].String(), Visibility: contestsmodel.ContestVisibilityPublic}))
	}
	permissions := access.NewPermissionsUseCase(s.contestsRepo, s.usersRepo, s.problemsRepo, s.teamsRepo, s.organizationsRepo)
	reader := contests.NewContestsUseCase(s.contestsRepo, nil, nil, permissions)
	ctx := accessmodel.WithUser(s.ctx, owner)
	for _, orgID := range []*uuid.UUID{nil, &org.ID} {
		list, err := reader.ListWorkshopContests(ctx, contestsmodel.WorkshopContestsFilter{UserId: owner.Id, Search: "Needle", SortBy: "title", SortOrder: "asc", Page: 2, PageSize: 1, OrganizationID: orgID})
		s.Require().NoError(err)
		s.Require().Len(list.Contests, 1)
		s.Equal(ids[0], list.Contests[0].ID)
		s.EqualValues(2, list.Pagination.TotalPages)
	}
	list, err := reader.ListUserContests(ctx, contestsmodel.UserContestsFilter{UserId: owner.Id, Search: "Needle", SortBy: "title", SortOrder: "desc", Page: 2, PageSize: 1})
	s.Require().NoError(err)
	s.Require().Len(list.Contests, 1)
	s.Equal(ids[2], list.Contests[0].ID)
	s.EqualValues(2, list.Pagination.TotalPages)
	public, err := s.contestsRepo.ListPublicContests(s.ctx, contestsmodel.PublicContestsFilter{Search: "Needle", SortBy: "title", SortOrder: "asc", Page: 1, PageSize: 1})
	s.Require().NoError(err)
	s.Require().Len(public.Contests, 1)
	s.Equal(ids[2], public.Contests[0].ID)
	s.EqualValues(2, public.Pagination.TotalPages)
	_, err = reader.ListAdminContests(ctx, contestsmodel.AdminContestsFilter{})
	s.ErrorIs(err, pkg.NoPermission)
	_, err = reader.ListUserContests(ctx, contestsmodel.UserContestsFilter{UserId: other.Id})
	s.ErrorIs(err, pkg.NoPermission)
	for _, user := range []accessmodel.User{owner, other} {
		s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: ids[0], UserId: user.Id, Role: "participant"}))
	}
	first, err := reader.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{ContestId: ids[0], Page: 1, PageSize: 1})
	s.Require().NoError(err)
	second, err := reader.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{ContestId: ids[0], Page: 2, PageSize: 1})
	s.Require().NoError(err)
	s.Require().Len(first.Members, 1)
	s.Require().Len(second.Members, 1)
	s.NotEqual(first.Members[0].UserID, second.Members[0].UserID)
	s.EqualValues(2, first.Pagination.TotalPages)
}

func (s *IntegrationTestSuite) TestProblemTemplateCatalogHidesPrivateOrganizationTemplates() {
	owner := s.createUser("template_read_owner", accessmodel.UserRoleUser)
	outsider := s.createUser("template_read_outsider", accessmodel.UserRoleUser)
	org := s.createOrganization("private-template-reads", "Private templates", owner.Id)
	created, err := s.client.CreateProblem(withTestUser(s.ctx, owner.Id), corev1.CreateProblemParams{IdempotencyKey: uuid.NewString(), Title: "Private template", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	_, err = s.dbPool.Exec(s.ctx, "UPDATE problems SET is_template=true WHERE id=$1", created.ID)
	s.Require().NoError(err)
	params := corev1.ListProblemTemplatesParams{OrganizationID: corev1.NewOptUUID(org.ID)}
	ownerTemplates, err := s.client.ListProblemTemplates(withTestUser(s.ctx, owner.Id), params)
	s.Require().NoError(err)
	found := false
	for _, template := range ownerTemplates {
		if template.ID == created.ID.String() {
			found = true
		}
	}
	s.True(found)
	templates, err := s.client.ListProblemTemplates(withTestUser(s.ctx, outsider.Id), params)
	s.Require().NoError(err)
	s.Require().NotEmpty(templates)
	for _, template := range templates {
		s.True(template.IsBuiltin)
		s.NotEqual(created.ID.String(), template.ID)
	}
	_, err = s.client.ListProblemMembers(withTestUser(s.ctx, outsider.Id), corev1.ListProblemMembersParams{ID: created.ID})
	s.Equal(http.StatusForbidden, s.getStatusCode(err))
}
