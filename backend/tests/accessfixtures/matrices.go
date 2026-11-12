// Package accessfixtures contains the access-policy action expectations shared by
// in-memory policies and future SQL visibility parity tests. A 1 permits access;
// a 0 denies it. D rows test only the gate; concrete changes have separate tests.
package accessfixtures

import m "github.com/brawler2011/gate/backend/internal/access/model"

type Row[A ~string] struct {
	Action  A
	Allowed string
}

// Organization columns: guest, outsider, member, admin, owner. Join policy=open.
var Organization = []Row[m.OrgAction]{
	{m.ActionViewOrganization, "11111"},
	{m.ActionManageOrganization, "00011"},
	{m.ActionDeleteOrganization, "00001"},
	{m.ActionListOrganizationMembers, "00111"},
	{m.ActionManageOrganizationMembers, "00011"},
	{m.ActionTransferOrganizationOwner, "00001"},
	{m.ActionLeaveOrganization, "00111"},
	{m.ActionCreateTeam, "00011"},
	{m.ActionCreateProblem, "00011"},
	{m.ActionCreateContest, "00011"},
	{m.ActionJoinOrganization, "01000"},
	{m.ActionRequestOrganizationMembership, "00000"},
}

// Team columns: guest, outsider, member, maintainer, org admin, org owner.
var Team = []Row[m.TeamAction]{
	{m.ActionViewTeam, "001111"},
	{m.ActionListTeamMembers, "001111"},
	{m.ActionUpdateTeam, "000111"},
	{m.ActionDeleteTeam, "000011"},
	{m.ActionManageTeamMembers, "000111"},
	{m.ActionLeaveTeam, "001100"},
	{m.ActionListTeamResources, "001111"},
}

// Problem columns: guest, outsider, viewer, moderator, admin, owner.
var Problem = []Row[m.ProblemAction]{
	{m.ActionViewProblem, "001111"},
	{m.ActionUseProblem, "001111"},
	{m.ActionEditProblem, "000111"},
	{m.ActionAdminProblem, "000011"},
	{m.ActionManageProblemMembers, "000011"},
	{m.ActionDeleteProblem, "000001"},
	{m.ActionTransferProblemOwner, "000001"},
	{m.ActionListProblemMembers, "000011"},
}

// Contest columns: guest, outsider, participant, moderator, owner.
// Conditions are all true; own-submission rows use the actual actor as owner.
// Private contest, invitation-only participation.
var Contest = []Row[m.ContestAction]{
	{m.ActionGetContest, "00111"},
	{m.ActionGetContestProblem, "00111"},
	{m.ActionUpdateContest, "00011"},
	{m.ActionAdminContest, "00011"},
	{m.ActionManageContest, "00011"},
	{m.ActionDeleteContest, "00001"},
	{m.ActionTransferContestOwner, "00001"},
	{m.ActionGetMonitor, "00111"},
	{m.ActionViewLiveScoreboard, "00011"},
	{m.ActionListUsersSubmissions, "00111"},
	{m.ActionListOwnSubmissions, "00111"},
	{m.ActionGetOwnSubmission, "00111"},
	{m.ActionGetOtherUserSubmission, "00111"},
	{m.ActionGetSubmissionDetails, "00111"},
	{m.ActionCreateSubmission, "00111"},
	{m.ActionListContestMembers, "00011"},
}
