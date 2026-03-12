package sandbox

import (
	"context"
	"fmt"
)

// Orchestrator coordinates compilation and execution workflows
type Orchestrator struct {
	compiler *Compiler
	executor *Executor
	client   *Client
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(client *Client) *Orchestrator {
	compiler := NewCompiler(client)
	executor := NewExecutor(client, compiler)

	return &Orchestrator{
		compiler: compiler,
		executor: executor,
		client:   client,
	}
}

// ValidateTest compiles validator (if needed) and validates test input
func (o *Orchestrator) ValidateTest(ctx context.Context, req ValidateTestRequest) (*ValidationResult, error) {
	// If validator FileID is not provided, return error
	if req.ValidatorFileID == "" {
		return &ValidationResult{
			Valid: false,
			Error: "validator binary not provided",
		}, nil
	}

	// Run validator
	validatorReq := ValidatorRunRequest{
		BinaryFileID: req.ValidatorFileID,
		Input:        req.Input,
		Limits:       req.Limits,
	}

	result, err := o.executor.RunValidator(ctx, validatorReq)
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Error: fmt.Sprintf("validator execution failed: %v", err),
		}, nil
	}

	return &ValidationResult{
		Valid:   result.Valid,
		Message: result.Message,
		Error:   result.Error,
	}, nil
}

// GenerateTest compiles generator and generates test data
func (o *Orchestrator) GenerateTest(ctx context.Context, req GenerateTestRequest) (*GeneratedTest, error) {
	// If generator FileID is not provided, return error
	if req.GeneratorFileID == "" {
		return &GeneratedTest{
			Success: false,
			Error:   "generator binary not provided",
		}, nil
	}

	// Run generator
	generatorReq := GeneratorRunRequest{
		BinaryFileID: req.GeneratorFileID,
		Arguments:    req.Arguments,
		Seed:         req.Seed,
		Limits:       req.Limits,
	}

	result, err := o.executor.RunGenerator(ctx, generatorReq)
	if err != nil {
		return &GeneratedTest{
			Success: false,
			Error:   fmt.Sprintf("generator execution failed: %v", err),
		}, nil
	}

	if !result.Success {
		return &GeneratedTest{
			Success: false,
			Error:   result.Error,
		}, nil
	}

	return &GeneratedTest{
		Input:   result.Output,
		Success: true,
	}, nil
}

