package domain

import (
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
)

type Submission struct {
	ID           uuid.UUID           `json:"id"`
	CreatedBy    uuid.UUID           `json:"created_by"`
	Username     string              `json:"username"`
	Submission   string              `json:"submission"`
	State        models.State        `json:"state"`
	Score        int64               `json:"score"`
	Penalty      int64               `json:"penalty"`
	TimeStat     int64               `json:"time_stat"`
	MemoryStat   int64               `json:"memory_stat"`
	Language     models.LanguageName `json:"language"`
	ProblemID    uuid.UUID           `json:"problem_id"`
	ProblemTitle string              `json:"problem_title"`
	Position     int64               `json:"position"`
	ContestID    uuid.UUID           `json:"contest_id"`
	ContestTitle string              `json:"contest_title"`
	UpdatedAt    time.Time           `json:"updated_at"`
	CreatedAt    time.Time           `json:"created_at"`
}

type SubmissionsList struct {
	Submissions []Submission `json:"submissions"`
	Pagination  Pagination   `json:"pagination"`
}

