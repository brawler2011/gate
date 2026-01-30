package judge

import (
	"context"
	"fmt"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

// JudgingStrategy defines the interface for different judging strategies
type JudgingStrategy interface {
	Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error)
}

// StandardStrategy implements fail-fast judging for standard problems
type StandardStrategy struct {
	sandboxClient      *sandbox.Client
	eventPublisher     *EventPublisher
	pkg                *problemformat.ProblemPackage
	compiledComponents map[string]string // component type -> fileID
}

// NewStandardStrategy creates a new standard strategy
func NewStandardStrategy(
	sandboxClient *sandbox.Client,
	eventPublisher *EventPublisher,
	pkg *problemformat.ProblemPackage,
	compiledComponents map[string]string,
) *StandardStrategy {
	return &StandardStrategy{
		sandboxClient:      sandboxClient,
		eventPublisher:     eventPublisher,
		pkg:                pkg,
		compiledComponents: compiledComponents,
	}
}

// Judge executes standard judging with fail-fast strategy
func (s *StandardStrategy) Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error) {
	orchestrator := sandbox.NewOrchestrator(s.sandboxClient)

	// Get checker (optional for standard problems)
	checkerFileID := ""
	if fileID, exists := s.compiledComponents["checker"]; exists {
		checkerFileID = fileID
	}

	// Convert language
	languageStr := convertLanguage(language)

	var results []TestResult

	// Publish testing started
	if err := s.eventPublisher.PublishTestingStarted(ctx, submissionID, meta); err != nil {
		return nil, fmt.Errorf("failed to publish testing started: %w", err)
	}

	// Run tests in order, stop on first failure
	for _, testCase := range s.pkg.TestCases {
		// Publish test started
		if err := s.eventPublisher.PublishTestStarted(ctx, submissionID, int32(testCase.Ordinal), meta); err != nil {
			return nil, fmt.Errorf("failed to publish test started: %w", err)
		}

		// Judge solution on this test
		judgeReq := sandbox.JudgeSolutionRequest{
			SolutionCode:     sourceCode,
			SolutionLanguage: languageStr,
			CheckerFileID:    checkerFileID,
			Input:            testCase.Input,
			Answer:           testCase.Output,
			TimeLimitMs:      int64(s.pkg.Manifest.TimeLimitMs),
			MemoryLimitMB:    int64(s.pkg.Manifest.MemoryLimitMb),
		}

		result, err := orchestrator.JudgeSolution(ctx, judgeReq)
		if err != nil {
			return nil, fmt.Errorf("failed to judge test %d: %w", testCase.Ordinal, err)
		}

		testResult := ConvertJudgeResultToTestResult(testCase.Ordinal, result)
		results = append(results, testResult)

		// Stop on first non-AC verdict (fail-fast)
		if result.Verdict != "OK" && result.Verdict != "AC" && result.Verdict != "Accepted" {
			break
		}
	}

	// Calculate final verdict
	calculator := NewVerdictCalculator("pass-fail", s.pkg.TestsMetadata)
	verdict := calculator.Calculate(results)

	return verdict, nil
}

// ScoringStrategy implements judging for scoring problems with test groups
type ScoringStrategy struct {
	sandboxClient      *sandbox.Client
	eventPublisher     *EventPublisher
	pkg                *problemformat.ProblemPackage
	compiledComponents map[string]string
}

// NewScoringStrategy creates a new scoring strategy
func NewScoringStrategy(
	sandboxClient *sandbox.Client,
	eventPublisher *EventPublisher,
	pkg *problemformat.ProblemPackage,
	compiledComponents map[string]string,
) *ScoringStrategy {
	return &ScoringStrategy{
		sandboxClient:      sandboxClient,
		eventPublisher:     eventPublisher,
		pkg:                pkg,
		compiledComponents: compiledComponents,
	}
}

