package submissionsmodel

import (
	"time"

	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/points"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type State = int32

const (
	Saved State = 1

	GotCE State = 101 // compilation error
	GotTL State = 102 // time limit exceeded
	GotML State = 103 // memory limit exceeded
	GotRE State = 104 // runtime error
	GotPE State = 105 // presentation error
	GotWA State = 106 // wrong answer
	GotIE State = 107 // internal error
	// Partial is a points submission that scored above zero but below the maximum.
	Partial State = 108

	Accepted State = 200

	Disqualified State = 300
)

// Kind records how the author took part when submitting; only contest submissions reach the standings.
type Kind string

const (
	KindContest   Kind = "contest"
	KindUpsolving Kind = "upsolving"
	KindVirtual   Kind = "virtual"
)

type TestDetailItem struct {
	TestIndex int32  `json:"test_index"`
	Verdict   string `json:"verdict"`
	TimeMs    int32  `json:"time_ms"`
	MemoryKb  int32  `json:"memory_kb"`
	Sample    bool   `json:"sample,omitempty"`
	Group     string `json:"group,omitempty"`
	// Points the test earned on a points problem.
	Points *points.Value `json:"points,omitempty"`
}

// GroupResult scores of a pass-fail problem are nil; the unnamed one is the testset.
type GroupResult struct {
	Name     string        `json:"name"`
	Feedback string        `json:"feedback,omitempty"`
	Score    *points.Value `json:"score,omitempty"`
	Points   *points.Value `json:"points,omitempty"`
}

type FailedTestDetail struct {
	TestIndex int32 `json:"test_index"`
	// Public marks a sample whose input the statement shows as is.
	Public        bool   `json:"public,omitempty"`
	Input         string `json:"input"`
	Output        string `json:"output"`
	Answer        string `json:"answer"`
	CheckerOutput string `json:"checker_output"`
	ErrorMessage  string `json:"error_message"`
	IsTruncated   bool   `json:"is_truncated"`
}

type SubmissionTestDetails struct {
	TimeStat          *int32            `json:"time_stat,omitempty"`
	MemoryStat        *int32            `json:"memory_stat,omitempty"`
	FailedTest        *int32            `json:"failed_test,omitempty"`
	CompilerOutput    *string           `json:"compiler_output,omitempty"`
	Tests             []TestDetailItem  `json:"tests,omitempty"`
	Groups            []GroupResult     `json:"groups,omitempty"`
	FailedTestDetails *FailedTestDetail `json:"failed_test_details,omitempty"`
}

type SubmissionUpdate struct {
	State       State
	Score       points.Value
	TimeStat    int32
	MemoryStat  int32
	FailedTest  *int32
	TestDetails *SubmissionTestDetails
	Pretests    bool
	// Recorded only when a benchmark calibrated the host.
	Calibration calibration.Calibration
}

// OutputsLanguage stands in for a language on output-only submissions, which run nothing.
const OutputsLanguage = "output"

type SubmissionCreation struct {
	Solution string
	// Outputs is the SHA-256 of an output-only submission's outputs archive; Solution is then empty.
	Outputs   *string
	ProblemId uuid.UUID
	ContestId uuid.UUID
	UserId    uuid.UUID
	Language  string
	Penalty   int32
	Kind      Kind
	State     *State
	BanReason *string
}

type SubmissionsFilter struct {
	Page      int32
	PageSize  int32
	ContestId *uuid.UUID
	UserId    *uuid.UUID
	ProblemId *uuid.UUID
	Language  *string
	State     *State
	Order     *int32
}

type RejudgeFilter struct {
	ContestID    uuid.UUID
	ProblemID    *uuid.UUID
	SubmissionID *uuid.UUID
	UserID       *uuid.UUID
}

type SubmissionListItem struct {
	ID                uuid.UUID    `json:"id"`
	UserID            uuid.UUID    `json:"user_id"`
	Username          string       `json:"username"`
	State             State        `json:"state"`
	Score             points.Value `json:"score"`
	Penalty           int32        `json:"penalty"`
	TimeStat          int32        `json:"time_stat"`
	MemoryStat        int32        `json:"memory_stat"`
	Language          string       `json:"language"`
	FailedTest        *int32       `json:"failed_test,omitempty"`
	BanReason         *string      `json:"ban_reason,omitempty"`
	ProblemID         uuid.UUID    `json:"problem_id"`
	ProblemTitle      string       `json:"problem_title"`
	Position          int32        `json:"position"`
	ContestID         uuid.UUID    `json:"contest_id"`
	ContestLogin      string       `json:"contest_login"`
	ContestTitle      string       `json:"contest_title"`
	OrganizationLogin string       `json:"organization_login"`
	UpdatedAt         string       `json:"updated_at"`
	CreatedAt         string       `json:"created_at"`
}

type Submission struct {
	Revision     int64                  `json:"revision"`
	ResultHidden bool                   `json:"-"`
	SourceHidden bool                   `json:"-"`
	ID           uuid.UUID              `json:"id"`
	Kind         Kind                   `json:"kind"`
	CreatedBy    *uuid.UUID             `json:"created_by"`
	Username     string                 `json:"username"`
	Submission   string                 `json:"submission"`
	State        State                  `json:"state"`
	Score        points.Value           `json:"score"`
	Penalty      int32                  `json:"penalty"`
	TimeStat     int32                  `json:"time_stat"`
	MemoryStat   int32                  `json:"memory_stat"`
	Language     string                 `json:"language"`
	FailedTest   *int32                 `json:"failed_test,omitempty"`
	TestDetails  *SubmissionTestDetails `json:"test_details,omitempty"`
	// Pretests marks a contest submission judged on pretests only, until system testing.
	Pretests bool    `json:"pretests,omitempty"`
	Outputs  *string `json:"-"`
	// FailedTestFeedback stands in for TestDetails in lists, which do not load reports.
	FailedTestFeedback string     `json:"-"`
	BanReason          *string    `json:"ban_reason,omitempty"`
	ProblemID          *uuid.UUID `json:"problem_id"`
	ProblemTitle       string     `json:"problem_title"`
	Position           *int32     `json:"position"`
	ContestID          *uuid.UUID `json:"contest_id"`
	ContestLogin       string     `json:"contest_login"`
	ContestTitle       string     `json:"contest_title"`
	OrganizationLogin  string     `json:"organization_login"`
	UpdatedAt          time.Time  `json:"updated_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

type SubmissionsList struct {
	TotalCount  int32             `json:"total_count"`
	Submissions []Submission      `json:"submissions"`
	Pagination  paging.Pagination `json:"pagination"`
}

type SubmissionEventType = string

const (
	SubmissionEventCreated          SubmissionEventType = "submissions.created"
	SubmissionEventQueued           SubmissionEventType = "submissions.queued"
	SubmissionEventCompilingStarted SubmissionEventType = "submissions.compiling_started"
	SubmissionEventTestingStarted   SubmissionEventType = "submissions.testing_started"
	SubmissionEventTestStarted      SubmissionEventType = "submissions.test_started"
	SubmissionEventCompleted        SubmissionEventType = "submissions.completed"
)

type SubmissionEventMeta struct {
	Revision  int64      `json:"revision"`
	Kind      Kind       `json:"kind,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	UserId    *uuid.UUID `json:"user_id,omitempty"`
	Username  string     `json:"username"`

	ContestId    *uuid.UUID `json:"contest_id,omitempty"`
	ContestTitle string     `json:"contest_title"`

	ProblemId    *uuid.UUID `json:"problem_id,omitempty"`
	ProblemTitle string     `json:"problem_title"`
	Position     *int32     `json:"position"`

	Language string `json:"language,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

type SubmissionReset struct {
	ID            uuid.UUID
	PreviousState State
}

type SubmissionCreatedEvent struct {
	SubmissionEventMeta

	Id     uuid.UUID `json:"id"`
	State  State     `json:"state"`
	Source string    `json:"source"`
}

type SubmissionQueuedEvent struct {
	SubmissionEventMeta

	Id uuid.UUID `json:"id"`
}

type SubmissionCompilingStartedEvent struct {
	SubmissionEventMeta

	Id uuid.UUID `json:"id"`
}

type SubmissionTestingStartedEvent struct {
	SubmissionEventMeta

	Id uuid.UUID `json:"id"`
}

type SubmissionTestStartedEvent struct {
	SubmissionEventMeta

	Id     uuid.UUID `json:"id"`
	Number int32     `json:"number"`
}

type SubmissionChange struct {
	Revision      int64
	PreviousState State
}

type SubmissionCompletedEvent struct {
	SubmissionEventMeta

	Id         uuid.UUID    `json:"id"`
	State      State        `json:"state"`
	Score      points.Value `json:"score"`
	Penalty    int32        `json:"penalty"`
	TimeStat   int32        `json:"time_stat"`
	MemoryStat int32        `json:"memory_stat"`
	FailedTest *int32       `json:"failed_test,omitempty"`
	// FailedTestFeedback is the feedback policy of the failed test's group, which may hide the number.
	FailedTestFeedback string `json:"failed_test_feedback,omitempty"`
	Pretests           bool   `json:"pretests,omitempty"`
}

func (s Submission) EventMeta() SubmissionEventMeta {
	return SubmissionEventMeta{Revision: s.Revision, Kind: s.Kind, UpdatedAt: s.UpdatedAt, UserId: s.CreatedBy, Username: s.Username, ContestId: s.ContestID, ContestTitle: s.ContestTitle, ProblemId: s.ProblemID, ProblemTitle: s.ProblemTitle, Position: s.Position, Language: s.Language, CreatedAt: s.CreatedAt}
}

func (s Submission) CompletedEvent() SubmissionCompletedEvent {
	e := SubmissionCompletedEvent{Id: s.ID, State: s.State, Score: s.Score, Penalty: s.Penalty, TimeStat: s.TimeStat, MemoryStat: s.MemoryStat, FailedTest: s.FailedTest, Pretests: s.Pretests, SubmissionEventMeta: s.EventMeta()}
	if s.FailedTest != nil {
		e.FailedTestFeedback = s.FailedFeedback()
	}
	return e
}

// FailedFeedback is the feedback policy of the failed test's group; empty means complete.
func (s Submission) FailedFeedback() string {
	if s.TestDetails == nil || s.FailedTest == nil {
		return s.FailedTestFeedback
	}
	return s.TestDetails.Feedback(*s.FailedTest)
}

// Feedback finds the feedback policy of the group holding a test; empty means complete.
func (d *SubmissionTestDetails) Feedback(test int32) string {
	if d == nil {
		return ""
	}
	for _, t := range d.Tests {
		if t.TestIndex == test {
			for _, g := range d.Groups {
				if g.Name == t.Group {
					return g.Feedback
				}
			}
		}
	}
	return ""
}

// FeedbackShowsFailure reports whether a feedback policy shows which of its tests failed.
func FeedbackShowsFailure(policy string) bool {
	return policy == "" || policy == "complete" || policy == "icpc"
}

func (s Submission) CreatedEvent() SubmissionCreatedEvent {
	return SubmissionCreatedEvent{SubmissionEventMeta: s.EventMeta(), Id: s.ID, State: s.State, Source: s.Submission}
}
