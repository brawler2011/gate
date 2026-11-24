//go:build integration

package integration

import (
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestContestInvitation() {
	owner := s.createUser("cinv_owner", accessmodel.UserRoleUser)
	guest := s.createUser("cinv_guest", accessmodel.UserRoleUser)
	other := s.createUser("cinv_other", accessmodel.UserRoleUser)
	org := s.createOrganization("cinv", "Contest invitations", owner.Id)
	contest := uuid.New()
	s.Require().NoError(dbfixtures.InsertContest(s.ctx, s.dbPool, dbfixtures.ContestSpec{ID: contest, OrganizationID: org.ID, Owner: owner.Id, Login: "final"}))
	params := corev1.InviteContestParticipantParams{ContestID: contest}
	inviteAs := func(user accessmodel.User) (*corev1.InvitationModel, error) {
		return s.client.InviteContestParticipant(withTestUser(s.ctx, owner.Id), &corev1.InviteContestParticipantRequestModel{UserID: user.Id}, params)
	}

	inv, err := inviteAs(guest)
	s.Require().NoError(err)
	_, err = inviteAs(guest)
	s.Equal(http.StatusConflict, s.getStatusCode(err), "one pending invitation per user")

	_, err = s.client.GetContest(withTestUser(s.ctx, guest.Id), corev1.GetContestParams{OrgLogin: "cinv", ContestLogin: "final"})
	s.Equal(http.StatusNotFound, s.getStatusCode(err), "a pending invitation grants nothing")
	err = s.client.RespondInvitation(withTestUser(s.ctx, other.Id), &corev1.RespondInvitationRequestModel{Status: corev1.RespondInvitationRequestModelStatusAccepted}, corev1.RespondInvitationParams{ID: inv.ID})
	s.Equal(http.StatusNotFound, s.getStatusCode(err), "only the invitee may accept")

	s.Require().NoError(s.client.RespondInvitation(withTestUser(s.ctx, guest.Id), &corev1.RespondInvitationRequestModel{Status: corev1.RespondInvitationRequestModelStatusAccepted}, corev1.RespondInvitationParams{ID: inv.ID}))
	s.Require().Error(s.client.RespondInvitation(withTestUser(s.ctx, guest.Id), &corev1.RespondInvitationRequestModel{Status: corev1.RespondInvitationRequestModelStatusAccepted}, corev1.RespondInvitationParams{ID: inv.ID}), "an invitation is used once")
	got, err := s.client.GetContest(withTestUser(s.ctx, guest.Id), corev1.GetContestParams{OrgLogin: "cinv", ContestLogin: "final"})
	s.Require().NoError(err)
	s.Equal("participant", got.MyRole.Value, "a participant without joining the organization")
	_, err = s.organizationsRepo.GetMember(s.ctx, org.ID, guest.Id)
	s.Require().Error(err)

	_, err = inviteAs(guest)
	s.Equal(http.StatusConflict, s.getStatusCode(err), "already a participant")

	inv, err = inviteAs(other)
	s.Require().NoError(err)
	s.Require().NoError(s.client.RespondInvitation(withTestUser(s.ctx, other.Id), &corev1.RespondInvitationRequestModel{Status: corev1.RespondInvitationRequestModelStatusDeclined}, corev1.RespondInvitationParams{ID: inv.ID}))
	list, err := s.client.ListContestInvitations(withTestUser(s.ctx, owner.Id), corev1.ListContestInvitationsParams{ContestID: contest})
	s.Require().NoError(err)
	s.Empty(list.Invitations)
}

func (s *IntegrationTestSuite) TestOrganizationInvitationCancel() {
	owner := s.createUser("oinv_owner", accessmodel.UserRoleUser)
	maintainer := s.createUser("oinv_maintainer", accessmodel.UserRoleUser)
	invitee := s.createUser("oinv_invitee", accessmodel.UserRoleUser)
	org := s.createOrganization("oinv", "Org invitations", owner.Id)
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, maintainer.Id, accessmodel.OrganizationMaintainer))
	invite := func(role corev1.InviteOrganizationMemberRequestModelRole) uuid.UUID {
		inv, err := s.client.InviteOrganizationMember(withTestUser(s.ctx, owner.Id), &corev1.InviteOrganizationMemberRequestModel{UserID: invitee.Id, Role: role}, corev1.InviteOrganizationMemberParams{Login: "oinv"})
		s.Require().NoError(err)
		return inv.ID
	}
	cancel := func(user accessmodel.User, id uuid.UUID) error {
		return s.client.CancelInvitation(withTestUser(s.ctx, user.Id), corev1.CancelInvitationParams{ID: id})
	}

	admin := invite(corev1.InviteOrganizationMemberRequestModelRoleAdmin)
	s.Equal(http.StatusForbidden, s.getStatusCode(cancel(maintainer, admin)), "maintainers cancel only roles they may grant")
	s.Equal(http.StatusForbidden, s.getStatusCode(cancel(invitee, admin)), "invitees decline rather than cancel")
	s.Require().NoError(cancel(owner, admin))
	member := invite(corev1.InviteOrganizationMemberRequestModelRoleMember)
	s.Require().NoError(cancel(maintainer, member))
	s.Require().Error(s.client.RespondInvitation(withTestUser(s.ctx, invitee.Id), &corev1.RespondInvitationRequestModel{Status: corev1.RespondInvitationRequestModelStatusAccepted}, corev1.RespondInvitationParams{ID: member}))
}

