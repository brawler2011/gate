package models

// Action types for permissions

type OrgAction string

const (
	ActionViewOrganization   OrgAction = "view_org"
	ActionManageOrganization OrgAction = "manage_org"
	ActionDeleteOrganization OrgAction = "delete_org"
)

type ProblemAction string

const (
	ActionViewProblem   ProblemAction = "view_problem"
	ActionEditProblem   ProblemAction = "edit_problem"
	ActionAdminProblem  ProblemAction = "admin_problem"
	ActionDeleteProblem ProblemAction = "delete_problem"
)

type ContestAction string

const (
	ActionGetContest             ContestAction = "get_contest"
	ActionUpdateContest          ContestAction = "update_contest"
	ActionAdminContest           ContestAction = "admin_contest"
	ActionManageContest          ContestAction = "manage_contest"
	ActionGetMonitor             ContestAction = "get_monitor"
	ActionListUsersSubmissions   ContestAction = "list_users_submissions"
	ActionListOwnSubmissions     ContestAction = "list_own_submissions"
	ActionGetOwnSubmission       ContestAction = "get_own_submission"
	ActionGetOtherUserSubmission ContestAction = "get_other_user_submission"
	ActionCreateSubmission       ContestAction = "create_submission"
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
	CreateSubmission       bool
}
