//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
)

// TestAccessMatrix sends each request as every kind of actor through HTTP and the real schema.
// Expected statuses: o success, u 401, f 403, n 404.
func (s *IntegrationTestSuite) TestAccessMatrix() {
	user := func(name string) uuid.UUID { return s.createUser("matrix_"+name, accessmodel.UserRoleUser).Id }
	owner, orgAdmin, member, viewer, teamUser, participant, outsider, target := user("owner"), user("org_admin"), user("member"), user("viewer"), user("team_user"), user("participant"), user("outsider"), user("target")
	siteAdmin := s.createUser("matrix_site_admin", accessmodel.UserRoleAdmin).Id
	org := s.createOrganization("matrix", "Matrix", owner)
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, orgAdmin, accessmodel.OrganizationAdmin))
	for _, id := range []uuid.UUID{member, teamUser, target} {
		s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, id, accessmodel.OrganizationMember))
	}
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Matrix team", Slug: "matrix", Privacy: organizationsmodel.TeamPrivacyClosed})
	s.Require().NoError(err)
	s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, teamUser, accessmodel.TeamMember))
	problem, contest := uuid.New(), uuid.New()
	s.Require().NoError(dbfixtures.InsertProblem(s.ctx, s.dbPool, dbfixtures.ProblemSpec{ID: problem, OrganizationID: org.ID, Owner: owner, Title: "Matrix", ShortName: "matrix"}))
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	s.Require().NoError(dbfixtures.InsertContest(s.ctx, s.dbPool, dbfixtures.ContestSpec{ID: contest, OrganizationID: org.ID, Owner: owner, Login: "matrix", Start: &start, End: &end}))
	exec := func(query string, args ...any) {
		_, err := s.dbPool.Exec(s.ctx, query, args...)
		s.Require().NoError(err)
	}
	exec(`INSERT INTO problem_members(problem_id,user_id,role) VALUES ($1,$2,'viewer')`, problem, viewer)
	exec(`INSERT INTO problem_teams(problem_id,team_id,organization_id,role) VALUES ($1,$2,$3,'editor')`, problem, team.ID, org.ID)
	exec(`INSERT INTO contest_members(contest_id,user_id,role) VALUES ($1,$2,'participant')`, contest, participant)
	exec(`INSERT INTO contest_teams(contest_id,team_id,organization_id,role) VALUES ($1,$2,$3,'moderator')`, contest, team.ID, org.ID)

	actors := []struct {
		name string
		id   uuid.UUID
	}{{"guest", uuid.Nil}, {"outsider", outsider}, {"org member", member}, {"problem viewer", viewer}, {"team member", teamUser},
		{"participant", participant}, {"org admin", orgAdmin}, {"owner", owner}, {"site admin", siteAdmin}}
	send := func(actor uuid.UUID, method, path, body string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if actor != uuid.Nil {
			req.Header.Set("X-Test-User-ID", actor.String())
		}
		w := httptest.NewRecorder()
		s.handler.ServeHTTP(w, req)
		return w.Code
	}
	code := func(status int) byte {
		switch {
		case status >= 200 && status < 300:
			return 'o'
		case status == http.StatusUnauthorized:
			return 'u'
		case status == http.StatusForbidden:
			return 'f'
		case status == http.StatusNotFound:
			return 'n'
		}
		return '?'
	}
	contestPath := "/organizations/matrix/contests/matrix"
	revokeTarget := func() {
		exec(`DELETE FROM problem_members WHERE user_id=$1`, target)
		exec(`DELETE FROM contest_members WHERE user_id=$1`, target)
	}
	for _, tc := range []struct {
		name, method, path, body string
		want                     string
	}{
		{"view problem", http.MethodGet, "/problems/" + problem.String(), "", "unnoonooo"},
		{"edit problem", http.MethodPatch, "/problems/" + problem.String(), `{"visibility":"private"}`, "unnfonooo"},
		{"grant problem editor", http.MethodPost, fmt.Sprintf("/problems/%s/members?user_id=%s&role=editor", problem, target), "", "unnffnooo"},
		{"grant problem owner", http.MethodPost, fmt.Sprintf("/problems/%s/members?user_id=%s&role=owner", problem, target), "", "unnffnfoo"},
		{"view contest", http.MethodGet, contestPath, "", "unnnooooo"},
		{"edit contest", http.MethodPatch, contestPath, `{"title":"Matrix"}`, "unnnofooo"},
		{"add contest participant", http.MethodPost, contestPath + "/members?user_id=" + target.String(), "", "unnnofooo"},
		{"edit team", http.MethodPatch, "/teams/" + team.ID.String(), `{"name":"Matrix team"}`, "ufffffooo"},
		{"edit organization", http.MethodPatch, "/organizations/matrix", `{"name":"Matrix"}`, "ufffffooo"},
		{"list organization members", http.MethodGet, "/organizations/matrix/members", "", "ufofofooo"},
	} {
		got := make([]byte, len(actors))
		for i, a := range actors {
			got[i] = code(send(a.id, tc.method, tc.path, tc.body))
			revokeTarget()
		}
		s.Equal(tc.want, string(got), "%s (columns: guest, outsider, org member, problem viewer, team member, participant, org admin, owner, site admin)", tc.name)
	}

	s.Run("problem list", func() {
		got := make([]byte, len(actors))
		for i, a := range actors {
			ctx := s.ctx
			if a.id != uuid.Nil {
				ctx = withTestUser(ctx, a.id)
			}
			got[i] = '0'
			list, err := s.client.ListProblems(ctx, corev1.ListProblemsParams{OrganizationID: corev1.NewOptUUID(org.ID)})
			s.Require().NoError(err, a.name)
			for _, p := range list.Problems {
				if p.ID == problem {
					got[i] = '1'
				}
			}
		}
		s.Equal("000110111", string(got))
	})
}
