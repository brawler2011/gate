package sandbox

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Executor handles execution of problem components and solutions
type Executor struct {
	client   *Client
	compiler *Compiler
}

// NewExecutor creates a new executor
func NewExecutor(client *Client, compiler *Compiler) *Executor {
	return &Executor{
		client:   client,
		compiler: compiler,
	}
}

// RunChecker executes a checker with test input, participant output, and answer
func (e *Executor) RunChecker(ctx context.Context, req CheckerRunRequest) (*CheckerResult, error) {
	// Checkers typically use the 3-file protocol:
	// stdin: test input
	// argv[1]: file with participant output
	// argv[2]: file with answer

	// For testlib-based checkers, we need to provide files via copyIn
	files := map[string][]byte{
		"output.txt": req.Output,
		"answer.txt": req.Answer,
	}

	execReq := ExecuteRequest{
		BinaryFileID:   req.BinaryFileID,
		ExecutableName: "./checker",
		Args:           []string{"input.txt", "output.txt", "answer.txt"},
		Stdin:          req.Input,
		Files:          files,
		Limits:         req.Limits,
	}

	// Also add input.txt to files
	execReq.Files["input.txt"] = req.Input

	result, err := e.client.Execute(ctx, execReq)
	if err != nil {
		return &CheckerResult{
			Success: false,
			Error:   fmt.Sprintf("execution failed: %v", err),
		}, nil
	}

	// Parse checker output
	// Testlib checkers typically output verdict on first line
	checkerResult := &CheckerResult{
		Time:    result.Time,
		Memory:  result.Memory,
		Success: true,
	}

	// Parse checker output from stdout
	output := strings.TrimSpace(string(result.Stdout))
	lines := strings.Split(output, "\n")

	if len(lines) > 0 {
		// First line typically contains the verdict
		firstLine := strings.ToUpper(strings.TrimSpace(lines[0]))

		// Determine verdict from exit status and output
		if result.Status == "Accepted" && result.ExitStatus == 0 {
			checkerResult.Verdict = "OK"
		} else if strings.Contains(firstLine, "WRONG") || result.ExitStatus == 1 {
			checkerResult.Verdict = "WA"
		} else if strings.Contains(firstLine, "PRESENTATION") || result.ExitStatus == 2 {
			checkerResult.Verdict = "PE"
		} else if result.ExitStatus == 3 {
			checkerResult.Verdict = "FAIL" // Checker fail
		} else if result.ExitStatus == 7 {
			checkerResult.Verdict = "POINTS" // Partial points
			// Try to extract score
			if score := extractScore(output); score != nil {
				checkerResult.Score = score
			}
		} else {
			checkerResult.Verdict = "FAIL"
		}

		// Message is the rest of the output
		if len(lines) > 1 {
			checkerResult.Message = strings.Join(lines[1:], "\n")
		} else if len(lines) == 1 && len(lines[0]) > 0 {
			checkerResult.Message = lines[0]
		}
	}

	// If checker failed to run properly
	if result.Status != "Accepted" && result.Status != "Nonzero Exit Status" {
		checkerResult.Success = false
		checkerResult.Error = fmt.Sprintf("checker execution error: %s", result.Status)
	}

	return checkerResult, nil
}

// RunValidator validates test input
func (e *Executor) RunValidator(ctx context.Context, req ValidatorRunRequest) (*ValidatorResult, error) {
	// Validators read from stdin and return 0 if valid, non-zero if invalid
	execReq := ExecuteRequest{
		BinaryFileID:   req.BinaryFileID,
		ExecutableName: "./validator",
		Args:           []string{},
		Stdin:          req.Input,
		Files:          make(map[string][]byte),
		Limits:         req.Limits,
	}

	result, err := e.client.Execute(ctx, execReq)
	if err != nil {
		return &ValidatorResult{
			Valid:  false,
			Error:  fmt.Sprintf("execution failed: %v", err),
			Time:   0,
			Memory: 0,
		}, nil
	}

	validatorResult := &ValidatorResult{
		Time:   result.Time,
		Memory: result.Memory,
	}

	// Validator is valid if exit status is 0 and execution was successful
	if result.Status == "Accepted" && result.ExitStatus == 0 {
		validatorResult.Valid = true
		validatorResult.Message = string(result.Stdout)
	} else {
		validatorResult.Valid = false
		validatorResult.Message = string(result.Stderr)
		if result.Status != "Accepted" {
			validatorResult.Error = fmt.Sprintf("validator execution error: %s", result.Status)
		}
	}

	return validatorResult, nil
}

// RunGenerator generates test data
func (e *Executor) RunGenerator(ctx context.Context, req GeneratorRunRequest) (*GeneratorResult, error) {
	// Generators output to stdout
	execReq := ExecuteRequest{
		BinaryFileID:   req.BinaryFileID,
		ExecutableName: "./generator",
		Args:           req.Arguments,
		Stdin:          []byte{}, // Generators typically don't use stdin
		Files:          make(map[string][]byte),
		Limits:         req.Limits,
	}

	result, err := e.client.Execute(ctx, execReq)
	if err != nil {
		return &GeneratorResult{
			Success: false,
			Error:   fmt.Sprintf("execution failed: %v", err),
		}, nil
	}

	generatorResult := &GeneratorResult{
		Time:   result.Time,
		Memory: result.Memory,
	}

	if result.Status == "Accepted" && result.ExitStatus == 0 {
		generatorResult.Success = true
		generatorResult.Output = result.Stdout
	} else {
		generatorResult.Success = false
		generatorResult.Error = fmt.Sprintf("generator failed: status=%s, exit=%d, stderr=%s",
			result.Status, result.ExitStatus, string(result.Stderr))
	}

	return generatorResult, nil
}

