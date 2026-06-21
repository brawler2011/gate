package judge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

type JudgingStrategy interface {
	Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error)
}

type FlatTest struct {
	SubtaskName string
	TestIndex   int
	Test        gfmt.Test
}

type StandardStrategy struct {
	sandbox            *sandbox.Sandbox
	eventPublisher     *EventPublisher
	pkg                *gfmt.GateFormat
	compiledComponents map[string]sandbox.Executable
}

func NewStandardStrategy(
	sandbox *sandbox.Sandbox,
	eventPublisher *EventPublisher,
	pkg *gfmt.GateFormat,
	compiledComponents map[string]sandbox.Executable,
) *StandardStrategy {
	return &StandardStrategy{
		sandbox:            sandbox,
		eventPublisher:     eventPublisher,
		pkg:                pkg,
		compiledComponents: compiledComponents,
	}
}

func (s *StandardStrategy) Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error) {
	languageStr := convertLanguage(language)

	// Compile user solution
	solExec, err := s.sandbox.Compile(ctx, []byte(sourceCode), languageStr, nil)
	if err != nil {
		return &FinalVerdict{
			State:     models.GotCE,
			Score:     0,
			MaxTime:   0,
			MaxMemory: 0,
			Message:   fmt.Sprintf("Compilation failed: %v", err),
		}, nil
	}

	checkerExec, hasChecker := s.compiledComponents["checker"]

	// Collect tests
	flatTests := collectFlatTests(s.pkg.Problem)

	var results []TestResult

	if err := s.eventPublisher.PublishTestingStarted(ctx, submissionID, meta); err != nil {
		return nil, fmt.Errorf("failed to publish testing started: %w", err)
	}

	for _, tc := range flatTests {
		if err := s.eventPublisher.PublishTestStarted(ctx, submissionID, int32(tc.TestIndex), meta); err != nil {
			return nil, fmt.Errorf("failed to publish test started: %w", err)
		}

		input, err := s.getTestInput(ctx, tc.Test)
		if err != nil {
			return nil, fmt.Errorf("failed to get input for test %d: %w", tc.TestIndex, err)
		}

		answer, err := s.getTestAnswer(ctx, tc.Test, input)
		if err != nil {
			return nil, fmt.Errorf("failed to get answer for test %d: %w", tc.TestIndex, err)
		}

		// Run solution
		runRes, err := s.sandbox.Test(ctx, solExec, languageStr, input, s.pkg.Problem.Limits.TimeMs, s.pkg.Problem.Limits.MemoryMb)
		if err != nil {
			return nil, fmt.Errorf("failed to run solution for test %d: %w", tc.TestIndex, err)
		}

		verdict := string(runRes.Status)
		message := string(runRes.Stderr)
		var checkerScore *float64

		if runRes.Status == sandbox.StatusOK {
			if hasChecker {
				chkRes, err := s.sandbox.Check(ctx, checkerExec, input, runRes.Stdout, answer)
				if err != nil {
					verdict = "IE"
					message = fmt.Sprintf("Checker execution failed: %v", err)
				} else {
					verdict = string(chkRes.Status)
					message = chkRes.Message
					checkerScore = chkRes.Score
				}
			} else {
				if string(runRes.Stdout) == string(answer) || strings.TrimSpace(string(runRes.Stdout)) == strings.TrimSpace(string(answer)) {
					verdict = "OK"
					message = "Answer is correct"
				} else {
					verdict = "WA"
					message = "Output does not match expected answer"
				}
			}
		}

		res := TestResult{
			TestNumber: tc.TestIndex,
			Verdict:    verdict,
			Score:      checkerScore,
			Time:       runRes.Time.Nanoseconds(),
			Memory:     runRes.Memory,
			Message:    message,
		}
		results = append(results, res)

		if verdict != "OK" && verdict != "AC" && verdict != "Accepted" {
			break
		}
	}

	calculator := NewVerdictCalculator(s.pkg.Problem.Type, s.pkg.Problem)
	return calculator.Calculate(results), nil
}

func (s *StandardStrategy) getTestInput(ctx context.Context, test gfmt.Test) ([]byte, error) {
	if test.Manual != "" {
		return s.pkg.GetTestInput(test.Manual)
	}

	if test.Generate != "" {
		parts := strings.Fields(test.Generate)
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty generate command")
		}
		genName := parts[0]
		genArgs := parts[1:]

		genExec, exists := s.compiledComponents["generator_"+genName]
		if !exists {
			dir := filepath.Join(s.pkg.Path, "generators")
			entries, err := os.ReadDir(dir)
			if err != nil {
				return nil, err
			}
			var filename string
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasPrefix(entry.Name(), genName) {
					filename = entry.Name()
					break
				}
			}
			if filename == "" {
				return nil, fmt.Errorf("generator source not found for %s", genName)
			}
			data, err := os.ReadFile(filepath.Join(dir, filename))
			if err != nil {
				return nil, err
			}
			deps, _ := loadLibDependencies(s.pkg.Path)
			lang := detectLanguage(filepath.Ext(filename))
			compiled, err := s.sandbox.Compile(ctx, data, lang, deps)
			if err != nil {
				return nil, err
			}
			genExec = compiled
			s.compiledComponents["generator_"+genName] = genExec
		}

		return s.sandbox.Generate(ctx, genExec, genArgs)
	}

	return nil, fmt.Errorf("invalid test case")
}

