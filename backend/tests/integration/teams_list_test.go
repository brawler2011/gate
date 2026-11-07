//go:build integration

package integration

import (
	"math"
	"net/http"
	"sort"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListTeamsSearchVisibilityAndPagination() {
	suffix := uuid.NewString()[:8]
	owner := s.createUser("list_owner_"+suffix, accessmodel.UserRoleUser)
	member := s.createUser("list_member_"+suffix, accessmodel.UserRoleUser)
	orgAdmin := s.createUser("list_org_admin_"+suffix, accessmodel.UserRoleUser)
	admin := s.createUser("list_admin_"+suffix, accessmodel.UserRoleAdmin)
	outsider := s.createUser("list_outsider_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("list-org-"+suffix, "Team list organization", owner.Id)
	otherOrg := s.createOrganization("list-other-"+suffix, "Other organization", owner.Id)
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, member.Id, organizationsmodel.OrgRoleMember))
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, orgAdmin.Id, organizationsmodel.OrgRoleAdmin))

	createdAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	create := func(orgID uuid.UUID, name string, secret bool) organizationsmodel.Team {
		privacy := organizationsmodel.TeamPrivacyClosed
		if secret {
			privacy = organizationsmodel.TeamPrivacySecret
		}
		team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{
			OrganizationID: orgID, Name: name, Slug: uuid.NewString(), Privacy: privacy,
		})
		s.Require().NoError(err)
		_, err = s.dbPool.Exec(s.ctx, "UPDATE teams SET created_at = $2 WHERE id = $1", team.ID, createdAt)
		s.Require().NoError(err)
		team.CreatedAt = createdAt
		return *team
	}
	closed := []organizationsmodel.Team{
		create(org.ID, "ПреКОмАнДаПосле", false),
		create(org.ID, "prefixALPHAsuffix", false),
		create(org.ID, "MixedКоМAlpha", false),
		create(org.ID, "Other name", false),
	}
	secretMember := create(org.ID, "Секретная КоМанда", true)
	secretHidden := create(org.ID, "Скрытая Команда", true)
	other := create(otherOrg.ID, "Другая Команда", true)
	for _, team := range append(append([]organizationsmodel.Team{}, closed...), secretMember, other) {
		s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, member.Id, organizationsmodel.TeamRoleMember))
	}
	// Multiple members must not duplicate list rows or inflate COUNT.
	for _, team := range closed {
		s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, owner.Id, organizationsmodel.TeamRoleMaintainer))
	}
	// Exercise created_at precedence as well as the id tie-breaker.
	closed[3].CreatedAt = createdAt.Add(-time.Hour)
	_, err := s.dbPool.Exec(s.ctx, "UPDATE teams SET created_at = $2 WHERE id = $1", closed[3].ID, closed[3].CreatedAt)
	s.Require().NoError(err)
	orderedIDs := func(teams []organizationsmodel.Team) []uuid.UUID {
		teams = append([]organizationsmodel.Team{}, teams...)
		sort.Slice(teams, func(i, j int) bool {
			if !teams[i].CreatedAt.Equal(teams[j].CreatedAt) {
				return teams[i].CreatedAt.After(teams[j].CreatedAt)
			}
			return teams[i].ID.String() > teams[j].ID.String()
		})
		ids := make([]uuid.UUID, len(teams))
		for i, team := range teams {
			ids[i] = team.ID
		}
		return ids
	}
	check := func(user uuid.UUID, organizationID *uuid.UUID, search string, page, size int32, want []uuid.UUID, total int) {
		params := corev1.ListTeamsParams{Search: corev1.NewOptString(search), Page: corev1.NewOptInt32(page), PageSize: corev1.NewOptInt32(size)}
		if organizationID != nil {
			params.OrganizationID = corev1.NewOptUUID(*organizationID)
		}
		response, err := s.client.ListTeams(withTestUser(s.ctx, user), params)
		s.Require().NoError(err)
		s.Require().NotNil(response.Teams, "empty pages must be JSON arrays")
		ids := make([]uuid.UUID, len(response.Teams))
		for i, team := range response.Teams {
			ids[i] = team.ID
		}
		s.Equal(want, ids)
		s.Equal(int32(total), response.Pagination.TotalItems)
		s.Equal(page, response.Pagination.Page)
	}

	s.Run("visibility", func() {
		all := append(append([]organizationsmodel.Team{}, closed...), secretMember, secretHidden)
		visible := append(append([]organizationsmodel.Team{}, closed...), secretMember)
		check(member.Id, &org.ID, "", 1, 100, orderedIDs(visible), len(visible))
		check(outsider.Id, &org.ID, "", 1, 100, orderedIDs(closed), len(closed))
		for _, user := range []uuid.UUID{owner.Id, orgAdmin.Id, admin.Id} {
			check(user, &org.ID, "", 1, 100, orderedIDs(all), len(all))
		}
		check(member.Id, nil, "", 1, 100, orderedIDs(append(visible, other)), len(visible)+1)
		// Unscoped lists use the same view predicate as scoped lists and details.
		check(admin.Id, nil, "", 1, 100, orderedIDs(append(all, other)), len(all)+1)
		check(orgAdmin.Id, nil, "", 1, 100, orderedIDs(all), len(all))
	})

	s.Run("search in both modes", func() {
		for _, scope := range []*uuid.UUID{nil, &org.ID} {
			cyrillic := []organizationsmodel.Team{closed[0], closed[2], secretMember}
			if scope == nil {
				cyrillic = append(cyrillic, other)
			}
			for _, search := range []string{"ком", "КОМ", "КоМ"} {
				check(member.Id, scope, search, 1, 100, orderedIDs(cyrillic), len(cyrillic))
			}
			for _, search := range []string{"alpha", "ALPHA", "aLpHa", "AL_HA"} {
				check(member.Id, scope, search, 1, 100, orderedIDs(closed[1:3]), 2)
			}
			check(member.Id, scope, "КОМaLpHa", 1, 100, orderedIDs(closed[2:3]), 1)
			check(member.Id, scope, "сле", 1, 100, orderedIDs(closed[:1]), 1)
			short := []organizationsmodel.Team{closed[0], secretMember}
			if scope == nil {
				short = append(short, other)
			}
			check(member.Id, scope, "р", 1, 100, orderedIDs(short), len(short))
			check(member.Id, scope, "no such name", 1, 100, []uuid.UUID{}, 0)
			visible := append(append([]organizationsmodel.Team{}, closed...), secretMember)
			if scope == nil {
				visible = append(visible, other)
			}
			check(member.Id, scope, "%", 1, 100, orderedIDs(visible), len(visible))
			check(member.Id, scope, "_", 1, 100, orderedIDs(visible), len(visible))
		}
		check(outsider.Id, &org.ID, "Скрытая", 1, 1, []uuid.UUID{}, 0)
		check(owner.Id, &org.ID, "СКРЫТАЯ", 1, 1, []uuid.UUID{secretHidden.ID}, 1)
	})

	s.Run("stable pages and filtered total", func() {
		for _, scope := range []*uuid.UUID{nil, &org.ID} {
			matched := []organizationsmodel.Team{closed[0], closed[2], secretMember}
			if scope == nil {
				matched = append(matched, other)
			}
			ids := orderedIDs(matched)
			for repeat := 0; repeat < 2; repeat++ {
				for i, id := range ids {
					check(member.Id, scope, "ком", int32(i+1), 1, []uuid.UUID{id}, len(ids))
				}
			}
			check(member.Id, scope, "ком", int32(len(ids)+1), 1, []uuid.UUID{}, len(ids))
			check(member.Id, scope, "ком", math.MaxInt32, 100, []uuid.UUID{}, len(ids))
		}
		ids := orderedIDs(append(append([]organizationsmodel.Team{}, closed...), secretMember))
		check(member.Id, &org.ID, "", 3, 2, ids[4:], len(ids))
		check(member.Id, &org.ID, "", 4, 2, []uuid.UUID{}, len(ids))
	})

	s.Run("HTTP validation", func() {
		for _, params := range []corev1.ListTeamsParams{
			{Page: corev1.NewOptInt32(0)}, {Page: corev1.NewOptInt32(-1)},
			{PageSize: corev1.NewOptInt32(0)}, {PageSize: corev1.NewOptInt32(-1)}, {PageSize: corev1.NewOptInt32(101)},
		} {
			_, err := s.client.ListTeams(withTestUser(s.ctx, member.Id), params)
			s.Require().Error(err)
			s.Equal(http.StatusBadRequest, s.getStatusCode(err))
		}
		_, err := s.client.ListTeams(s.ctx, corev1.ListTeamsParams{})
		s.Require().Error(err)
		s.Equal(http.StatusUnauthorized, s.getStatusCode(err))
	})
}

