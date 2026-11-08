//go:build integration

package integration

import (
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	cm "github.com/brawler2011/gate/backend/internal/contests/model"
	pm "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestCapabilitiesRESTParity() {
	owner := s.createUser("caps_owner", m.UserRoleUser)
	org := s.createOrganization("caps-org", "Capabilities", owner.Id)
	ctx := withTestUser(s.ctx, owner.Id)
	problem, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{Title: "Capabilities", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	contest := uuid.New()
	end := time.Now().Add(time.Hour)
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &cm.CreateContestParams{ID: contest, OrganizationID: org.ID, OwnerID: &owner.Id, Title: "Capabilities", Login: "caps-round", Visibility: cm.ContestVisibilityPrivate, EndTime: &end, Settings: map[string]any{}}))
	detail, err := s.client.GetContest(ctx, corev1.GetContestParams{OrgLogin: org.Login, ContestLogin: "caps-round"})
	s.Require().NoError(err)
	list, err := s.client.ListOrganizationContests(ctx, corev1.ListOrganizationContestsParams{OrgLogin: org.Login})
	s.Require().NoError(err)
	s.Require().Len(list.Contests, 1)
	role, err := s.client.GetMyContestRole(ctx, corev1.GetMyContestRoleParams{OrgLogin: org.Login, ContestLogin: "caps-round"})
	s.Require().NoError(err)
	dashboard, err := s.client.GetMyDashboard(ctx)
	s.Require().NoError(err)
	s.Require().Len(dashboard.RecentContests, 1)
	s.ElementsMatch(detail.Contest.Capabilities, list.Contests[0].Capabilities)
	s.ElementsMatch(detail.Contest.Capabilities, role.Capabilities)
	s.ElementsMatch(detail.Contest.Capabilities, dashboard.RecentContests[0].Capabilities)
	s.Contains(detail.Contest.Capabilities, "submit")
	s.Contains(detail.Contest.Capabilities, "delete")
	pd, err := s.client.GetProblem(ctx, corev1.GetProblemParams{ID: problem.ID})
	s.Require().NoError(err)
	pl, err := s.client.ListProblems(ctx, corev1.ListProblemsParams{OrganizationID: corev1.NewOptUUID(org.ID)})
	s.Require().NoError(err)
	s.Require().Len(pl.Problems, 1)
	s.Require().Len(dashboard.MyProblems, 1)
	s.ElementsMatch(pd.Problem.Capabilities, pl.Problems[0].Capabilities)
	s.ElementsMatch(pd.Problem.Capabilities, dashboard.MyProblems[0].Capabilities)
	s.Contains(pd.Problem.Capabilities, "edit")
	members, err := s.client.ListOrganizationMembers(ctx, corev1.ListOrganizationMembersParams{Login: org.Login})
	s.Require().NoError(err)
	s.Require().Len(members.Members, 1)
	s.NotNil(members.Members[0].Capabilities)
	s.Empty(members.Members[0].Capabilities, "last owner's own membership cannot be removed or demoted")
	for _, grant := range []struct {
		name    string
		problem pm.ProblemRole
		contest cm.ContestRole
	}{{"viewer", pm.ProblemRoleViewer, cm.ContestRoleParticipant}, {"moderator", pm.ProblemRoleModerator, cm.ContestRoleModerator}} {
		s.Run(grant.name, func() {
			user := s.createUser("caps_"+grant.name, m.UserRoleUser)
			s.Require().NoError(s.problemsRepo.CreateProblemMember(s.ctx, &pm.CreateProblemMemberParams{ProblemID: problem.ID, UserID: user.Id, Role: grant.problem}))
			s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &cm.CreateContestMemberParams{ContestId: contest, UserId: user.Id, Role: grant.contest}))
			viewer := withTestUser(s.ctx, user.Id)
			cd, err := s.client.GetContest(viewer, corev1.GetContestParams{OrgLogin: org.Login, ContestLogin: "caps-round"})
			s.Require().NoError(err)
			cl, err := s.client.ListOrganizationContests(viewer, corev1.ListOrganizationContestsParams{OrgLogin: org.Login})
			s.Require().NoError(err)
			s.Require().Len(cl.Contests, 1)
			cr, err := s.client.GetMyContestRole(viewer, corev1.GetMyContestRoleParams{OrgLogin: org.Login, ContestLogin: "caps-round"})
			s.Require().NoError(err)
			dash, err := s.client.GetMyDashboard(viewer)
			s.Require().NoError(err)
			s.Require().Len(dash.RecentContests, 1)
			s.Require().Len(dash.MyProblems, 1)
			s.ElementsMatch(cd.Contest.Capabilities, cl.Contests[0].Capabilities)
			s.ElementsMatch(cd.Contest.Capabilities, cr.Capabilities)
			s.ElementsMatch(cd.Contest.Capabilities, dash.RecentContests[0].Capabilities)
			s.NotContains(cd.Contest.Capabilities, "delete")
			pd, err := s.client.GetProblem(viewer, corev1.GetProblemParams{ID: problem.ID})
			s.Require().NoError(err)
			pl, err := s.client.ListProblems(viewer, corev1.ListProblemsParams{OrganizationID: corev1.NewOptUUID(org.ID)})
			s.Require().NoError(err)
			s.Require().Len(pl.Problems, 1)
			s.ElementsMatch(pd.Problem.Capabilities, pl.Problems[0].Capabilities)
			s.ElementsMatch(pd.Problem.Capabilities, dash.MyProblems[0].Capabilities)
			s.NotContains(pd.Problem.Capabilities, "delete")
		})
	}
	// After finish, all projections stop offering official submit.
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, cm.ContestUpdateInput{ID: contest, EndTimeSet: true, EndTime: ptrTime(time.Now().Add(-time.Second))}))
	detail, err = s.client.GetContest(ctx, corev1.GetContestParams{OrgLogin: org.Login, ContestLogin: "caps-round"})
	s.Require().NoError(err)
	role, err = s.client.GetMyContestRole(ctx, corev1.GetMyContestRoleParams{OrgLogin: org.Login, ContestLogin: "caps-round"})
	s.Require().NoError(err)
	s.NotContains(detail.Contest.Capabilities, "submit")
	s.ElementsMatch(detail.Contest.Capabilities, role.Capabilities)
}
func ptrTime(v time.Time) *time.Time { return &v }
