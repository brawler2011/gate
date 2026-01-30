package models

import (
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
)

// UpdateFileRequest represents a request to update a file
type UpdateFileRequest struct {
	ProblemID uuid.UUID
	UserID    uuid.UUID
	Path      string
	Content   []byte
}

// CompileComponentRequest represents a request to compile a component
type CompileComponentRequest struct {
	ProblemID     uuid.UUID
	ComponentType string // "checker", "validator", "generator", "interactor"
	UserID        uuid.UUID
}

// CompileResult represents the result of a compilation
type CompileResult struct {
	Success      bool
	FileID       string
	SHA256       string
	CompileLog   string
	CompileError string
}

// GenerateTestsRequest represents a request to generate tests
type GenerateTestsRequest struct {
	ProblemID     uuid.UUID
	GeneratorName string
	TestNumbers   []int
	Arguments     [][]string // args for each test
	UserID        uuid.UUID
}

// ValidationReport contains validation results for tests
type ValidationReport struct {
	TotalTests   int
	ValidTests   int
	InvalidTests int
	Results      []TestValidationResult
}

// TestValidationResult represents validation result for a single test
type TestValidationResult struct {
	TestNumber int
	Valid      bool
	Message    string
	Error      string
}

// TestSolutionRequest represents a request to test a solution
type TestSolutionRequest struct {
	ProblemID    uuid.UUID
	SolutionPath string
	TestNumbers  []int // nil = all tests
	UserID       uuid.UUID
}

// TestReport contains test results
type TestReport struct {
	TotalTests  int
	PassedTests int
	FailedTests int
	Results     []TestResult
}

// TestResult represents result for a single test
type TestResult struct {
	TestNumber int
	Verdict    string
	Time       int64
	Memory     int64
	Message    string
}

// WorkshopStatus represents the current status of a workshop
type WorkshopStatus struct {
	CurrentSHA            string
	ModifiedFiles         []vcs.FileStatus
	HasUncommittedChanges bool
}
