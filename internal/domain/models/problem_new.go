package models

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// НОВАЯ СХЕМА PROBLEMS (SeaweedFS + JSONB)
// ============================================================================
//
// Старые модели остаются в problem.go для обратной совместимости
// Новые модели используют суффикс V2 или без суффикса (где не конфликтует)

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
	ProblemVisibilityPrivateV2  = "private"
	ProblemVisibilityPublicV2   = "public"
	ProblemVisibilityUnlistedV2 = "unlisted"
)

// ProblemV2 - новая модель задачи (метаданные в БД, контент в SeaweedFS)
type ProblemV2 struct {
	ID         uuid.UUID         `json:"id" db:"id"`
	OwnerID    *uuid.UUID        `json:"owner_id" db:"owner_id"`
	Visibility string            `json:"visibility" db:"visibility"`
	Titles     map[string]string `json:"titles" db:"titles"` // {"en": "Sum", "ru": "Сумма"}
	ShortName  string            `json:"short_name" db:"short_name"`
	CreatedAt  time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at" db:"updated_at"`
}

type CreateProblemV2Input struct {
	Titles     map[string]string // {"en": "Sum", "ru": "Сумма"}
	ShortName  string
	Visibility string
	OwnerID    uuid.UUID
}

type CreateProblemV2Params struct {
	ID         uuid.UUID
	Titles     map[string]string
	ShortName  string
	Visibility string
	OwnerID    uuid.UUID
}

type UpdateProblemV2Input struct {
	Titles     *map[string]string
	Visibility *string
}

type ProblemsV2Filter struct {
	Page       int32
	PageSize   int32
	OwnerID    *uuid.UUID
	Search     *string
	Descending bool
}

func (f ProblemsV2Filter) Offset() int32 {
	return (f.Page - 1) * f.PageSize
}

type ProblemV2List struct {
	Problems   []ProblemV2 `json:"problems"`
	Pagination Pagination  `json:"pagination"`
}

// ContestProblem - связь контеста и задачи с зафиксированной версией
type ContestProblem struct {
	ContestID   uuid.UUID `json:"contest_id" db:"contest_id"`
	ProblemID   uuid.UUID `json:"problem_id" db:"problem_id"`
	Ordinal     int       `json:"ordinal" db:"ordinal"`
	PackageHash string    `json:"package_hash" db:"package_hash"` // hash версии из SeaweedFS
}
