package accessmodel

type OrgAction string

const (
	ActionViewOrganization   OrgAction = "view_org"
	ActionManageOrganization OrgAction = "manage_org"
	ActionDeleteOrganization OrgAction = "delete_org"
)

type ProblemAction string

const (
	ActionViewProblem   ProblemAction = "view_problem"
	ActionUseProblem    ProblemAction = "use_problem"
	ActionEditProblem   ProblemAction = "edit_problem"
	ActionAdminProblem  ProblemAction = "admin_problem"
	ActionDeleteProblem ProblemAction = "delete_problem"
)

type ContestAction string

const (
	ActionGetContest             ContestAction = "get_contest"
	ActionGetContestProblem      ContestAction = "get_contest_problem"
	ActionUpdateContest          ContestAction = "update_contest"
	ActionAdminContest           ContestAction = "admin_contest"
	ActionManageContest          ContestAction = "manage_contest"
	ActionGetMonitor             ContestAction = "get_monitor"
	ActionListUsersSubmissions   ContestAction = "list_users_submissions"
	ActionListOwnSubmissions     ContestAction = "list_own_submissions"
	ActionGetOwnSubmission       ContestAction = "get_own_submission"
	ActionGetOtherUserSubmission ContestAction = "get_other_user_submission"
	ActionGetSubmissionDetails   ContestAction = "get_submission_details"
	ActionCreateSubmission       ContestAction = "create_submission"
)

// TeamAction describes operations on a team, never on its linked resources.
type TeamAction string

const (
	ActionViewTeam          TeamAction = "view_team"
	ActionListTeamMembers   TeamAction = "list_team_members"
	ActionUpdateTeam        TeamAction = "update_team"
	ActionDeleteTeam        TeamAction = "delete_team"
	ActionManageTeamMembers TeamAction = "manage_team_members"
	ActionLeaveTeam         TeamAction = "leave_team"
	ActionListTeamResources TeamAction = "list_team_resources"
)

const (
	ActionListOrganizationMembers       OrgAction     = "list_org_members"
	ActionManageOrganizationMembers     OrgAction     = "manage_org_members"
	ActionTransferOrganizationOwner     OrgAction     = "transfer_org_owner"
	ActionLeaveOrganization             OrgAction     = "leave_org"
	ActionCreateTeam                    OrgAction     = "create_team"
	ActionCreateProblem                 OrgAction     = "create_problem"
	ActionCreateContest                 OrgAction     = "create_contest"
	ActionJoinOrganization              OrgAction     = "join_org"
	ActionRequestOrganizationMembership OrgAction     = "request_org_membership"
	ActionManageProblemMembers          ProblemAction = "manage_problem_members"
	ActionTransferProblemOwner          ProblemAction = "transfer_problem_owner"
	ActionListProblemMembers            ProblemAction = "list_problem_members"
	ActionDeleteContest                 ContestAction = "delete_contest"
	ActionTransferContestOwner          ContestAction = "transfer_contest_owner"
	ActionViewLiveScoreboard            ContestAction = "view_live_scoreboard"
	ActionListContestMembers            ContestAction = "list_contest_members"
)

// ProblemPermissions represents calculated permissions for a problem
type ProblemPermissions struct {
	ViewProblem  bool
	EditProblem  bool
	AdminProblem bool
}

// ContestPermissions represents calculated permissions for a contest
type ContestPermissions struct {
	GetContest             bool
	UpdateContest          bool
	ManageContest          bool
	AdminContest           bool
	GetMonitor             bool
	ListUsersSubmissions   bool
	ListOwnSubmissions     bool
	GetOtherUserSubmission bool
	GetOwnSubmission       bool
	GetSubmissionDetails   bool
	CreateSubmission       bool
}
