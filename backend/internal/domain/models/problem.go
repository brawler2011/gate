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

type Media struct {
	Images []Image `json:"images"`
}

type Image struct {
	Filename string `json:"filename"`
}

// ============================================================================
// МОДЕЛИ БД (новая схема problems)
// ============================================================================

const (
	ProblemVisibilityPrivate  = "private"
	ProblemVisibilityPublic   = "public"
	ProblemVisibilityUnlisted = "unlisted"
)

// ProblemRole defines roles for direct problem access
type ProblemRole string

const (
	ProblemRoleOwner     ProblemRole = "owner"
	ProblemRoleModerator ProblemRole = "moderator"
	ProblemRoleViewer    ProblemRole = "viewer"
)

// Problem - новая модель задачи (метаданные в БД, контент в SeaweedFS)
type Problem struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	OwnerID        *uuid.UUID
	Visibility     string
	Titles         map[string]string // {"en": "Sum", "ru": "Сумма"}
	ShortName      string
	GitCommitHash  *string
	TimeLimitMs    int
	MemoryLimitMb  int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ProblemMember represents a user's direct membership to a problem
type ProblemMember struct {
	ProblemID uuid.UUID
	UserID    uuid.UUID
	Role      ProblemRole
	Username  string
	Email     string
	CreatedAt time.Time
}

// ProblemPackage represents a compiled, immutable version of a problem
type ProblemPackage struct {
	ID             uuid.UUID
	ProblemID      uuid.UUID
	OrganizationID uuid.UUID
	GitCommitHash  string
	PackageHash    string
	URL            *string
	Status         string // "pending", "building", "ready", "failed"
	BuildLog       *string
	CreatedAt      time.Time
	CompiledAt     *time.Time
}

type CreateProblemInput struct {
	OrganizationID uuid.UUID
	Titles         map[string]string // {"en": "Sum", "ru": "Сумма"}
	ShortName      string
	Visibility     string
	OwnerID        *uuid.UUID
}

type CreateProblemParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Titles         map[string]string
	ShortName      string
	Visibility     string
	OwnerID        *uuid.UUID
}

type UpdateProblemInput struct {
	Titles     *map[string]string
	Visibility *string
	OwnerID    *uuid.UUID
}

type ProblemsFilter struct {
	Page           int32
	PageSize       int32
	OrganizationID *uuid.UUID
	OwnerID        *uuid.UUID
	Search         string
	Visibility     string
}

type ProblemsList struct {
	Problems   []Problem  `json:"problems"`
	Pagination Pagination `json:"pagination"`
}

type ProblemUpdate struct {
	Titles        *map[string]string
	Visibility    *string
	OwnerID       *uuid.UUID
	GitCommitHash *string
}

type CreateProblemMemberParams struct {
	ProblemID uuid.UUID
	UserID    uuid.UUID
	Role      ProblemRole
}

type ProblemTest struct {
	ID        uuid.UUID
	ProblemID uuid.UUID
	Ordinal   int32
	Input     string
	Output    string
	IsSample  bool
}

type ProblemTests []ProblemTest