func (s *IntegrationTestSuite) TestListTeamsDefaultPageSize() {
	suffix := uuid.NewString()[:8]
	user := s.createUser("list_default_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("list_default_"+suffix, "Default page size", user.Id)
	_, err := s.dbPool.Exec(s.ctx, `WITH inserted AS (
		INSERT INTO teams (organization_id, name, slug)
		SELECT $1, 'Team ' || n, 'team-' || n FROM generate_series(1, 105) n RETURNING id
	) INSERT INTO team_members (team_id, user_id, role) SELECT id, $2, 'member' FROM inserted`, org.ID, user.Id)
	s.Require().NoError(err)
	for _, scoped := range []bool{false, true} {
		params := corev1.ListTeamsParams{}
		if scoped {
			params.OrganizationID = corev1.NewOptUUID(org.ID)
		}
		first, err := s.client.ListTeams(withTestUser(s.ctx, user.Id), params)
		s.Require().NoError(err)
		s.Len(first.Teams, 50)
		s.Equal(int32(1), first.Pagination.Page)
		s.Equal(int32(105), first.Pagination.TotalItems)
		s.Equal(int32(3), first.Pagination.TotalPages)
		params.Page = corev1.NewOptInt32(3)
		last, err := s.client.ListTeams(withTestUser(s.ctx, user.Id), params)
		s.Require().NoError(err)
		s.Len(last.Teams, 5)
		s.Equal(int32(105), last.Pagination.TotalItems)
		params.Page = corev1.NewOptInt32(1)
		params.PageSize = corev1.NewOptInt32(100)
		large, err := s.client.ListTeams(withTestUser(s.ctx, user.Id), params)
		s.Require().NoError(err)
		s.Len(large.Teams, 100)
		s.Equal(int32(105), large.Pagination.TotalItems)
		s.Equal(int32(2), large.Pagination.TotalPages)
	}
}