func (s *StandardStrategy) getTestAnswer(ctx context.Context, test gfmt.Test, input []byte) ([]byte, error) {
	if test.Manual != "" {
		ansFile := strings.TrimSuffix(test.Manual, ".in") + ".out"
		data, err := s.pkg.GetTestOutput(ansFile)
		if err != nil {
			ansFile = strings.TrimSuffix(test.Manual, ".in") + ".ans"
			data, err = s.pkg.GetTestOutput(ansFile)
		}
		return data, err
	}

	var okSol string
	for solFile, tag := range s.pkg.Problem.Solutions {
		if tag == "OK" || tag == "Accepted" || tag == "main" {
			okSol = solFile
			break
		}
	}
	if okSol == "" {
		return nil, fmt.Errorf("no correct solution found to generate answer")
	}

	solExec, exists := s.compiledComponents["correct_sol"]
	if !exists {
		data, err := os.ReadFile(filepath.Join(s.pkg.Path, "solutions", okSol))
		if err != nil {
			return nil, err
		}
		lang := detectLanguage(filepath.Ext(okSol))
		compiled, err := s.sandbox.Compile(ctx, data, lang, nil)
		if err != nil {
			return nil, err
		}
		solExec = compiled
		s.compiledComponents["correct_sol"] = solExec
	}

	runRes, err := s.sandbox.Test(ctx, solExec, detectLanguage(filepath.Ext(okSol)), input, s.pkg.Problem.Limits.TimeMs, s.pkg.Problem.Limits.MemoryMb)
	if err != nil {
		return nil, err
	}
	if runRes.Status != sandbox.StatusOK {
		return nil, fmt.Errorf("correct solution failed to run: %s", runRes.Status)
	}
	return runRes.Stdout, nil
}

type ScoringStrategy struct {
	sandbox            *sandbox.Sandbox
	eventPublisher     *EventPublisher
	pkg                *gfmt.GateFormat
	compiledComponents map[string]sandbox.Executable
}

func NewScoringStrategy(
	sandbox *sandbox.Sandbox,
	eventPublisher *EventPublisher,
	pkg *gfmt.GateFormat,
	compiledComponents map[string]sandbox.Executable,
) *ScoringStrategy {
	return &ScoringStrategy{
		sandbox:            sandbox,
		eventPublisher:     eventPublisher,
		pkg:                pkg,
		compiledComponents: compiledComponents,
	}
}

func (s *ScoringStrategy) Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error) {
	languageStr := convertLanguage(language)

	solExec, err := s.sandbox.Compile(ctx, []byte(sourceCode), languageStr, nil)
	if err != nil {
		return &FinalVerdict{
			State:     models.GotCE,
			Score:     0,
			MaxTime:   0,
			MaxMemory: 0,
			Message:   fmt.Sprintf("Compilation failed: %v", err),
		}, nil
	}

	checkerExec, hasChecker := s.compiledComponents["checker"]
	if !hasChecker {
		return nil, fmt.Errorf("checker is required for scoring strategy")
	}

	flatTests := collectFlatTests(s.pkg.Problem)
	var results []TestResult

	if err := s.eventPublisher.PublishTestingStarted(ctx, submissionID, meta); err != nil {
		return nil, fmt.Errorf("failed to publish testing started: %w", err)
	}

	for _, tc := range flatTests {
		if err := s.eventPublisher.PublishTestStarted(ctx, submissionID, int32(tc.TestIndex), meta); err != nil {
			return nil, fmt.Errorf("failed to publish test started: %w", err)
		}

		// Reuse StandardStrategy helpers by wrapping them
		wrapper := StandardStrategy{
			sandbox:            s.sandbox,
			pkg:                s.pkg,
			compiledComponents: s.compiledComponents,
		}

		input, err := wrapper.getTestInput(ctx, tc.Test)
		if err != nil {
			return nil, fmt.Errorf("failed to get input for test %d: %w", tc.TestIndex, err)
		}

		answer, err := wrapper.getTestAnswer(ctx, tc.Test, input)
		if err != nil {
			return nil, fmt.Errorf("failed to get answer for test %d: %w", tc.TestIndex, err)
		}

		runRes, err := s.sandbox.Test(ctx, solExec, languageStr, input, s.pkg.Problem.Limits.TimeMs, s.pkg.Problem.Limits.MemoryMb)
		if err != nil {
			return nil, fmt.Errorf("failed to run solution for test %d: %w", tc.TestIndex, err)
		}

		verdict := string(runRes.Status)
		message := string(runRes.Stderr)
		var checkerScore *float64

		if runRes.Status == sandbox.StatusOK {
			chkRes, err := s.sandbox.Check(ctx, checkerExec, input, runRes.Stdout, answer)
			if err != nil {
				verdict = "IE"
				message = fmt.Sprintf("Checker execution failed: %v", err)
			} else {
				verdict = string(chkRes.Status)
				message = chkRes.Message
				checkerScore = chkRes.Score
			}
		}

		res := TestResult{
			TestNumber: tc.TestIndex,
			Verdict:    verdict,
			Score:      checkerScore,
			Time:       runRes.Time.Nanoseconds(),
			Memory:     runRes.Memory,
			Message:    message,
		}
		results = append(results, res)
	}

	calculator := NewVerdictCalculator(s.pkg.Problem.Type, s.pkg.Problem)
	return calculator.Calculate(results), nil
}

