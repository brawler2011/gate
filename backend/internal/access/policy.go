package access

import (
	"fmt"

	m "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func validateInput(in m.PolicyInput) error {
	if in.LoadError != nil {
		return fmt.Errorf("load policy facts: %w", in.LoadError)
	}
	if in.Now.IsZero() {
		return pkg.NoPermission
	}
	u := in.Actor
	if u.Role == m.UserRoleGuest && u.Id == uuid.Nil {
		return nil
	}
	if u.Id == uuid.Nil || (u.Role != m.UserRoleUser && u.Role != m.UserRoleAdmin) {
		return pkg.ErrUnauthenticated
	}
	if u.ExpiresAt != nil && !in.Now.Before(*u.ExpiresAt) {
		return pkg.ErrUnauthenticated
	}
	return nil
}

func orgRank(r m.OrganizationRole) int {
	switch r {
	case "":
		return 0
	case m.OrganizationMember:
		return 1
	case m.OrganizationAdmin:
		return 2
	case m.OrganizationOwner:
		return 3
	default:
		return -1
	}
}
func teamRank(r m.TeamRole) int {
	switch r {
	case "":
		return 0
	case m.TeamMember:
		return 1
	case m.TeamMaintainer:
		return 2
	default:
		return -1
	}
}
func problemRank(r m.ProblemRole) int {
	switch r {
	case "":
		return 0
	case m.ProblemViewer:
		return 1
	case m.ProblemModerator:
		return 2
	case m.ProblemAdmin:
		return 3
	case m.ProblemOwner:
		return 4
	default:
		return -1
	}
}
func problemTeamRank(r m.ProblemTeamRole) int {
	switch r {
	case m.ProblemTeamRead:
		return 1
	case m.ProblemTeamWrite:
		return 2
	case m.ProblemTeamAdmin:
		return 3
	default:
		return -1
	}
}
func contestRank(r m.ContestRole) int {
	switch r {
	case "":
		return 0
	case m.ContestParticipant:
		return 1
	case m.ContestModerator:
		return 2
	case m.ContestOwner:
		return 3
	default:
		return -1
	}
}
func contestTeamRank(r m.ContestTeamRole) int {
	switch r {
	case m.ContestTeamParticipant:
		return 1
	case m.ContestTeamModerator:
		return 2
	default:
		return -1
	}
}

func organizationRank(in m.PolicyInput, c m.OrganizationContext) (int, error) {
	if err := validateInput(in); err != nil {
		return 0, err
	}
	r := orgRank(c.Role)
	if c.ID == uuid.Nil || r < 0 || (c.JoinPolicy != m.JoinOpen && c.JoinPolicy != m.JoinByRequest && c.JoinPolicy != m.JoinInviteOnly) {
		return 0, pkg.NoPermission
	}
	if in.Actor.IsGuest() {
		return 0, nil
	}
	if in.Actor.IsAdmin() {
		r = max(r, 2)
	}
	return r, nil
}
func teamPolicyRank(in m.PolicyInput, c m.TeamContext) (int, error) {
	org, err := organizationRank(in, c.Organization)
	if err != nil {
		return 0, err
	}
	r := teamRank(c.Role)
	if c.ID == uuid.Nil || r < 0 || (c.Privacy != m.TeamClosed && c.Privacy != m.TeamSecret) {
		return 0, pkg.NoPermission
	}
	if in.Actor.IsGuest() {
		return 0, nil
	}
	if org >= 2 {
		r = 3
	}
	return r, nil
}
func validVisibility(v m.Visibility) bool { return v == m.VisibilityPublic || v == m.VisibilityPrivate }

func problemPolicyRank(in m.PolicyInput, c m.ProblemContext) (int, error) {
	org, err := organizationRank(in, c.Organization)
	if err != nil {
		return 0, err
	}
	r := problemRank(c.DirectRole)
	if c.ID == uuid.Nil || r < 0 || c.DirectRole == m.ProblemAdmin || !validVisibility(c.Visibility) {
		return 0, pkg.NoPermission
	}
	for _, g := range c.Teams {
		rank := problemTeamRank(g.Role)
		if rank < 0 || teamRank(g.Membership) < 0 {
			return 0, pkg.NoPermission
		}
		if g.Active && g.Membership != "" && g.OrganizationID == c.Organization.ID {
			r = max(r, rank)
		}
	}
	if in.Actor.IsGuest() {
		return 0, nil
	}
	if org == 3 || c.OwnerID == in.Actor.Id {
		r = 4
	} else if org == 2 {
		r = max(r, 3)
	}
	return r, nil
}
func contestPolicyRank(in m.PolicyInput, c m.ContestContext) (int, error) {
	org, err := organizationRank(in, c.Organization)
	if err != nil {
		return 0, err
	}
	r := contestRank(c.DirectRole)
	if c.ID == uuid.Nil || r < 0 || !validVisibility(c.Visibility) || (c.ParticipationMode != m.ParticipationOpen && c.ParticipationMode != m.ParticipationByRequest && c.ParticipationMode != m.ParticipationInviteOnly) {
		return 0, pkg.NoPermission
	}
	for _, g := range c.Teams {
		rank := contestTeamRank(g.Role)
		if rank < 0 || teamRank(g.Membership) < 0 {
			return 0, pkg.NoPermission
		}
		if g.Active && g.Membership != "" && g.OrganizationID == c.Organization.ID {
			r = max(r, rank)
		}
	}
	if in.Actor.IsGuest() {
		return 0, nil
	}
	if org == 3 || c.OwnerID == in.Actor.Id {
		r = 3
	} else if org == 2 {
		r = max(r, 2)
	}
	return r, nil
}
func decision(allowed bool) error {
	if !allowed {
		return pkg.NoPermission
	}
	return nil
}