// Judge executes scoring judging (runs all tests)
func (s *ScoringStrategy) Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error) {
	orchestrator := sandbox.NewOrchestrator(s.sandboxClient)

	// Get checker (required for scoring problems)
	checkerFileID, exists := s.compiledComponents["checker"]
	if !exists {
		return nil, fmt.Errorf("checker is required for scoring problems")
	}

	// Convert language
	languageStr := convertLanguage(language)

	var results []TestResult

	// Publish testing started
	if err := s.eventPublisher.PublishTestingStarted(ctx, submissionID, meta); err != nil {
		return nil, fmt.Errorf("failed to publish testing started: %w", err)
	}

	// Run all tests (no fail-fast)
	for _, testCase := range s.pkg.TestCases {
		// Publish test started
		if err := s.eventPublisher.PublishTestStarted(ctx, submissionID, int32(testCase.Ordinal), meta); err != nil {
			return nil, fmt.Errorf("failed to publish test started: %w", err)
		}

		// Judge solution on this test
		judgeReq := sandbox.JudgeSolutionRequest{
			SolutionCode:     sourceCode,
			SolutionLanguage: languageStr,
			CheckerFileID:    checkerFileID,
			Input:            testCase.Input,
			Answer:           testCase.Output,
			TimeLimitMs:      int64(s.pkg.Manifest.TimeLimitMs),
			MemoryLimitMB:    int64(s.pkg.Manifest.MemoryLimitMb),
		}

		result, err := orchestrator.JudgeSolution(ctx, judgeReq)
		if err != nil {
			return nil, fmt.Errorf("failed to judge test %d: %w", testCase.Ordinal, err)
		}

		testResult := ConvertJudgeResultToTestResult(testCase.Ordinal, result)
		results = append(results, testResult)

		// Continue even if test failed (scoring problems run all tests)
	}

	// Calculate final verdict with scoring
	calculator := NewVerdictCalculator("scoring", s.pkg.TestsMetadata)
	verdict := calculator.Calculate(results)

	return verdict, nil
}

// InteractiveStrategy implements judging for interactive problems
type InteractiveStrategy struct {
	sandboxClient      *sandbox.Client
	eventPublisher     *EventPublisher
	pkg                *problemformat.ProblemPackage
	compiledComponents map[string]string
}

// NewInteractiveStrategy creates a new interactive strategy
func NewInteractiveStrategy(
	sandboxClient *sandbox.Client,
	eventPublisher *EventPublisher,
	pkg *problemformat.ProblemPackage,
	compiledComponents map[string]string,
) *InteractiveStrategy {
	return &InteractiveStrategy{
		sandboxClient:      sandboxClient,
		eventPublisher:     eventPublisher,
		pkg:                pkg,
		compiledComponents: compiledComponents,
	}
}

// Judge executes interactive judging
func (s *InteractiveStrategy) Judge(ctx context.Context, submissionID uuid.UUID, sourceCode string, language models.LanguageName, meta models.SubmissionEventMeta) (*FinalVerdict, error) {
	// TODO: Interactive judging requires special sandbox support for bidirectional communication
	// For now, we'll use a simplified approach similar to standard judging
	// In a full implementation, this would:
	// 1. Compile both solution and interactor
	// 2. Run them in parallel with pipe communication
	// 3. Interactor controls the verdict

	orchestrator := sandbox.NewOrchestrator(s.sandboxClient)

	// Get interactor (required for interactive problems)
	interactorFileID, exists := s.compiledComponents["interactor"]
	if !exists {
		return nil, fmt.Errorf("interactor is required for interactive problems")
	}

	// For now, fallback to using interactor as a checker
	// This is a simplified implementation
	checkerFileID := interactorFileID

	// Convert language
	languageStr := convertLanguage(language)

	var results []TestResult

	// Publish testing started
	if err := s.eventPublisher.PublishTestingStarted(ctx, submissionID, meta); err != nil {
		return nil, fmt.Errorf("failed to publish testing started: %w", err)
	}

	// Run tests with interactor
	for _, testCase := range s.pkg.TestCases {
		// Publish test started
		if err := s.eventPublisher.PublishTestStarted(ctx, submissionID, int32(testCase.Ordinal), meta); err != nil {
			return nil, fmt.Errorf("failed to publish test started: %w", err)
		}

		// Judge solution on this test (using interactor as checker)
		judgeReq := sandbox.JudgeSolutionRequest{
			SolutionCode:     sourceCode,
			SolutionLanguage: languageStr,
			CheckerFileID:    checkerFileID,
			Input:            testCase.Input,
			Answer:           testCase.Output,
			TimeLimitMs:      int64(s.pkg.Manifest.TimeLimitMs),
			MemoryLimitMB:    int64(s.pkg.Manifest.MemoryLimitMb),
		}

		result, err := orchestrator.JudgeSolution(ctx, judgeReq)
		if err != nil {
			return nil, fmt.Errorf("failed to judge test %d: %w", testCase.Ordinal, err)
		}

		testResult := ConvertJudgeResultToTestResult(testCase.Ordinal, result)
		results = append(results, testResult)

		// For interactive, we might stop on first failure depending on the problem
		// For now, continue like scoring problems
	}

	// Calculate final verdict
	calculator := NewVerdictCalculator("interactive", s.pkg.TestsMetadata)
	verdict := calculator.Calculate(results)

	return verdict, nil
}

// convertLanguage converts models.LanguageName to string for sandbox
func convertLanguage(lang models.LanguageName) string {
	switch lang {
	case models.Golang:
		return "go"
	case models.Cpp:
		return "cpp17"
	case models.Python:
		return "python3"
	default:
		return "cpp17" // default
	}
}