// JudgeSolution compiles solution, runs on input, and checks with checker
func (o *Orchestrator) JudgeSolution(ctx context.Context, req JudgeSolutionRequest) (*JudgeResult, error) {
	// Step 1: Compile the solution
	compileLimits := o.compiler.GetCompileLimits(req.SolutionLanguage)
	compileLimits.CPUTimeMs = 30000 // 30 seconds for compilation
	compileLimits.MemoryMB = 512

	binary, err := o.compiler.CompileSolution(ctx, req.SolutionCode, req.SolutionLanguage, compileLimits)
	if err != nil {
		return &JudgeResult{
			Verdict:      "CE",
			Message:      "Compilation failed",
			CompileError: fmt.Sprintf("compilation error: %v", err),
		}, nil
	}

	if !binary.Success {
		return &JudgeResult{
			Verdict:      "CE",
			Message:      "Compilation failed",
			CompileError: binary.CompileLog,
		}, nil
	}

	// Step 2: Run the solution
	runLimits := ResourceLimits{
		CPUTimeMs: req.TimeLimitMs,
		MemoryMB:  req.MemoryLimitMB,
		ProcLimit: 1,
		StackMB:   256,
	}

	var solutionResult *SolutionResult

	// Handle compiled vs interpreted languages
	if binary.FileID != "" {
		// Compiled language
		solutionResult, err = o.executor.RunSolution(ctx, SolutionRunRequest{
			BinaryFileID: binary.FileID,
			Input:        req.Input,
			Limits:       runLimits,
		})
	} else {
		// Interpreted language - need special handling
		solutionResult, err = o.executor.RunSolutionWithSource(ctx, req.SolutionCode, req.SolutionLanguage, req.Input, runLimits)
	}

	if err != nil {
		return &JudgeResult{
			Verdict:        "IE",
			Message:        "Internal error during execution",
			ExecutionError: fmt.Sprintf("execution error: %v", err),
		}, nil
	}

	// Check for execution errors
	if !solutionResult.Success {
		verdict := "RE"
		if solutionResult.Status == "Time Limit Exceeded" {
			verdict = "TLE"
		} else if solutionResult.Status == "Memory Limit Exceeded" {
			verdict = "MLE"
		}

		return &JudgeResult{
			Verdict:        verdict,
			Message:        solutionResult.Error,
			Time:           solutionResult.Time,
			Memory:         solutionResult.Memory,
			ExecutionError: string(solutionResult.Stderr),
		}, nil
	}

	// Step 3: Run the checker
	if req.CheckerFileID == "" {
		// No checker - do simple text comparison
		if string(solutionResult.Output) == string(req.Answer) {
			return &JudgeResult{
				Verdict: "OK",
				Message: "Output matches expected answer",
				Time:    solutionResult.Time,
				Memory:  solutionResult.Memory,
			}, nil
		} else {
			return &JudgeResult{
				Verdict: "WA",
				Message: "Output does not match expected answer",
				Time:    solutionResult.Time,
				Memory:  solutionResult.Memory,
			}, nil
		}
	}

	// Run checker
	checkerLimits := ResourceLimits{
		CPUTimeMs: 10000, // 10 seconds for checker
		MemoryMB:  256,
		ProcLimit: 1,
		StackMB:   256,
	}

	checkerResult, err := o.executor.RunChecker(ctx, CheckerRunRequest{
		BinaryFileID: req.CheckerFileID,
		Input:        req.Input,
		Output:       solutionResult.Output,
		Answer:       req.Answer,
		Limits:       checkerLimits,
	})

	if err != nil {
		return &JudgeResult{
			Verdict:        "IE",
			Message:        "Checker execution failed",
			Time:           solutionResult.Time,
			Memory:         solutionResult.Memory,
			ExecutionError: fmt.Sprintf("checker error: %v", err),
		}, nil
	}

	if !checkerResult.Success {
		return &JudgeResult{
			Verdict:        "IE",
			Message:        "Checker failed",
			Time:           solutionResult.Time,
			Memory:         solutionResult.Memory,
			ExecutionError: checkerResult.Error,
		}, nil
	}

	// Return final result
	return &JudgeResult{
		Verdict: checkerResult.Verdict,
		Score:   checkerResult.Score,
		Message: checkerResult.Message,
		Time:    solutionResult.Time,
		Memory:  solutionResult.Memory,
	}, nil
}

// CompileAndCacheComponent compiles a component and returns its binary info
func (o *Orchestrator) CompileAndCacheComponent(ctx context.Context, req ComponentCompileRequest) (*ComponentBinary, error) {
	return o.compiler.CompileComponent(ctx, req)
}

// TestSolutionOnMultipleTests runs a solution on multiple test cases
func (o *Orchestrator) TestSolutionOnMultipleTests(ctx context.Context, solutionCode, solutionLanguage, checkerFileID string, tests []TestCase, timeLimitMs, memoryLimitMB int64) ([]JudgeResult, error) {
	results := make([]JudgeResult, len(tests))

	for i, test := range tests {
		req := JudgeSolutionRequest{
			SolutionCode:     solutionCode,
			SolutionLanguage: solutionLanguage,
			CheckerFileID:    checkerFileID,
			Input:            test.Input,
			Answer:           test.Answer,
			TimeLimitMs:      timeLimitMs,
			MemoryLimitMB:    memoryLimitMB,
		}

		result, err := o.JudgeSolution(ctx, req)
		if err != nil {
			results[i] = JudgeResult{
				Verdict:        "IE",
				Message:        fmt.Sprintf("Failed to judge test %02d: %v", i+1, err),
				ExecutionError: err.Error(),
			}
			continue
		}

		results[i] = *result
	}

	return results, nil
}

// TestCase represents a single test case
type TestCase struct {
	Input  []byte
	Answer []byte
}

// CompileComponentFromSource is a convenience method that compiles from source code directly
func (o *Orchestrator) CompileComponentFromSource(ctx context.Context, componentType, sourceCode, language string, dependencies map[string]string) (*ComponentBinary, error) {
	req := ComponentCompileRequest{
		Type:         componentType,
		SourceCode:   sourceCode,
		Language:     language,
		Dependencies: dependencies,
	}

	return o.compiler.CompileComponent(ctx, req)
}
