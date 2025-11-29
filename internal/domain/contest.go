package domain

import (
	"time"

	"github.com/google/uuid"
)

type Contest struct {
	ID                     uuid.UUID `json:"id"`
	Title                  string    `json:"title"`
	Description            string    `json:"description"`
	Visibility             string    `json:"visibility"`
	MonitorScope           string    `json:"monitor_scope"`
	SubmissionsListScope   string    `json:"submissions_list_scope"`
	SubmissionsReviewScope string    `json:"submissions_review_scope"`
	CreatedBy              uuid.UUID `json:"created_by"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type ContestsList struct {
	Contests   []Contest  `json:"contests"`
	Pagination Pagination `json:"pagination"`
}

type ContestProblem struct {
	ProblemID        uuid.UUID `json:"problem_id"`
	Title            string    `json:"title"`
	TimeLimit        int64     `json:"time_limit"`
	MemoryLimit      int64     `json:"memory_limit"`
	Position         int64     `json:"position"`
	LegendHtml       string    `json:"legend_html"`
	InputFormatHtml  string    `json:"input_format_html"`
	OutputFormatHtml string    `json:"output_format_html"`
	NotesHtml        string    `json:"notes_html"`
	ScoringHtml      string    `json:"scoring_html"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ContestMember struct {
	UserID      uuid.UUID `json:"user_id"`
	ContestID   uuid.UUID `json:"contest_id"`
	Username    string    `json:"username"`
	Role        string    `json:"role"`
	ContestRole string    `json:"contest_role"`
	KratosID    string    `json:"kratos_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ContestMembersList struct {
	Members    []ContestMember `json:"members"`
	Pagination Pagination      `json:"pagination"`
}