// RequireOrganization evaluates only the action gate. ACL operations additionally
// require RequireGrantOrgRole; leave additionally requires RequireLeaveOrganization.
func RequireOrganization(in m.PolicyInput, c m.OrganizationContext, action m.OrgAction) error {
	r, err := organizationRank(in, c)
	if err != nil {
		return err
	}
	switch action {
	case m.ActionViewOrganization:
		return nil
	case m.ActionManageOrganization, m.ActionManageOrganizationMembers, m.ActionCreateTeam, m.ActionCreateProblem, m.ActionCreateContest:
		return decision(r >= 2)
	case m.ActionDeleteOrganization, m.ActionTransferOrganizationOwner:
		return decision(r == 3)
	case m.ActionListOrganizationMembers:
		return decision(r >= 1)
	case m.ActionLeaveOrganization:
		return decision(!in.Actor.IsGuest() && c.Role != "")
	case m.ActionJoinOrganization:
		return decision(!in.Actor.IsGuest() && c.Role == "" && c.JoinPolicy == m.JoinOpen)
	case m.ActionRequestOrganizationMembership:
		return decision(!in.Actor.IsGuest() && c.Role == "" && c.JoinPolicy == m.JoinByRequest)
	default:
		return pkg.NoPermission
	}
}
func RequireCreateOrganization(in m.PolicyInput) error {
	if err := validateInput(in); err != nil {
		return err
	}
	return decision(in.Actor.IsAdmin())
}
func RequireTeam(in m.PolicyInput, c m.TeamContext, action m.TeamAction) error {
	r, err := teamPolicyRank(in, c)
	if err != nil {
		return err
	}
	switch action {
	case m.ActionViewTeam, m.ActionListTeamMembers, m.ActionListTeamResources:
		return decision(!in.Actor.IsGuest() && (r > 0 || c.Privacy == m.TeamClosed))
	case m.ActionUpdateTeam, m.ActionManageTeamMembers:
		return decision(r >= 2)
	case m.ActionDeleteTeam:
		return decision(r == 3)
	case m.ActionLeaveTeam:
		return decision(!in.Actor.IsGuest() && c.Role != "")
	default:
		return pkg.NoPermission
	}
}
func RequireProblem(in m.PolicyInput, c m.ProblemContext, action m.ProblemAction) error {
	r, err := problemPolicyRank(in, c)
	if err != nil {
		return err
	}
	switch action {
	case m.ActionViewProblem:
		return decision(c.Visibility == m.VisibilityPublic || r > 0)
	case m.ActionUseProblem:
		return decision(!in.Actor.IsGuest() && (c.Visibility == m.VisibilityPublic || r > 0))
	case m.ActionEditProblem:
		return decision(r >= 2)
	case m.ActionAdminProblem, m.ActionManageProblemMembers, m.ActionListProblemMembers:
		return decision(r >= 3)
	case m.ActionDeleteProblem, m.ActionTransferProblemOwner:
		return decision(r == 4)
	default:
		return pkg.NoPermission
	}
}
func RequireContest(in m.PolicyInput, c m.ContestContext, action m.ContestAction) error {
	r, err := contestPolicyRank(in, c)
	if err != nil {
		return err
	}
	view := c.Visibility == m.VisibilityPublic || r > 0
	manager := r >= 2
	eligible := !in.Actor.IsGuest() && (r == 1 || c.DirectRole != "" || hasContestParticipation(c) || (c.Visibility == m.VisibilityPublic && c.ParticipationMode == m.ParticipationOpen))
	t := m.ContestConditions{}
	if c.Conditions != nil {
		t = *c.Conditions
	}
	submissionValid := c.Submission.OwnerID != uuid.Nil && c.Submission.ContestID == c.ID
	own := !in.Actor.IsGuest() && submissionValid && c.Submission.OwnerID == in.Actor.Id
	ownAccess := own && (r > 0 || eligible)
	switch action {
	case m.ActionGetContest:
		return decision(view)
	case m.ActionGetContestProblem:
		return decision(manager || (view && t.ViewStatements))
	case m.ActionUpdateContest, m.ActionAdminContest, m.ActionManageContest, m.ActionViewLiveScoreboard, m.ActionListContestMembers:
		return decision(manager)
	case m.ActionDeleteContest, m.ActionTransferContestOwner:
		return decision(r == 3)
	case m.ActionGetMonitor:
		return decision(manager || (view && t.ViewScoreboard))
	case m.ActionListUsersSubmissions:
		return decision(manager || (view && t.ListSubmissions))
	case m.ActionListOwnSubmissions:
		return decision(!in.Actor.IsGuest() && (r > 0 || eligible))
	case m.ActionGetOwnSubmission:
		return decision(ownAccess)
	case m.ActionGetOtherUserSubmission:
		return decision(submissionValid && (manager || (view && t.ViewOtherSubmission)))
	case m.ActionGetSubmissionDetails:
		return decision(submissionValid && (manager || (view && t.ViewSubmissionDetails && (ownAccess || t.ViewOtherSubmission))))
	case m.ActionCreateSubmission:
		return decision(eligible && t.Submit)
	default:
		return pkg.NoPermission
	}
}
func hasContestParticipation(c m.ContestContext) bool {
	for _, g := range c.Teams {
		if g.Active && g.Membership != "" && g.OrganizationID == c.Organization.ID {
			return true
		}
	}
	return false
}
