package problemsmodel

import (
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	"github.com/brawler2011/gate/backend/pkg/points"

	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/google/uuid"
)

type ProblemManifest struct {
	// Revision identifies the file package read by the caller; it is not persisted metadata.
	Revision    string    `json:"-"`
	LastUpdated time.Time `json:"last_updated"`
	ProblemType string    `json:"problem_type"`

	MaxScore      *points.Value  `json:"max_score"` // nil for pass-fail problems
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
	Filename string `json:"filename"`
	Version  string `json:"version"`
}

type FileMetadata struct {
	Type         string       `json:"type"`
	Filename     string       `json:"filename"`
	Compiler     string       `json:"compiler"`
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
	PointsPolicy string       `json:"points_policy"`
	DependsOn    []int        `json:"depends_on"`
	Tests        [2]int       `json:"tests"` // range [start, end]
}

type TestCase struct {
	Filename  string  `json:"filename"` // input name relative to tests/; empty for generated tests
	Ordinal   int     `json:"ordinal"`
	Method    string  `json:"method"`
	Generator *string `json:"generator"` // generator name followed by arguments
	IsSample  bool    `json:"is_sample"`
}

type Media struct {
	Images []Image `json:"images"`
}

type Image struct {
	Filename string `json:"filename"`
}

const (
	ProblemVisibilityPrivate = "private"
	ProblemVisibilityPublic  = "public"
)

type Problem struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	OrganizationLogin string
	Visibility        string
	Title             string
	ShortName         string
	TimeLimitMs       int
	MemoryLimitMb     int
	IsTemplate        bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ProblemMember struct {
	ProblemID uuid.UUID
	UserID    uuid.UUID
	Role      accessmodel.ProblemRole
	Username  string
	Email     string
	CreatedAt time.Time
}

type ProblemPackage struct {
	ID             uuid.UUID
	ProblemID      uuid.UUID
	OrganizationID uuid.UUID
	BuildID        uuid.UUID
	ObjectID       uuid.UUID
	Revision       int64
	PackageHash    string
	CreatedAt      time.Time
	Title, Type    string
	Statements     []DraftStatement
	TimeLimitMs    int32
	MemoryLimitMb  int32
	InputFile      string
	OutputFile     string
}

type CreateProblemInput struct {
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
}

type ProblemsFilter struct {
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
	IsTemplate *bool
}

type CreateProblemMemberParams struct {
	ProblemID uuid.UUID
	UserID    uuid.UUID
	Role      accessmodel.ProblemRole
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

type PackageStatement struct {
	ProblemID  uuid.UUID
	Title      string
	Statements []DraftStatement
	Samples    []Sample
	// Assets maps each statement asset name to its content SHA-256.
	Assets map[string]string
}

type StatementContent struct {
	Statement *Statement
	Samples   []Sample
	Assets    map[string]string
}

type PackageAsset struct {
	Name string
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
