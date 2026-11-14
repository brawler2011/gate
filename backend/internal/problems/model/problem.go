package problemsmodel

import (
	"github.com/brawler2011/gate/backend/pkg/points"
	"time"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type ProblemManifest struct {
	// Revision identifies the file package read by the caller; it is not persisted metadata.
	Revision    string    `json:"-"`
	LastUpdated time.Time `json:"last_updated"`
	ProblemType string    `json:"problem_type"` // "pass-fail", "scoring", etc

	MaxScore      *points.Value  `json:"max_score"` // null для pass-fail задач
	FilesMetadata []FileMetadata `json:"meta_files"`

	TimeLimitMs   int `json:"time_limit_ms"`
	MemoryLimitMb int `json:"memory_limit_mb"`

	Statement Statement `json:"statement"`
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

type TestsMetadata struct {
	Groups []TestGroup `json:"groups"`
	Tests  []TestCase  `json:"tests"`
}

type TestGroup struct {
	Ordinal      int          `json:"ordinal"`
	Name         string       `json:"name"`
	Points       points.Value `json:"points"`
	PointsPolicy string       `json:"points_policy"` // "complete-group", "each-test"
	DependsOn    []int        `json:"depends_on"`    // номера групп или null
	Tests        [2]int       `json:"tests"`         // диапазон [start, end]
}

type TestCase struct {
	Filename  string  `json:"filename"` // input name relative to tests/; empty for generated tests
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
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	OrganizationLogin string
	OwnerID           *uuid.UUID
	Visibility        string
	Title             string
	ShortName         string
	TimeLimitMs       int
	MemoryLimitMb     int
	IsTemplate        bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
	BuildID        uuid.UUID
	ObjectID       uuid.UUID
	Revision       int64
	PackageHash    string
	CreatedAt      time.Time
}

type CreateProblemInput struct {
	IdempotencyKey string
	OrganizationID uuid.UUID
	Title          string
	TemplateID     string
}

type CreateProblemParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Title          string
	ShortName      string
	Visibility     string
	OwnerID        *uuid.UUID
}

type ProblemsFilter struct {
	// ViewerID restricts rows to read access; nil is reserved for trusted internal lists.
	ViewerID       *uuid.UUID
	Page           int32
	PageSize       int32
	OrganizationID *uuid.UUID
	OwnerID        *uuid.UUID
	Search         string
	Visibility     string
	IsTemplate     *bool
	Descending     *bool
}

type ProblemsList struct {
	Problems   []Problem         `json:"problems"`
	Pagination paging.Pagination `json:"pagination"`
}

type ProblemUpdate struct {
	Visibility *string
	OwnerID    *uuid.UUID
	IsTemplate *bool
}

type CreateProblemMemberParams struct {
	ProblemID uuid.UUID
	UserID    uuid.UUID
	Role      ProblemRole
}

type DashboardProblem struct {
	ID                uuid.UUID
	Title             string
	OrganizationID    uuid.UUID
	OrganizationName  string
	OrganizationLogin string
	TimeLimitMs       int
	MemoryLimitMb     int
	UpdatedAt         time.Time
}

type Sample struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// PackageStatement is the statement-facing content of a built package. It is
// stored at publication, so reading it never opens the package archive.
type PackageStatement struct {
	ProblemID uuid.UUID
	Draft     Draft
	Samples   []Sample
	// Assets maps each statement asset path to its content SHA-256.
	Assets map[string]string
}

// StatementContent is a package statement in one language with its samples
// and statement asset hashes by path.
type StatementContent struct {
	Statement *Statement
	Samples   []Sample
	Assets    map[string]string
}

// PackageAsset is a statement asset stored as its own object.
type PackageAsset struct {
	Path string
	Key  string
}
type ProblemDetails struct {
	Problem   Problem
	Statement *Statement
	Samples   []Sample
}

type LocalizedStatement struct {
	Statement Statement
	Languages []string
	Language  string
}
