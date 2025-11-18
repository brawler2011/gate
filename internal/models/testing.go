package models

import (
	"time"

	"github.com/google/uuid"
)

type TestGroup struct {
	Id        uuid.UUID `json:"id"`
	ProblemId uuid.UUID `json:"problem_id"`
	Name      string    `json:"name"`
	Ordinal   int64     `json:"ordinal"`
	Points    int64     `json:"points"`
	IsSample  bool      `json:"is_sample"`
}

type TestResult struct {
	Id           uuid.UUID `json:"id"`
	SubmissionId uuid.UUID `json:"submission_id"`
	TesterId     uuid.UUID `json:"tester_id"`

	Verdict int64 `json:"verdict"`

	TimeMs   int64 `json:"time_ms"`
	MemoryKb int64 `json:"memory_kb"`

	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	CompileOutput string `json:"compile_output"`

	CreatedAt time.Time `json:"created_at"`
}

type ProblemTest struct {
	Id           uuid.UUID `json:"id"`
	ProblemId    uuid.UUID `json:"problem_id"`
	GroupId      uuid.UUID `json:"group_id"`
	Ordinal      int64     `json:"ordinal"`
	InputObject  string    `json:"input_object"`
	OutputObject string    `json:"output_object"`
	Checksum     string    `json:"checksum"`
}
