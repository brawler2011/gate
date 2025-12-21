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

func (f SubmissionsFilter) Offset() int32 {
	return (f.Page - 1) * f.PageSize
}

type WebSocketMessageType string

const (
	MessageTypeSubmissionCreated WebSocketMessageType = "submission_created"
	MessageTypeSubmissionUpdated WebSocketMessageType = "submission_updated"

	MessageTypeTestingStarted   WebSocketMessageType = "testing_started"
	MessageTypeTestCompleted    WebSocketMessageType = "test_completed"
	MessageTypeTestingCompleted WebSocketMessageType = "testing_completed"
)

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

type SubmissionWebSocketEvent struct {
	MessageType WebSocketMessageType `json:"message_type"`
	Submission  *SubmissionListItem  `json:"submission,omitempty"`
	Message     string               `json:"message,omitempty"`

	SubmissionID *uuid.UUID `json:"submission_id,omitempty"`
	TestNumber   int        `json:"test_number,omitempty"`
	TotalTests   int        `json:"total_tests,omitempty"`
	Passed       *bool      `json:"passed,omitempty"`
	State        *State     `json:"state,omitempty"`

	ContestID         *uuid.UUID `json:"contest_id,omitempty"`
	UserID            *uuid.UUID `json:"user_id,omitempty"`
	ProblemID         *uuid.UUID `json:"problem_id,omitempty"`
	Language          *int32     `json:"language,omitempty"`
	ContestVisibility string     `json:"contest_visibility,omitempty"`
}

type WebSocketFilter struct {
	ContestID *uuid.UUID
	UserID    *uuid.UUID
	ProblemID *uuid.UUID
	State     *State
	Language  *LanguageName
}

func (f WebSocketFilter) MatchesSubmission(submission *SubmissionListItem) bool {
	if f.ContestID != nil && *f.ContestID != submission.ContestID {
		return false
	}
	if f.UserID != nil && *f.UserID != submission.UserID {
		return false
	}
	if f.ProblemID != nil && *f.ProblemID != submission.ProblemID {
		return false
	}
	if f.State != nil && int32(*f.State) != int32(submission.State) {
		return false
	}
	if f.Language != nil && int32(*f.Language) != submission.Language {
		return false
	}
	return true
}

func (f WebSocketFilter) MatchesEvent(event *SubmissionWebSocketEvent) bool {
	if event.Submission != nil {
		return f.MatchesSubmission(event.Submission)
	}

	if event.ContestVisibility == "private" {
		if f.ContestID == nil {
			return false
		}
		if event.ContestID != nil && *f.ContestID != *event.ContestID {
			return false
		}
	}

	if f.ContestID != nil && event.ContestID != nil && *f.ContestID != *event.ContestID {
		return false
	}
	if f.UserID != nil && event.UserID != nil && *f.UserID != *event.UserID {
		return false
	}
	if f.ProblemID != nil && event.ProblemID != nil && *f.ProblemID != *event.ProblemID {
		return false
	}
	if f.Language != nil && event.Language != nil && int32(*f.Language) != *event.Language {
		return false
	}

	return true
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
