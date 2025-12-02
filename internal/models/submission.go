package models

import (
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type LanguageName int64

const (
	Golang LanguageName = 10
	Cpp    LanguageName = 20
	Python LanguageName = 30
)

func (n LanguageName) Valid() error {
	switch n {
	case Golang, Cpp, Python:
		return nil
	default:
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid language")
	}
}

type State int64

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
	Score      int64
	TimeStat   int64
	MemoryStat int64
	FailedTest *int
}

type SubmissionCreation struct {
	Solution  string
	ProblemId uuid.UUID
	ContestId uuid.UUID
	UserId    uuid.UUID
	Language  LanguageName
	Penalty   int64
}

type SolutionsFilter struct {
	Page      int64
	PageSize  int64
	ContestId *uuid.UUID
	UserId    *uuid.UUID
	ProblemId *uuid.UUID
	Language  *LanguageName
	State     *State
	Order     *int64
}

func (f SolutionsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

// WebSocket message types for submission list updates
type WebSocketMessageType string

const (
	MessageTypeSubmissionCreated WebSocketMessageType = "submission_created"
	MessageTypeSubmissionUpdated WebSocketMessageType = "submission_updated"
	// Test progress message types - sent while submission is being tested
	MessageTypeTestingStarted   WebSocketMessageType = "testing_started"
	MessageTypeTestCompleted    WebSocketMessageType = "test_completed"
	MessageTypeTestingCompleted WebSocketMessageType = "testing_completed"
)

// SubmissionListItem represents a submission in the list format (matches SubmissionsListItemModel)
type SubmissionListItem struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Username     string    `json:"username"`
	State        State     `json:"state"`
	Score        int64     `json:"score"`
	Penalty      int64     `json:"penalty"`
	TimeStat     int64     `json:"time_stat"`
	MemoryStat   int64     `json:"memory_stat"`
	Language     int64     `json:"language"`
	ProblemID    uuid.UUID `json:"problem_id"`
	ProblemTitle string    `json:"problem_title"`
	Position     int64     `json:"position"`
	ContestID    uuid.UUID `json:"contest_id"`
	ContestTitle string    `json:"contest_title"`
	FailedTest   *int      `json:"failed_test,omitempty"`
	UpdatedAt    string    `json:"updated_at"`
	CreatedAt    string    `json:"created_at"`
	// ContestVisibility is included for permission filtering
	// "public" contests are visible to everyone, "private" requires membership
	ContestVisibility string `json:"contest_visibility,omitempty"`
}

// SubmissionWebSocketEvent is the event sent to WebSocket clients
type SubmissionWebSocketEvent struct {
	MessageType WebSocketMessageType `json:"message_type"`
	Submission  *SubmissionListItem  `json:"submission,omitempty"`
	Message     string               `json:"message,omitempty"`
	// Test progress fields (for testing_started, test_completed, testing_completed events)
	SubmissionID *uuid.UUID `json:"submission_id,omitempty"`
	TestNumber   int        `json:"test_number,omitempty"`
	TotalTests   int        `json:"total_tests,omitempty"`
	Passed       *bool      `json:"passed,omitempty"`
	State        *State     `json:"state,omitempty"`
	// Filter metadata for matching (included in progress events)
	ContestID         *uuid.UUID `json:"contest_id,omitempty"`
	UserID            *uuid.UUID `json:"user_id,omitempty"`
	ProblemID         *uuid.UUID `json:"problem_id,omitempty"`
	Language          *int64     `json:"language,omitempty"`
	ContestVisibility string     `json:"contest_visibility,omitempty"`
}

// WebSocketFilter represents the filter parameters for WebSocket subscriptions
type WebSocketFilter struct {
	ContestID *uuid.UUID
	UserID    *uuid.UUID
	ProblemID *uuid.UUID
	State     *State
	Language  *LanguageName
}

// MatchesSubmission checks if a submission matches the filter criteria
// Permission logic:
// - If contest is "public", event is visible to all matching filters
// - If contest is "private", event is only visible if the client has a contestId filter
//   that matches (meaning they have access to that specific contest)
func (f WebSocketFilter) MatchesSubmission(submission *SubmissionListItem) bool {
	// Permission check based on contest visibility
	// Private contests require explicit contestId filter to prove access
	if submission.ContestVisibility == "private" {
		// If no contestId filter specified, user shouldn't see private contest submissions
		if f.ContestID == nil {
			return false
		}
		// If contestId filter doesn't match, skip (standard filter behavior)
		if *f.ContestID != submission.ContestID {
			return false
		}
	}

	// Standard filter matching
	if f.ContestID != nil && *f.ContestID != submission.ContestID {
		return false
	}
	if f.UserID != nil && *f.UserID != submission.UserID {
		return false
	}
	if f.ProblemID != nil && *f.ProblemID != submission.ProblemID {
		return false
	}
	if f.State != nil && int64(*f.State) != int64(submission.State) {
		return false
	}
	if f.Language != nil && int64(*f.Language) != submission.Language {
		return false
	}
	return true
}

// MatchesEvent checks if a WebSocket event matches the filter criteria
// Used for test progress events that don't have full submission data
func (f WebSocketFilter) MatchesEvent(event *SubmissionWebSocketEvent) bool {
	// For submission events, use the submission data
	if event.Submission != nil {
		return f.MatchesSubmission(event.Submission)
	}

	// For progress events, use the event metadata
	// Permission check based on contest visibility
	if event.ContestVisibility == "private" {
		if f.ContestID == nil {
			return false
		}
		if event.ContestID != nil && *f.ContestID != *event.ContestID {
			return false
		}
	}

	// Standard filter matching using event metadata
	if f.ContestID != nil && event.ContestID != nil && *f.ContestID != *event.ContestID {
		return false
	}
	if f.UserID != nil && event.UserID != nil && *f.UserID != *event.UserID {
		return false
	}
	if f.ProblemID != nil && event.ProblemID != nil && *f.ProblemID != *event.ProblemID {
		return false
	}
	if f.Language != nil && event.Language != nil && int64(*f.Language) != *event.Language {
		return false
	}
	// Note: We don't filter by state for progress events since the state is changing
	return true
}

func (u *LanguageName) String() string {
	if u == nil {
		return "nil"
	}
	return "LanguageName"
}

func (s *State) String() string {
	if s == nil {
		return "nil"
	}
	return "State"
}