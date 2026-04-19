package sandbox

import "time"

// StatusAccepted is the go-judge status string for successful execution.
const StatusAccepted = "Accepted"

// ResourceLimits defines execution resource constraints
type ResourceLimits struct {
	CPUTimeMs int64 // CPU time limit in milliseconds
	MemoryMB  int64 // Memory limit in megabytes
	ProcLimit int   // Maximum number of processes
	StackMB   int64 // Stack size limit in megabytes
}

// ToNanoseconds converts CPU time from ms to ns for go-judge
func (r ResourceLimits) ToNanoseconds() int64 {
	return r.CPUTimeMs * 1_000_000
}

// ToBytes converts memory from MB to bytes for go-judge
func (r ResourceLimits) ToBytes() int64 {
	return r.MemoryMB * 1024 * 1024
}

// ClientConfig configures the sandbox client
type ClientConfig struct {
	Addr    string // gRPC address
	Timeout time.Duration
}

// CompileRequest represents a compilation request
type CompileRequest struct {
	SourceCode   string
	Language     string
	SourceFile   string            // e.g., "solution.cpp"
	OutputFile   string            // e.g., "solution"
	Dependencies map[string]string // filename -> content
	Limits       ResourceLimits
}

// CompileResult represents a compilation result
type CompileResult struct {
	Success    bool
	FileID     string // go-judge cached file ID
	Stdout     string
	Stderr     string
	ExitStatus int
	Time       int64 // in nanoseconds
	Memory     int64 // in bytes
}

// ExecuteRequest represents an execution request
type ExecuteRequest struct {
	BinaryFileID   string // cached binary from compilation
	ExecutableName string // e.g., "./solution"
	Args           []string
	Stdin          []byte
	Files          map[string][]byte // additional input files
	Limits         ResourceLimits
}

// ExecuteResult represents an execution result
type ExecuteResult struct {
	Status     string // "Accepted", "Time Limit Exceeded", etc.
	ExitStatus int
	Stdout     []byte
	Stderr     []byte
	Time       int64 // in nanoseconds
	Memory     int64 // in bytes
}

// InteractiveExecutionCommand describes one process in interactive execution.
type InteractiveExecutionCommand struct {
	BinaryFileID   string // cached binary from compilation
	ExecutableName string // e.g., "./solution"
	Args           []string
	Env            []string
	Files          map[string][]byte // additional input files
	Limits         ResourceLimits
}

// PipeEndpoint references one command file descriptor in a multi-command request.
type PipeEndpoint struct {
	CommandIndex int32
	FD           int32
}

// PipeMapping describes one pipe between two command descriptors.
type PipeMapping struct {
	In              PipeEndpoint
	Out             PipeEndpoint
	Proxy           bool
	Name            string
	MaxBytes        uint64
	DisableZeroCopy bool
}

// InteractiveExecutionRequest describes two-process interactive execution.
type InteractiveExecutionRequest struct {
	Solution    InteractiveExecutionCommand
	Interactor  InteractiveExecutionCommand
	PipeMapping []PipeMapping
}

// InteractiveExecutionResult contains per-process results.
type InteractiveExecutionResult struct {
	Solution   ExecuteResult
	Interactor ExecuteResult
}

// ComponentCompileRequest represents a problem component compilation request
type ComponentCompileRequest struct {
	Type         string // "checker", "validator", "generator", "interactor"
	SourceCode   string
	Language     string            // from FileMetadata.Compiler
	Dependencies map[string]string // filename -> content (e.g., "testlib.h")
}

// ComponentBinary represents a compiled component
type ComponentBinary struct {
	FileID       string // go-judge cached file ID
	BinarySHA256 string // for manifest.json
	CompileLog   string
	Success      bool
	Error        string
}

// CheckerRunRequest represents a checker execution request
type CheckerRunRequest struct {
	BinaryFileID string // from ComponentBinary
	Input        []byte // test input
	Output       []byte // participant output
	Answer       []byte // correct answer
	Limits       ResourceLimits
}