func (s *IntegrationTestSuite) TestJoinLinks() {
	owner := s.createUser("link_owner", accessmodel.UserRoleUser)
	teacher := s.createUser("link_teacher", accessmodel.UserRoleUser)
	student := s.createUser("link_student", accessmodel.UserRoleUser)
	org := s.createOrganization("links", "Join links", owner.Id)
	_, err := s.dbPool.Exec(s.ctx, "UPDATE organizations SET join_policy = 'invite_only' WHERE id = $1", org.ID)
	s.Require().NoError(err)
	s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, teacher.Id, accessmodel.OrganizationMember))
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "10A", Slug: "10a"})
	s.Require().NoError(err)
	s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, teacher.Id, accessmodel.TeamMaintainer))
	asTeacher, asStudent := withTestUser(s.ctx, teacher.Id), withTestUser(s.ctx, student.Id)

	_, err = s.client.ResetTeamJoinLink(asStudent, corev1.ResetTeamJoinLinkParams{ID: team.ID})
	s.Equal(http.StatusForbidden, s.getStatusCode(err), "only team maintainers manage the link")
	link, err := s.client.GetTeamJoinLink(asTeacher, corev1.GetTeamJoinLinkParams{ID: team.ID})
	s.Require().NoError(err)
	s.False(link.Token.Set, "links start off")

	link, err = s.client.ResetTeamJoinLink(asTeacher, corev1.ResetTeamJoinLinkParams{ID: team.ID})
	s.Require().NoError(err)
	token := link.Token.Value
	target, err := s.client.GetJoinLinkTarget(asStudent, corev1.GetJoinLinkTargetParams{Token: token})
	s.Require().NoError(err)
	s.Equal("10A", target.TeamName.Value)
	_, err = s.organizationsRepo.GetMember(s.ctx, org.ID, student.Id)
	s.Require().Error(err, "previewing a link joins nothing")

	for range 2 {
		_, err = s.client.FollowJoinLink(asStudent, corev1.FollowJoinLinkParams{Token: token})
		s.Require().NoError(err, "a link joins even an invite-only organization, and following it twice is harmless")
	}
	member, err := s.organizationsRepo.GetMember(s.ctx, org.ID, student.Id)
	s.Require().NoError(err)
	s.Equal(accessmodel.OrganizationMember, member.Role)
	_, err = s.teamsRepo.GetTeamMember(s.ctx, team.ID, student.Id)
	s.Require().NoError(err)

	fresh, err := s.client.ResetTeamJoinLink(asTeacher, corev1.ResetTeamJoinLinkParams{ID: team.ID})
	s.Require().NoError(err)
	_, err = s.client.GetJoinLinkTarget(asStudent, corev1.GetJoinLinkTargetParams{Token: token})
	s.Equal(http.StatusNotFound, s.getStatusCode(err), "regenerating kills the old link")
	s.Require().NoError(s.client.DisableTeamJoinLink(asTeacher, corev1.DisableTeamJoinLinkParams{ID: team.ID}))
	_, err = s.client.FollowJoinLink(asStudent, corev1.FollowJoinLinkParams{Token: fresh.Token.Value})
	s.Equal(http.StatusNotFound, s.getStatusCode(err))

	contest := uuid.New()
	s.Require().NoError(dbfixtures.InsertContest(s.ctx, s.dbPool, dbfixtures.ContestSpec{ID: contest, OrganizationID: org.ID, Owner: owner.Id, Login: "olymp"}))
	outsider := s.createUser("link_outsider", accessmodel.UserRoleUser)
	contestLink, err := s.client.ResetContestJoinLink(withTestUser(s.ctx, owner.Id), corev1.ResetContestJoinLinkParams{ContestID: contest})
	s.Require().NoError(err)
	_, err = s.client.FollowJoinLink(withTestUser(s.ctx, outsider.Id), corev1.FollowJoinLinkParams{Token: contestLink.Token.Value})
	s.Require().NoError(err)
	got, err := s.client.GetContest(withTestUser(s.ctx, outsider.Id), corev1.GetContestParams{OrgLogin: "links", ContestLogin: "olymp"})
	s.Require().NoError(err)
	s.Equal("participant", got.MyRole.Value)
	_, err = s.organizationsRepo.GetMember(s.ctx, org.ID, outsider.Id)
	s.Require().Error(err, "a contest link does not join the organization")
}