// RunInteractor handles interactive problems
func (e *Executor) RunInteractor(ctx context.Context, req InteractorRunRequest) (*InteractorResult, error) {
	// Interactive problems are complex and require process communication
	// This is a simplified implementation
	// In production, we'd need to set up pipes between solution and interactor

	// For now, return not implemented
	return &InteractorResult{
		Success: false,
		Error:   "interactor execution not yet implemented",
	}, nil
}

// RunSolution executes a user solution
func (e *Executor) RunSolution(ctx context.Context, req SolutionRunRequest) (*SolutionResult, error) {
	execReq := ExecuteRequest{
		BinaryFileID:   req.BinaryFileID,
		ExecutableName: "./solution",
		Args:           []string{},
		Stdin:          req.Input,
		Files:          make(map[string][]byte),
		Limits:         req.Limits,
	}

	result, err := e.client.Execute(ctx, execReq)
	if err != nil {
		return &SolutionResult{
			Success: false,
			Error:   fmt.Sprintf("execution failed: %v", err),
		}, nil
	}

	solutionResult := &SolutionResult{
		Output:     result.Stdout,
		Stderr:     result.Stderr,
		ExitStatus: result.ExitStatus,
		Status:     result.Status,
		Time:       result.Time,
		Memory:     result.Memory,
	}

	// Determine success based on execution status
	if result.Status == "Accepted" {
		solutionResult.Success = true
	} else {
		solutionResult.Success = false
		solutionResult.Error = result.Status
	}

	return solutionResult, nil
}

// RunSolutionWithSource compiles and runs a solution from source code
func (e *Executor) RunSolutionWithSource(ctx context.Context, sourceCode, language string, input []byte, limits ResourceLimits) (*SolutionResult, error) {
	// Compile the solution
	binary, err := e.compiler.CompileSolution(ctx, sourceCode, language, ResourceLimits{})
	if err != nil {
		return &SolutionResult{
			Success: false,
			Error:   fmt.Sprintf("compilation failed: %v", err),
			Status:  "Compilation Error",
		}, nil
	}

	if !binary.Success {
		return &SolutionResult{
			Success: false,
			Error:   binary.Error,
			Status:  "Compilation Error",
			Stderr:  []byte(binary.CompileLog),
		}, nil
	}

	// For interpreted languages, we need to run the source directly
	if binary.FileID == "" {
		// Handle interpreted languages
		return e.runInterpretedSolution(ctx, sourceCode, language, input, limits)
	}

	// Run the compiled solution
	return e.RunSolution(ctx, SolutionRunRequest{
		BinaryFileID: binary.FileID,
		Input:        input,
		Limits:       limits,
	})
}

// runInterpretedSolution runs interpreted language solutions (Python, etc.) by
// copying the source file into the sandbox and invoking the interpreter directly.
func (e *Executor) runInterpretedSolution(ctx context.Context, sourceCode, language string, input []byte, limits ResourceLimits) (*SolutionResult, error) {
	normalizedLang := NormalizeLanguageName(language)
	langConfig, ok := GetLanguageConfig(normalizedLang)
	if !ok {
		return &SolutionResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported language: %s", language),
			Status:  "Internal Error",
		}, nil
	}

	sourceFile := "solution" + langConfig.Extension
	executeCmd := e.compiler.GetExecuteCommand(normalizedLang, "solution")
	if len(executeCmd) == 0 {
		return &SolutionResult{
			Success: false,
			Error:   fmt.Sprintf("no execute command configured for language: %s", language),
			Status:  "Internal Error",
		}, nil
	}

	execReq := ExecuteRequest{
		BinaryFileID:   "",
		ExecutableName: executeCmd[0],
		Args:           executeCmd[1:],
		Stdin:          input,
		Files:          map[string][]byte{sourceFile: []byte(sourceCode)},
		Limits:         limits,
	}

	result, err := e.client.Execute(ctx, execReq)
	if err != nil {
		return &SolutionResult{
			Success: false,
			Error:   fmt.Sprintf("execution failed: %v", err),
		}, nil
	}

	solutionResult := &SolutionResult{
		Output:     result.Stdout,
		Stderr:     result.Stderr,
		ExitStatus: result.ExitStatus,
		Status:     result.Status,
		Time:       result.Time,
		Memory:     result.Memory,
		Success:    result.Status == StatusAccepted,
	}
	if !solutionResult.Success {
		solutionResult.Error = result.Status
	}
	return solutionResult, nil
}

// Helper function to extract score from checker output
func extractScore(output string) *float64 {
	// Look for patterns like "points: 0.5" or "score: 50"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "points:") || strings.Contains(lower, "score:") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if (strings.Contains(strings.ToLower(part), "points") ||
					strings.Contains(strings.ToLower(part), "score")) && i+1 < len(parts) {
					if score, err := strconv.ParseFloat(parts[i+1], 64); err == nil {
						return &score
					}
				}
			}
		}
	}
	return nil
}
