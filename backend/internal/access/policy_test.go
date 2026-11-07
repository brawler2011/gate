package access

import (
	"errors"
	"testing"
	"time"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/tests/accessfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func policyFixture() (m.PolicyInput, m.OrganizationContext, m.ProblemContext, m.ContestContext) {
	in := m.PolicyInput{Actor: m.User{Id: uuid.New(), Role: m.UserRoleUser}, Now: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}
	org := m.OrganizationContext{ID: uuid.New(), JoinPolicy: m.JoinOpen}
	problem := m.ProblemContext{ID: uuid.New(), Organization: org, Visibility: m.VisibilityPrivate}
	contest := m.ContestContext{ID: uuid.New(), Organization: org, Visibility: m.VisibilityPrivate, ParticipationMode: m.ParticipationInviteOnly,
		Conditions: &m.ContestConditions{ViewStatements: true, ViewScoreboard: true, ListSubmissions: true, ViewOtherSubmission: true, ViewSubmissionDetails: true, Submit: true}}
	contest.Submission = m.SubmissionContext{ContestID: contest.ID, OwnerID: in.Actor.Id}
	return in, org, problem, contest
}
func matrixResult(t *testing.T, allowed bool, err error) {
	t.Helper()
	if allowed {
		require.NoError(t, err)
	} else {
		require.ErrorIs(t, err, pkg.NoPermission)
	}
}
func TestOrganizationPolicyMatrix(t *testing.T) {
	for _, row := range accessfixtures.Organization {
		t.Run(string(row.Action), func(t *testing.T) {
			for col, role := range []m.OrganizationRole{"", "", m.OrganizationMember, m.OrganizationAdmin, m.OrganizationOwner} {
				in, org, _, _ := policyFixture()
				org.Role = role
				if col == 0 {
					in.Actor = m.Guest
				}
				matrixResult(t, row.Allowed[col] == '1', RequireOrganization(in, org, row.Action))
			}
		})
	}
}
func TestTeamPolicyMatrix(t *testing.T) {
	for _, privacy := range []m.TeamPrivacy{m.TeamSecret, m.TeamClosed} {
		for _, row := range accessfixtures.Team {
			t.Run(string(privacy)+"/"+string(row.Action), func(t *testing.T) {
				for col, role := range []m.TeamRole{"", "", m.TeamMember, m.TeamMaintainer, "", ""} {
					in, org, _, _ := policyFixture()
					if col == 0 {
						in.Actor = m.Guest
					}
					if col == 4 {
						org.Role = m.OrganizationAdmin
					}
					if col == 5 {
						org.Role = m.OrganizationOwner
					}
					c := m.TeamContext{ID: uuid.New(), Organization: org, Role: role, Privacy: privacy}
					allowed := row.Allowed[col] == '1'
					if col == 1 && privacy == m.TeamClosed && (row.Action == m.ActionViewTeam || row.Action == m.ActionListTeamMembers || row.Action == m.ActionListTeamResources) {
						allowed = true
					}
					matrixResult(t, allowed, RequireTeam(in, c, row.Action))
					if col >= 4 && row.Action == m.ActionLeaveTeam {
						c.Role = m.TeamMember
						require.NoError(t, RequireTeam(in, c, row.Action))
					}
				}
			})
		}
	}
}
func TestProblemPolicyMatrix(t *testing.T) {
	for _, visibility := range []m.Visibility{m.VisibilityPrivate, m.VisibilityPublic} {
		for _, row := range accessfixtures.Problem {
			t.Run(string(visibility)+"/"+string(row.Action), func(t *testing.T) {
				for col, role := range []m.ProblemRole{"", "", m.ProblemViewer, m.ProblemModerator, "", m.ProblemOwner} {
					in, _, c, _ := policyFixture()
					c.DirectRole = role
					c.Visibility = visibility
					if col == 0 {
						in.Actor = m.Guest
					}
					if col == 4 {
						c.Organization.Role = m.OrganizationAdmin
					}
					allowed := row.Allowed[col] == '1'
					if visibility == m.VisibilityPublic && (row.Action == m.ActionViewProblem || (col != 0 && row.Action == m.ActionUseProblem)) {
						allowed = true
					}
					matrixResult(t, allowed, RequireProblem(in, c, row.Action))
				}
			})
		}
	}
}
func TestContestPolicyMatrix(t *testing.T) {
	for _, public := range []bool{false, true} {
		for _, gates := range []bool{false, true} {
			for _, row := range accessfixtures.Contest {
				t.Run(string(row.Action)+"/public="+boolName(public)+"/T="+boolName(gates), func(t *testing.T) {
					for col, role := range []m.ContestRole{"", "", m.ContestParticipant, m.ContestModerator, m.ContestOwner} {
						in, _, _, c := policyFixture()
						c.DirectRole = role
						if col == 0 {
							in.Actor = m.Guest
						}
						if public {
							c.Visibility = m.VisibilityPublic
						}
						if !gates {
							c.Conditions = nil
						}
						allowed := row.Allowed[col] == '1'
						switch row.Action {
						case m.ActionGetContest:
							allowed = public || col >= 2
						case m.ActionGetContestProblem, m.ActionGetMonitor, m.ActionListUsersSubmissions, m.ActionGetOtherUserSubmission, m.ActionGetSubmissionDetails:
							allowed = col >= 3 || (gates && (public || col == 2))
						case m.ActionCreateSubmission:
							allowed = col >= 2 && gates
						}
						matrixResult(t, allowed, RequireContest(in, c, row.Action))
					}
				})
			}
		}
	}
}
func boolName(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func TestPoliciesFailClosed(t *testing.T) {
	for _, kind := range []string{"expired", "expiry equality", "load error", "unknown actor", "missing clock", "unknown org", "unknown join policy"} {
		t.Run(kind, func(t *testing.T) {
			in, org, p, c := policyFixture()
			switch kind {
			case "expired":
				expiry := in.Now.Add(-time.Second)
				in.Actor.ExpiresAt = &expiry
			case "expiry equality":
				in.Actor.ExpiresAt = &in.Now
			case "load error":
				in.LoadError = errors.New("database unavailable")
			case "unknown actor":
				in.Actor.Role = "root"
			case "missing clock":
				in.Now = time.Time{}
			case "unknown org":
				org.Role = "root"
			case "unknown join policy":
				org.JoinPolicy = "invalid"
			}
			p.Visibility = m.VisibilityPublic
			c.Visibility = m.VisibilityPublic
			p.Organization = org
			c.Organization = org
			require.Error(t, RequireOrganization(in, org, m.ActionViewOrganization))
			require.Error(t, RequireTeam(in, m.TeamContext{ID: uuid.New(), Organization: org, Privacy: m.TeamClosed}, m.ActionViewTeam))
			require.Error(t, RequireProblem(in, p, m.ActionViewProblem))
			require.Error(t, RequireContest(in, c, m.ActionGetContest))
		})
	}
	in, org, p, c := policyFixture()
	in.Actor.Role = m.UserRoleAdmin
	require.ErrorIs(t, RequireOrganization(in, org, "unknown"), pkg.NoPermission)
	require.ErrorIs(t, RequireProblem(in, p, "unknown"), pkg.NoPermission)
	require.ErrorIs(t, RequireContest(in, c, "unknown"), pkg.NoPermission)
	require.ErrorIs(t, RequireTeam(in, m.TeamContext{ID: uuid.New(), Organization: org, Privacy: m.TeamClosed}, "unknown"), pkg.NoPermission)
	p.DirectRole = "unknown"
	require.Error(t, RequireProblem(in, p, m.ActionViewProblem))
	c.DirectRole = "unknown"
	require.Error(t, RequireContest(in, c, m.ActionGetContest))
}
