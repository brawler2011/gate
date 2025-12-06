package domain

import (
	"time"

	"github.com/google/uuid"
)

type Problem struct {
	ID               uuid.UUID `json:"id"`
	CreatedBy        uuid.UUID `json:"created_by"`
	Visibility       string    `json:"visibility"`
	Title            string    `json:"title"`
	TimeLimit        int64     `json:"time_limit"`
	MemoryLimit      int64     `json:"memory_limit"`
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

type ProblemMember struct {
	ProblemID uuid.UUID `json:"problem_id"`
	UserID    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"`
}

type ProblemTest struct {
	ID        uuid.UUID  `json:"id"`
	ProblemID uuid.UUID  `json:"problem_id"`
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	Ordinal   int64      `json:"ordinal"`
	Input     string     `json:"input"`
	Output    string     `json:"output"`
	CreatedAt time.Time  `json:"created_at"`
}

type TestGroup struct {
	ID        uuid.UUID `json:"id"`
	ProblemID uuid.UUID `json:"problem_id"`
	Ordinal   int64     `json:"ordinal"`
	Name      string    `json:"name"`
	Points    int64     `json:"points"`
	IsSample  bool      `json:"is_sample"`
	CreatedAt time.Time `json:"created_at"`
}

type ProblemSample struct {
	ID        uuid.UUID `json:"id"`
	ProblemID uuid.UUID `json:"problem_id"`
	Ordinal   int64     `json:"ordinal"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	CreatedAt time.Time `json:"created_at"`
}