// CheckerResult represents a checker execution result
type CheckerResult struct {
	Verdict string   // "OK", "WA", "PE", etc.
	Score   *float64 // optional score for partial scoring
	Message string   // checker message
	Time    int64    // in nanoseconds
	Memory  int64    // in bytes
	Success bool
	Error   string
}

// ValidatorRunRequest represents a validator execution request
type ValidatorRunRequest struct {
	BinaryFileID string // from ComponentBinary
	Input        []byte // test input to validate
	Limits       ResourceLimits
}

// ValidatorResult represents a validator execution result
type ValidatorResult struct {
	Valid   bool
	Message string
	Time    int64 // in nanoseconds
	Memory  int64 // in bytes
	Error   string
}

// GeneratorRunRequest represents a generator execution request
type GeneratorRunRequest struct {
	BinaryFileID string   // from ComponentBinary
	Arguments    []string // from TestCase.Generator ("gen_border 1 2 3")
	Seed         int64
	Limits       ResourceLimits
}

// GeneratorResult represents a generator execution result
type GeneratorResult struct {
	Output  []byte
	Success bool
	Error   string
	Time    int64 // in nanoseconds
	Memory  int64 // in bytes
}

// InteractorRunRequest represents an interactor execution request
type InteractorRunRequest struct {
	BinaryFileID       string // compiled interactor binary
	SolutionFileID     string // compiled solution binary
	SolutionSourceCode string // used for interpreted solutions
	SolutionLanguage   string
	Input              []byte // test input
	Answer             []byte // expected output/answer file contents
	SolutionLimits     ResourceLimits
	InteractorLimits   ResourceLimits
}

// InteractorResult represents an interactor execution result
type InteractorResult struct {
	Verdict string   // "OK", "WA", "PE", etc.
	Score   *float64 // optional score
	Message string
	Time    int64 // in nanoseconds
	Memory  int64 // in bytes
	Success bool
	Error   string
}

// SolutionRunRequest represents a solution execution request
type SolutionRunRequest struct {
	BinaryFileID string // from ComponentBinary or compiled solution
	Input        []byte // test input
	Limits       ResourceLimits
}

// SolutionResult represents a solution execution result
type SolutionResult struct {
	Output     []byte
	Stderr     []byte
	ExitStatus int
	Status     string // "Accepted", "Time Limit Exceeded", "Runtime Error", etc.
	Time       int64  // in nanoseconds
	Memory     int64  // in bytes
	Success    bool
	Error      string
}

// ValidateTestRequest represents a test validation request
type ValidateTestRequest struct {
	ValidatorFileID string // compiled validator
	Input           []byte
	Limits          ResourceLimits
}

// ValidationResult represents a test validation result
type ValidationResult struct {
	Valid   bool
	Message string
	Error   string
}

// GenerateTestRequest represents a test generation request
type GenerateTestRequest struct {
	GeneratorFileID string // compiled generator
	Arguments       []string
	Seed            int64
	Limits          ResourceLimits
}

// GeneratedTest represents a generated test
type GeneratedTest struct {
	Input   []byte
	Error   string
	Success bool
}

// JudgeSolutionRequest represents a complete solution judging request
type JudgeSolutionRequest struct {
	SolutionCode     string
	SolutionLanguage string
	CheckerFileID    string // pre-compiled checker
	InteractorFileID string // pre-compiled interactor
	Input            []byte
	Answer           []byte
	TimeLimitMs      int64
	MemoryLimitMB    int64
}

// JudgeResult represents a complete judging result
type JudgeResult struct {
	Verdict        string   // "OK", "WA", "TLE", "MLE", "RE", "CE"
	Score          *float64 // optional score
	Message        string   // checker message
	Time           int64    // solution time in nanoseconds
	Memory         int64    // solution memory in bytes
	CompileError   string   // compilation error if any
	ExecutionError string   // execution error if any
}