type InteractiveStrategy struct {
	sandbox            *sandbox.Sandbox
	eventPublisher     *EventPublisher
	pkg                *gfmt.GateFormat
	compiledComponents map[string]sandbox.Executable
}

func NewInteractiveStrategy(
	sandbox *sandbox.Sandbox,
	eventPublisher *EventPublisher,
	pkg *gfmt.GateFormat,
	compiledComponents map[string]sandbox.Executable,
) *InteractiveStrategy {
	return &InteractiveStrategy{
		sandbox:            sandbox,
		eventPublisher:     eventPublisher,
		pkg:                pkg,
		compiledComponents: compiledComponents,
	}
}

func (s *InteractiveStrategy) Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error) {
	languageStr := convertLanguage(language)

	solExec, err := s.sandbox.Compile(ctx, []byte(sourceCode), languageStr, nil)
	if err != nil {
		return &FinalVerdict{
			State:     models.GotCE,
			Score:     0,
			MaxTime:   0,
			MaxMemory: 0,
			Message:   fmt.Sprintf("Compilation failed: %v", err),
		}, nil
	}

	interactorExec, hasInteractor := s.compiledComponents["interactor"]
	if !hasInteractor {
		return nil, fmt.Errorf("interactor is required for interactive strategy")
	}

	flatTests := collectFlatTests(s.pkg.Problem)
	var results []TestResult

	if err := s.eventPublisher.PublishTestingStarted(ctx, submissionID, meta); err != nil {
		return nil, fmt.Errorf("failed to publish testing started: %w", err)
	}

	for _, tc := range flatTests {
		if err := s.eventPublisher.PublishTestStarted(ctx, submissionID, int32(tc.TestIndex), meta); err != nil {
			return nil, fmt.Errorf("failed to publish test started: %w", err)
		}

		wrapper := StandardStrategy{
			sandbox:            s.sandbox,
			pkg:                s.pkg,
			compiledComponents: s.compiledComponents,
		}

		input, err := wrapper.getTestInput(ctx, tc.Test)
		if err != nil {
			return nil, fmt.Errorf("failed to get input for test %d: %w", tc.TestIndex, err)
		}

		interactRes, err := s.sandbox.Interact(ctx, solExec, languageStr, interactorExec, input, s.pkg.Problem.Limits.TimeMs, s.pkg.Problem.Limits.MemoryMb)
		if err != nil {
			return nil, fmt.Errorf("failed to interact for test %d: %w", tc.TestIndex, err)
		}

		res := TestResult{
			TestNumber: tc.TestIndex,
			Verdict:    string(interactRes.Status),
			Score:      interactRes.Score,
			Time:       interactRes.SolutionResult.Time.Nanoseconds(),
			Memory:     interactRes.SolutionResult.Memory,
			Message:    interactRes.Message,
		}
		results = append(results, res)
	}

	calculator := NewVerdictCalculator(s.pkg.Problem.Type, s.pkg.Problem)
	return calculator.Calculate(results), nil
}

func collectFlatTests(prob *gfmt.Problem) []FlatTest {
	var flatTests []FlatTest
	var subtaskNames []string
	if _, exists := prob.Subtasks["samples"]; exists {
		subtaskNames = append(subtaskNames, "samples")
	}
	for k := range prob.Subtasks {
		if k != "samples" {
			subtaskNames = append(subtaskNames, k)
		}
	}

	testIdx := 1
	for _, subName := range subtaskNames {
		sub := prob.Subtasks[subName]
		for _, t := range sub.Tests {
			flatTests = append(flatTests, FlatTest{
				SubtaskName: subName,
				TestIndex:   testIdx,
				Test:        t,
			})
			testIdx++
		}
	}
	return flatTests
}

func convertLanguage(lang models.LanguageName) string {
	switch lang {
	case models.Golang:
		return "go"
	case models.Cpp:
		return "cpp"
	case models.Python:
		return "python"
	default:
		return "cpp"
	}
}

func detectLanguage(ext string) string {
	switch ext {
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".java":
		return "java"
	default:
		return "cpp"
	}
}

func loadLibDependencies(pkgPath string) (map[string][]byte, error) {
	deps := make(map[string][]byte)
	libDir := filepath.Join(pkgPath, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		if os.IsNotExist(err) {
			return deps, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			data, err := os.ReadFile(filepath.Join(libDir, entry.Name()))
			if err != nil {
				return nil, err
			}
			deps[entry.Name()] = data
		}
	}
	return deps, nil
}
