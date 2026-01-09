package models

import (
	"time"

	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type LanguageName = int32

const (
	Golang LanguageName = 10
	Cpp    LanguageName = 20
	Python LanguageName = 30
)

func LanguageNameValid(n LanguageName) error {
	switch n {
	case Golang, Cpp, Python:
		return nil
	default:
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid language")
	}
}

type State = int32

const (
	Saved State = 1 // saved to db

	GotCE State = 101 // compilation error
	GotTL State = 102 // time limit exceeded
	GotML State = 103 // memory limit exceeded
	GotRE State = 104 // runtime error
	GotPE State = 105 // presentation error
	GotWA State = 106 // wrong answer

	Accepted State = 200 // accepted
)

type SubmissionUpdate struct {
	State      State
	Score      int32
	TimeStat   int32
	MemoryStat int32
}

type SubmissionCreation struct {
	Solution  string
	ProblemId uuid.UUID
	ContestId uuid.UUID
	UserId    uuid.UUID
	Language  LanguageName
	Penalty   int32
}

type SubmissionsFilter struct {
	Page      int32
	PageSize  int32
	ContestId *uuid.UUID
	UserId    *uuid.UUID
	ProblemId *uuid.UUID
	Language  *LanguageName
	State     *State
	Order     *int32
}

type SubmissionListItem struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Username     string    `json:"username"`
	State        State     `json:"state"`
	Score        int32     `json:"score"`
	Penalty      int32     `json:"penalty"`
	TimeStat     int32     `json:"time_stat"`
	MemoryStat   int32     `json:"memory_stat"`
	Language     int32     `json:"language"`
	ProblemID    uuid.UUID `json:"problem_id"`
	ProblemTitle string    `json:"problem_title"`
	Position     int32     `json:"position"`
	ContestID    uuid.UUID `json:"contest_id"`
	ContestTitle string    `json:"contest_title"`
	UpdatedAt    string    `json:"updated_at"`
	CreatedAt    string    `json:"created_at"`
}

type Submission struct {
	ID           uuid.UUID    `json:"id"`
	CreatedBy    *uuid.UUID   `json:"created_by"`
	Username     string       `json:"username"`
	Submission   string       `json:"submission"`
	State        State        `json:"state"`
	Score        int32        `json:"score"`
	Penalty      int32        `json:"penalty"`
	TimeStat     int32        `json:"time_stat"`
	MemoryStat   int32        `json:"memory_stat"`
	Language     LanguageName `json:"language"`
	ProblemID    *uuid.UUID   `json:"problem_id"`
	ProblemTitle string       `json:"problem_title"`
	Position     *int32       `json:"position"`
	ContestID    *uuid.UUID   `json:"contest_id"`
	ContestTitle string       `json:"contest_title"`
	UpdatedAt    time.Time    `json:"updated_at"`
	CreatedAt    time.Time    `json:"created_at"`
}

type SubmissionsList struct {
	Submissions []Submission `json:"submissions"`
	Pagination  Pagination   `json:"pagination"`
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
	UserId   *uuid.UUID `json:"user_id,omitempty"`
	Username string     `json:"username"`

	ContestId    *uuid.UUID `json:"contest_id,omitempty"`
	ContestTitle string     `json:"contest_title"`

	ProblemId    *uuid.UUID `json:"problem_id,omitempty"`
	ProblemTitle string     `json:"problem_title"`
	Position     *int32     `json:"position"`

	Language LanguageName `json:"language,omitempty"`

	CreatedAt time.Time `json:"created_at"`
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

type SubmissionCompletedEvent struct {
	SubmissionEventMeta

	Id         uuid.UUID `json:"id"`
	State      State     `json:"state"`
	Score      int32     `json:"score"`
	Penalty    int32     `json:"penalty"`
	TimeStat   int32     `json:"time_stat"`
	MemoryStat int32     `json:"memory_stat"`
}
