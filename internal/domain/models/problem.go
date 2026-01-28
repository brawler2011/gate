package models

import (
	"time"

	"github.com/google/uuid"
)

// ProblemManifest соответствует формату manifest.json в корне папки задачи
type ProblemManifest struct {
	LastUpdated time.Time `json:"last_updated"`
	ProblemType string    `json:"problem_type"` // "pass-fail", "scoring", etc

	MaxScore      *int           `json:"max_score"` // null для pass-fail задач
	FilesMetadata []FileMetadata `json:"meta_files"`

	TimeLimitMs     int `json:"time_limit_ms"`
	MemoryLimitMb   int `json:"memory_limit_mb"`
	StdoutLimitMb   int `json:"stdout_limit_mb"`
	CodeSizeLimitKb int `json:"code_size_limit_kb"`

	// {"en": {}, "ru": {}}
	Statements map[string]Statement `json:"statements"`
}

type Statement struct {
	Title string `json:"title"`

	Legend       string `json:"legend"`
	InputFormat  string `json:"input_format"`
	OutputFormat string `json:"output_format"`
	Notes        string `json:"notes"`
	Interaction  string `json:"interaction"`
	Scoring      string `json:"scoring"`
}

type Dependency struct {
	Filename string `json:"filename"` // "testlib.h"
	Version  string `json:"version"`  // "0.9.41"
}

type FileMetadata struct {
	Type         string       `json:"type"` // "checker", "validator", "generator", "interactor"
	Filename     string       `json:"filename"`
	Compiler     string       `json:"compiler"` // "cpp17", "python3", etc
	BinarySha256 *string      `json:"binary_sha256"`
	Dependencies []Dependency `json:"dependencies"`
}

// TestsMetadata соответствует формату tests/tests.json
type TestsMetadata struct {
	Groups []TestGroup `json:"groups"`
	Tests  []TestCase  `json:"tests"`
}

type TestGroup struct {
	Ordinal      int    `json:"ordinal"`
	Name         string `json:"name"`
	Points       int    `json:"points"`
	PointsPolicy string `json:"points_policy"` // "complete-group", "each-test"
	DependsOn    []int  `json:"depends_on"`    // номера групп или null
	Tests        [2]int `json:"tests"`         // диапазон [start, end]
}

type TestCase struct {
	Ordinal   int     `json:"ordinal"`
	Method    string  `json:"method"`    // "manual" или "generated"
	Generator *string `json:"generator"` // "gen_border 1 2 3" или null
	IsSample  bool    `json:"is_sample"`
}

// ============================================================================
// МОДЕЛИ БД (новая схема problems)
// ============================================================================

const (
	ProblemVisibilityPrivate = "private"
	ProblemVisibilityPublic  = "public"
)

// Problem - новая модель задачи (метаданные в БД, контент в SeaweedFS)
type Problem struct {
	ID         uuid.UUID
	OwnerID    *uuid.UUID
	Visibility string
	Titles     map[string]string
	ShortName  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CreateProblemInput struct {
	Titles     map[string]string // {"en": "Sum", "ru": "Сумма"}
	ShortName  string
	Visibility string
	OwnerID    uuid.UUID
}

type CreateProblemParams struct {
	ID         uuid.UUID
	Titles     map[string]string
	ShortName  string
	Visibility string
	OwnerID    uuid.UUID
}

type UpdateProblemInput struct {
	Titles     *map[string]string
	Visibility *string
}

type ProblemsFilter struct {
	Page       int32
	PageSize   int32
	OwnerID    *uuid.UUID
	Search     *string
	Descending bool
}

type ProblemList struct {
	Problems   []Problem  `json:"problems"`
	Pagination Pagination `json:"pagination"`
}
