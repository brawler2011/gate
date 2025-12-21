package models

// Contest Actions
type ContestAction string

const (
	ActionGetContest             ContestAction = "GetContest"
	ActionUpdateContest          ContestAction = "UpdateContest"
	ActionManageContest          ContestAction = "ManageContest"
	ActionAdminContest           ContestAction = "AdminContest"
	ActionGetMonitor             ContestAction = "GetMonitor"
	ActionListUsersSubmissions   ContestAction = "ListUsersSubmissions"
	ActionListOwnSubmissions     ContestAction = "ListOwnSubmissions"
	ActionGetOtherUserSubmission ContestAction = "GetOtherUserSubmission"
	ActionGetOwnSubmission       ContestAction = "GetOwnSubmission"
	ActionCreateSubmission       ContestAction = "CreateSubmission"
)

// Problem Actions
type ProblemAction string

const (
	ActionGetProblem    ProblemAction = "GetProblem"
	ActionUpdateProblem ProblemAction = "UpdateProblem"
	ActionAdminProblem  ProblemAction = "AdminProblem"
)
