//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestListContests() {
	user := s.createUser("testuser_contests", accessmodel.UserRoleUser)

	org := s.createOrganization("test-org", "Test Organization", user.Id)

	contestID := uuid.New()
	err := s.createContest(contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: org.ID,
		Title:          "Test Contest",
		Login:          "test-contest",
		Description:    "A test contest",
		Visibility:     contestsmodel.ContestVisibilityPublic,
	}, user.Id)
	s.Require().NoError(err)

	visibility := contestsmodel.ContestVisibilityPublic
	err = s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{
		ID:         contestID,
		Visibility: &visibility,
	})
	s.Require().NoError(err)

	resp, err := s.client.ListContests(withTestUser(s.ctx, user.Id), corev1.ListContestsParams{
		Page:       corev1.NewOptInt32(1),
		PageSize:   corev1.NewOptInt32(10),
		Visibility: corev1.NewOptListContestsVisibility(corev1.ListContestsVisibilityPublic),
	})
	s.Require().NoError(err)

	s.NotNil(resp)
	s.Len(resp.Contests, 1)
	s.Equal(contestID, resp.Contests[0].ID)
	s.Equal("Test Contest", resp.Contests[0].Title)
}

func (s *IntegrationTestSuite) TestListContestsPagination() {
	user := s.createUser("contest_paged_user_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("contest-paged-org-"+uuid.NewString()[:8], "Contest Paged Org", user.Id)

	prefix := "page-pub-" + uuid.NewString()[:6]
	for i := range 3 {
		id := uuid.New()
		err := s.createContest(contestsmodel.CreateContestParams{
			ID:             id,
			OrganizationID: org.ID,
			Title:          fmt.Sprintf("%s-%d", prefix, i),
			Login:          fmt.Sprintf("%s-%d", prefix, i),
			Description:    "Public contest pagination test",
			Visibility:     contestsmodel.ContestVisibilityPublic,
		}, user.Id)
		s.Require().NoError(err)
	}

	for _, invalid := range []corev1.ListContestsParams{
		{Page: corev1.NewOptInt32(0)},
		{PageSize: corev1.NewOptInt32(0)},
		{PageSize: corev1.NewOptInt32(101)},
	} {
		_, err := s.client.ListContests(withTestUser(s.ctx, user.Id), invalid)
		s.Require().Error(err)
		s.Equal(http.StatusBadRequest, s.getStatusCode(err))
	}

	resp, err := s.client.ListContests(withTestUser(s.ctx, user.Id), corev1.ListContestsParams{
		Search:   corev1.NewOptString(prefix),
		Page:     corev1.NewOptInt32(1),
		PageSize: corev1.NewOptInt32(2),
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Len(resp.Contests, 2)
}

func (s *IntegrationTestSuite) TestClarificationsLifecycle() {
	owner := s.createUser("clar_owner_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	participant := s.createUser("clar_part_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("clar-org-"+uuid.NewString()[:8], "Clar Org", owner.Id)

	contestID := uuid.New()
	login := "clar-contest-" + uuid.NewString()[:6]
	err := s.createContest(contestsmodel.CreateContestParams{
		ID:             contestID,
		OrganizationID: org.ID,
		Title:          "Clar Contest",
		Login:          login,
		Visibility:     contestsmodel.ContestVisibilityPublic,
	}, owner.Id)
	s.Require().NoError(err)

	err = s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{
		ContestId: contestID,
		UserId:    participant.Id,
		Role:      accessmodel.ContestParticipant,
	})
	s.Require().NoError(err)

	clar, err := s.client.CreateContestClarification(withTestUser(s.ctx, participant.Id), &corev1.CreateContestClarificationRequestModel{
		Question: "Is the problem statement correct?",
	}, corev1.CreateContestClarificationParams{
		ContestID: contestID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(clar)
	s.Equal("Is the problem statement correct?", clar.Question)

	listResp, err := s.client.ListContestClarifications(withTestUser(s.ctx, owner.Id), corev1.ListContestClarificationsParams{
		ContestID: contestID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(listResp)
	s.Require().Len(listResp.Clarifications, 1)

	answered, err := s.client.AnswerContestClarification(withTestUser(s.ctx, owner.Id), &corev1.AnswerContestClarificationRequestModel{
		Answer: "Yes, it is correct.",
	}, corev1.AnswerContestClarificationParams{
		ContestID:       contestID,
		ClarificationID: clar.ID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(answered)
	s.True(answered.Answer.IsSet())
	s.Equal("Yes, it is correct.", answered.Answer.Value)
}

// Join requests are resolved and canceled through one resource; the contest response carries the caller's request and role.
func (s *IntegrationTestSuite) TestContestJoinRequestResource() {
	owner := s.createUser("jr_owner", accessmodel.UserRoleUser)
	alice := s.createUser("jr_alice", accessmodel.UserRoleUser)
	bob := s.createUser("jr_bob", accessmodel.UserRoleUser)
	org := s.createOrganization("jr-org", "Join requests", owner.Id)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Title: "Requests",
		Login: "requests", Visibility: contestsmodel.ContestVisibilityPublic}, owner.Id))
	as := func(u accessmodel.User) context.Context { return withTestUser(s.ctx, u.Id) }
	s.Require().NoError(s.client.UpdateContest(as(owner), &corev1.UpdateContestRequestModel{
		JoinPolicy: corev1.NewOptUpdateContestRequestModelJoinPolicy(corev1.UpdateContestRequestModelJoinPolicyByRequest)},
		corev1.UpdateContestParams{ContestID: contestID}))
	request := func(u accessmodel.User) uuid.UUID {
		resp, err := s.client.CreateContestJoinRequest(as(u), corev1.NewOptCreateContestJoinRequestModel(corev1.CreateContestJoinRequestModel{}), corev1.CreateContestJoinRequestParams{ContestID: contestID})
		s.Require().NoError(err)
		return resp.Request.Value.ID
	}
	get := func(u accessmodel.User) *corev1.GetContestResponseModel {
		resp, err := s.client.GetContest(as(u), corev1.GetContestParams{OrgLogin: org.Login, ContestLogin: "requests"})
		s.Require().NoError(err)
		return resp
	}
	resolve := func(id uuid.UUID, status corev1.ResolveJoinRequestModelStatus) error {
		return s.client.ResolveContestJoinRequest(as(owner), &corev1.ResolveJoinRequestModel{Status: status},
			corev1.ResolveContestJoinRequestParams{ContestID: contestID, ID: id})
	}

	aliceRequest, bobRequest := request(alice), request(bob)
	s.Equal(aliceRequest, get(alice).MyJoinRequest.Value.ID)
	err := s.client.CancelContestJoinRequest(as(bob), corev1.CancelContestJoinRequestParams{ContestID: contestID, ID: aliceRequest})
	s.Equal(http.StatusNotFound, s.getStatusCode(err), "only the applicant cancels a request")
	s.Equal(http.StatusForbidden, s.getStatusCode(s.client.ResolveContestJoinRequest(as(alice),
		&corev1.ResolveJoinRequestModel{Status: corev1.ResolveJoinRequestModelStatusApproved},
		corev1.ResolveContestJoinRequestParams{ContestID: contestID, ID: aliceRequest})))

	s.Require().NoError(resolve(aliceRequest, corev1.ResolveJoinRequestModelStatusApproved))
	got := get(alice)
	s.False(got.MyJoinRequest.Set)
	s.Equal("participant", got.MyRole.Value)
	s.Equal(http.StatusConflict, s.getStatusCode(resolve(aliceRequest, corev1.ResolveJoinRequestModelStatusRejected)))

	s.Require().NoError(s.client.CancelContestJoinRequest(as(bob), corev1.CancelContestJoinRequestParams{ContestID: contestID, ID: bobRequest}))
	s.False(get(bob).MyJoinRequest.Set)
	s.Equal(http.StatusConflict, s.getStatusCode(resolve(bobRequest, corev1.ResolveJoinRequestModelStatusApproved)))
}

func (s *IntegrationTestSuite) TestListContestsTeamFilter() {
	owner := s.createUser("tf_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("tf-org", "Team filter", owner.Id)
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "Team", Slug: "team"})
	s.Require().NoError(err)
	linked, other := uuid.New(), uuid.New()
	for login, id := range map[string]uuid.UUID{"linked": linked, "other": other} {
		s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: id, OrganizationID: org.ID, Title: login,
			Login: login, Visibility: contestsmodel.ContestVisibilityPublic}, owner.Id))
	}
	s.Require().NoError(s.contestsRepo.CreateContestTeam(s.ctx, linked, team.ID, accessmodel.ContestParticipant))
	resp, err := s.client.ListContests(withTestUser(s.ctx, owner.Id), corev1.ListContestsParams{TeamID: corev1.NewOptUUID(team.ID)})
	s.Require().NoError(err)
	s.Require().Len(resp.Contests, 1)
	s.Equal(linked, resp.Contests[0].ID)
	_, err = s.client.ListContests(withTestUser(s.ctx, owner.Id), corev1.ListContestsParams{TeamID: corev1.NewOptUUID(uuid.New())})
	s.Equal(http.StatusNotFound, s.getStatusCode(err), "a team filter needs a visible team")
}
