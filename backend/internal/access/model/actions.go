package accessmodel

type OrganizationAction string

const (
	OrganizationView          OrganizationAction = "view"
	OrganizationListMembers   OrganizationAction = "list_members"
	OrganizationManage        OrganizationAction = "manage"
	OrganizationManageMembers OrganizationAction = "manage_members"
	OrganizationDelete        OrganizationAction = "delete"
	OrganizationJoin          OrganizationAction = "join"
	OrganizationRequestJoin   OrganizationAction = "request_join"
	OrganizationLeave         OrganizationAction = "leave"
)

type TeamAction string

const (
	TeamView   TeamAction = "view"
	TeamManage TeamAction = "manage"
	TeamDelete TeamAction = "delete"
	TeamLeave  TeamAction = "leave"
)

type ProblemAction string

const (
	ProblemView   ProblemAction = "view"
	ProblemUse    ProblemAction = "use"
	ProblemEdit   ProblemAction = "edit"
	ProblemManage ProblemAction = "manage"
	ProblemDelete ProblemAction = "delete"
)

type ContestAction string

const (
	ContestView               ContestAction = "view"
	ContestViewProblems       ContestAction = "view_problems"
	ContestViewStatements     ContestAction = "view_statements"
	ContestManage             ContestAction = "manage"
	ContestDelete             ContestAction = "delete"
	ContestViewScoreboard     ContestAction = "view_scoreboard"
	ContestListSubmissions    ContestAction = "list_submissions"
	ContestListOwnSubmissions ContestAction = "list_own_submissions"
	ContestSubmit             ContestAction = "submit"
	ContestStartVirtual       ContestAction = "start_virtual"
)

// SubmissionAction depends on who authored the submission.
type SubmissionAction string

const (
	SubmissionView        SubmissionAction = "view"
	SubmissionViewDetails SubmissionAction = "view_details"
)
