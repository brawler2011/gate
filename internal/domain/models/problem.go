package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ProblemVisibilityPublic  = "public"
	ProblemVisibilityPrivate = "private"
)

type ProblemsFilter struct {
	Page       int32
	PageSize   int32
	OwnerId    *uuid.UUID
	Search     *string
	Descending bool
}

func (f ProblemsFilter) Offset() int32 {
	return (f.Page - 1) * f.PageSize
}

type ProblemUpdate struct {
	Id uuid.UUID `db:"id"`

	Title       *string `db:"title"`
	MemoryLimit *int32  `db:"memory_limit"`
	TimeLimit   *int32  `db:"time_limit"`
	Visibility  *string `db:"visibility"`

	Legend       *string `db:"legend"`
	InputFormat  *string `db:"input_format"`
	OutputFormat *string `db:"output_format"`
	Notes        *string `db:"notes"`
	Scoring      *string `db:"scoring"`

	LegendHtml       *string `db:"legend_html"`
	InputFormatHtml  *string `db:"input_format_html"`
	OutputFormatHtml *string `db:"output_format_html"`
	NotesHtml        *string `db:"notes_html"`
	ScoringHtml      *string `db:"scoring_html"`
}

type ProblemStatement struct {
	Legend       string `db:"legend"`
	InputFormat  string `db:"input_format"`
	OutputFormat string `db:"output_format"`
	Notes        string `db:"notes"`
	Scoring      string `db:"scoring"`
}

type Html5ProblemStatement struct {
	LegendHtml       string `db:"legend_html"`
	InputFormatHtml  string `db:"input_format_html"`
	OutputFormatHtml string `db:"output_format_html"`
	NotesHtml        string `db:"notes_html"`
	ScoringHtml      string `db:"scoring_html"`
}

type ProblemSample struct {
	Id        uuid.UUID `db:"id"`
	ProblemId uuid.UUID `db:"problem_id"`
	Ordinal   int32     `db:"ordinal"`
	Input     string    `db:"input"`
	Output    string    `db:"output"`
	CreatedAt time.Time `db:"created_at"`
}

type ProblemSamples []ProblemSample

type ProblemTests []ProblemTest

type ProblemPermissions struct {
	ViewProblem  bool
	EditProblem  bool
	AdminProblem bool
}

const (
	ProblemRoleOwner     = "owner"
	ProblemRoleModerator = "moderator"
)

type ProblemMember struct {
	ProblemId uuid.UUID `db:"problem_id"`
	UserId    uuid.UUID `db:"user_id"`
	Role      string    `db:"role"`
}

func (m *ProblemMember) IsOwner() bool {
	return m.Role == ProblemRoleOwner
}

func (m *ProblemMember) IsModerator() bool {
	return m.Role == ProblemRoleModerator
}

type ProblemPermissionGet struct {
	ProblemId uuid.UUID
	UserId    uuid.UUID
}

type CreateProblemInput struct {
	Title  string
	UserId uuid.UUID
}

type CreateProblemParams struct {
	Id     uuid.UUID
	Title  string
	UserId uuid.UUID
}

type CreateProblemMemberParams struct {
	ProblemId uuid.UUID
	UserId    uuid.UUID
	Role      string
}

type Problem struct {
	ID               uuid.UUID `json:"id"`
	CreatedBy        uuid.UUID `json:"created_by"`
	Visibility       string    `json:"visibility"`
	Title            string    `json:"title"`
	TimeLimit        int32     `json:"time_limit"`
	MemoryLimit      int32     `json:"memory_limit"`
	Legend           string    `json:"legend"`
	InputFormat      string    `json:"input_format"`
	OutputFormat     string    `json:"output_format"`
	Notes            string    `json:"notes"`
	Scoring          string    `json:"scoring"`
	LegendHtml       string    `json:"legend_html"`
	InputFormatHtml  string    `json:"input_format_html"`
	OutputFormatHtml string    `json:"output_format_html"`
	NotesHtml        string    `json:"notes_html"`
	ScoringHtml      string    `json:"scoring_html"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ProblemsList struct {
	Problems   []Problem  `json:"problems"`
	Pagination Pagination `json:"pagination"`
}

type ProblemTest struct {
	ID        uuid.UUID `json:"id"`
	ProblemID uuid.UUID `json:"problem_id"`
	Ordinal   int32     `json:"ordinal"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	CreatedAt time.Time `json:"created_at"`
}
