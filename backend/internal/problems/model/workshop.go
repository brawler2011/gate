package problemsmodel

import (
	"github.com/google/uuid"
)

type UpdateFileRequest struct {
	// Revision is set for read/patch/write operations, such as statement PATCH.
	Revision  string
	ProblemID uuid.UUID
	Path      string
	Content   []byte
}

type CompileComponentRequest struct {
	ProblemID     uuid.UUID
	ComponentType string
}

type CompileResult struct {
	Success      bool
	FileID       string
	SHA256       string
	CompileLog   string
	CompileError string
}

type GenerateTestsRequest struct {
	ProblemID     uuid.UUID
	GeneratorName string
	TestNumbers   []int
	Arguments     [][]string // args for each test
}

type ValidationReport struct {
	TotalTests   int
	ValidTests   int
	InvalidTests int
	Results      []TestValidationResult
}

type TestValidationResult struct {
	TestNumber int
	Valid      bool
	Message    string
	Error      string
}

type TestSolutionRequest struct {
	ProblemID    uuid.UUID
	SolutionPath string
	TestNumbers  []int // nil = all tests
}

type TestReport struct {
	TotalTests  int
	PassedTests int
	FailedTests int
	Results     []TestResult
}

type TestResult struct {
	TestNumber int
	Verdict    string
	Time       int64
	Memory     int64
	Message    string
}
