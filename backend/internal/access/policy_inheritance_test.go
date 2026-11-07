package access

import (
	"testing"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInheritedAdministrationIsNotOwnership(t *testing.T) {
	for _, global := range []bool{false, true} {
		in, org, p, c := policyFixture()
		if global {
			in.Actor.Role = m.UserRoleAdmin
		} else {
			org.Role = m.OrganizationAdmin
		}
		p.Organization = org
		c.Organization = org
		require.NoError(t, RequireOrganization(in, org, m.ActionManageOrganization))
		require.NoError(t, RequireProblem(in, p, m.ActionAdminProblem))
		require.NoError(t, RequireContest(in, c, m.ActionManageContest))
		require.NoError(t, RequireTeam(in, m.TeamContext{ID: uuid.New(), Organization: org, Privacy: m.TeamSecret}, m.ActionDeleteTeam))
		require.ErrorIs(t, RequireOrganization(in, org, m.ActionDeleteOrganization), pkg.NoPermission)
		require.ErrorIs(t, RequireProblem(in, p, m.ActionDeleteProblem), pkg.NoPermission)
		require.ErrorIs(t, RequireContest(in, c, m.ActionDeleteContest), pkg.NoPermission)
		require.ErrorIs(t, RequireContest(in, c, m.ActionCreateSubmission), pkg.NoPermission, "inherited managers need participant admission")
		target := m.User{Id: uuid.New(), Role: m.UserRoleUser}
		before := m.OwnerState{Owners: []m.User{target}}
		_, err := PlanOrganizationOwnerTransfer(in, org, before, in.Actor, target.Id)
		require.ErrorIs(t, err, pkg.NoPermission)
		_, err = PlanProblemOwnerTransfer(in, p, before, in.Actor, target.Id)
		require.ErrorIs(t, err, pkg.NoPermission)
		_, err = PlanContestOwnerTransfer(in, c, before, in.Actor, target.Id)
		require.ErrorIs(t, err, pkg.NoPermission)
		// Global admin does not mask a real, explicit ownership grant.
		org.Role = m.OrganizationOwner
		p.DirectRole = m.ProblemOwner
		c.DirectRole = m.ContestOwner
		require.NoError(t, RequireOrganization(in, org, m.ActionDeleteOrganization))
		require.NoError(t, RequireProblem(in, p, m.ActionDeleteProblem))
		require.NoError(t, RequireContest(in, c, m.ActionDeleteContest))
	}
	in, org, p, c := policyFixture()
	org.Role = m.OrganizationMember
	p.Organization = org
	c.Organization = org
	require.Error(t, RequireProblem(in, p, m.ActionViewProblem))
	require.Error(t, RequireContest(in, c, m.ActionGetContest))
	org.Role = m.OrganizationOwner
	p.Organization = org
	c.Organization = org
	require.NoError(t, RequireProblem(in, p, m.ActionDeleteProblem))
	require.NoError(t, RequireContest(in, c, m.ActionDeleteContest))
	p.Organization.Role = ""
	c.Organization.Role = ""
	p.OwnerID = in.Actor.Id
	c.OwnerID = in.Actor.Id
	require.NoError(t, RequireProblem(in, p, m.ActionDeleteProblem))
	require.NoError(t, RequireContest(in, c, m.ActionDeleteContest))
}
func TestTeamGrantResolution(t *testing.T) {
	for _, active := range []bool{false, true} {
		for _, sameOrg := range []bool{false, true} {
			for _, membership := range []m.TeamRole{"", m.TeamMember, m.TeamMaintainer} {
				in, org, p, c := policyFixture()
				if !sameOrg {
					org.ID = uuid.New()
				}
				p.Teams = []m.TeamGrant[m.ProblemTeamRole]{{OrganizationID: org.ID, Active: active, Membership: membership, Role: m.ProblemTeamWrite}}
				c.Teams = []m.TeamGrant[m.ContestTeamRole]{{OrganizationID: org.ID, Active: active, Membership: membership, Role: m.ContestTeamModerator}}
				allowed := active && sameOrg && membership != ""
				matrixResult(t, allowed, RequireProblem(in, p, m.ActionEditProblem))
				matrixResult(t, allowed, RequireContest(in, c, m.ActionManageContest))
				require.Error(t, RequireProblem(in, p, m.ActionDeleteProblem))
				require.Error(t, RequireContest(in, c, m.ActionDeleteContest))
				// Absent or weaker grants cannot cancel a valid direct grant.
				p.DirectRole = m.ProblemModerator
				c.DirectRole = m.ContestModerator
				require.NoError(t, RequireProblem(in, p, m.ActionEditProblem))
				require.NoError(t, RequireContest(in, c, m.ActionManageContest))
			}
		}
	}
	in, org, p, c := policyFixture()
	p.Teams = []m.TeamGrant[m.ProblemTeamRole]{{OrganizationID: org.ID, Active: true, Membership: m.TeamMember, Role: m.ProblemTeamRead}, {OrganizationID: org.ID, Active: true, Membership: m.TeamMember, Role: m.ProblemTeamAdmin}}
	require.NoError(t, RequireProblem(in, p, m.ActionAdminProblem))
	require.Error(t, RequireProblem(in, p, m.ActionDeleteProblem))
	p.Teams[1].Role = "owner"
	require.Error(t, RequireProblem(in, p, m.ActionViewProblem))
	p.Teams[1].Role = m.ProblemTeamWrite
	p.Teams[1].Membership = "invalid"
	require.Error(t, RequireProblem(in, p, m.ActionViewProblem))
	c.Teams = []m.TeamGrant[m.ContestTeamRole]{{OrganizationID: org.ID, Active: true, Membership: m.TeamMember, Role: "owner"}}
	require.Error(t, RequireContest(in, c, m.ActionGetContest))
	c.Teams[0].Role = m.ContestTeamParticipant
	c.Teams[0].Membership = "invalid"
	require.Error(t, RequireContest(in, c, m.ActionGetContest))
}
func TestContestAdmissionAndSubmissionIdentity(t *testing.T) {
	for _, mode := range []m.ParticipationMode{m.ParticipationOpen, m.ParticipationByRequest, m.ParticipationInviteOnly} {
		in, _, _, c := policyFixture()
		c.ParticipationMode = mode
		require.Error(t, RequireContest(in, c, m.ActionCreateSubmission), "private never gives open admission")
		c.Visibility = m.VisibilityPublic
		matrixResult(t, mode == m.ParticipationOpen, RequireContest(in, c, m.ActionCreateSubmission))
		matrixResult(t, mode == m.ParticipationOpen, RequireContest(in, c, m.ActionListOwnSubmissions))
		in.Actor = m.Guest
		require.Error(t, RequireContest(in, c, m.ActionCreateSubmission))
	}
	in, org, _, c := policyFixture()
	c.DirectRole = m.ContestParticipant
	c.Submission.OwnerID = uuid.New()
	require.Error(t, RequireContest(in, c, m.ActionGetOwnSubmission))
	c.Submission.OwnerID = in.Actor.Id
	c.Submission.ContestID = uuid.New()
	require.Error(t, RequireContest(in, c, m.ActionGetOwnSubmission))
	require.Error(t, RequireContest(in, c, m.ActionGetSubmissionDetails))
	c.Submission.ContestID = c.ID
	c.Conditions.ViewOtherSubmission = false
	require.NoError(t, RequireContest(in, c, m.ActionGetOwnSubmission))
	require.NoError(t, RequireContest(in, c, m.ActionGetSubmissionDetails))
	c.Submission.OwnerID = uuid.New()
	require.Error(t, RequireContest(in, c, m.ActionGetSubmissionDetails))
	c.Conditions.ViewOtherSubmission = true
	c.Conditions.ViewSubmissionDetails = false
	require.Error(t, RequireContest(in, c, m.ActionGetSubmissionDetails))
	c.Organization.Role = m.OrganizationAdmin
	c.Conditions = nil
	require.Error(t, RequireContest(in, c, m.ActionCreateSubmission))
	c.Organization = org
	c.ParticipationMode = "invalid"
	require.Error(t, RequireContest(in, c, m.ActionGetContest))
}
func TestJoinAndCreation(t *testing.T) {
	in, org, _, _ := policyFixture()
	org.JoinPolicy = m.JoinByRequest
	require.NoError(t, RequireOrganization(in, org, m.ActionRequestOrganizationMembership))
	require.Error(t, RequireOrganization(in, org, m.ActionJoinOrganization))
	org.JoinPolicy = m.JoinInviteOnly
	require.Error(t, RequireOrganization(in, org, m.ActionRequestOrganizationMembership))
	require.Error(t, RequireOrganization(in, org, m.ActionJoinOrganization))
	require.Error(t, RequireCreateOrganization(in))
	in.Actor.Role = m.UserRoleAdmin
	require.NoError(t, RequireCreateOrganization(in))
	in.Actor.ExpiresAt = &in.Now
	require.ErrorIs(t, RequireCreateOrganization(in), pkg.ErrUnauthenticated)
}

func TestDetailsRequireSubmissionAccess(t *testing.T) {
	in, _, _, c := policyFixture()
	c.Visibility = m.VisibilityPublic
	c.Conditions.ViewOtherSubmission = false
	require.Error(t, RequireContest(in, c, m.ActionGetOwnSubmission))
	require.Error(t, RequireContest(in, c, m.ActionGetSubmissionDetails), "details audience alone cannot grant access to the submission")
	c.DirectRole = m.ContestParticipant
	require.NoError(t, RequireContest(in, c, m.ActionGetSubmissionDetails))
	c.Submission.OwnerID = uuid.Nil
	require.Error(t, RequireContest(in, c, m.ActionGetOtherUserSubmission))
	require.Error(t, RequireContest(in, c, m.ActionGetSubmissionDetails))
}
