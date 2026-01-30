package judge

import (
	"fmt"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/sandbox"
)

// TestResult represents the result of a single test
type TestResult struct {
	TestNumber int
	Verdict    string
	Score      *float64
	Time       int64 // nanoseconds
	Memory     int64 // bytes
	Message    string
	Error      string
}

// FinalVerdict represents the final judging result
type FinalVerdict struct {
	State      models.State
	Score      int32
	MaxTime    int32 // milliseconds
	MaxMemory  int32 // megabytes
	Message    string
	FailedTest *int // test number where it failed (for fail-fast)
}

// VerdictCalculator calculates final verdict from test results
type VerdictCalculator struct {
	problemType   string
	testsMetadata problemformat.TestsMetadata
}

// NewVerdictCalculator creates a new verdict calculator
func NewVerdictCalculator(problemType string, testsMetadata problemformat.TestsMetadata) *VerdictCalculator {
	return &VerdictCalculator{
		problemType:   problemType,
		testsMetadata: testsMetadata,
	}
}

// MapSandboxVerdict maps go-judge verdict to submission state
func MapSandboxVerdict(verdict string) models.State {
	switch verdict {
	case "OK", "AC", "Accepted":
		return models.Accepted
	case "WA", "Wrong Answer":
		return models.GotWA
	case "TLE", "Time Limit Exceeded":
		return models.GotTL
	case "MLE", "Memory Limit Exceeded":
		return models.GotML
	case "RE", "Runtime Error":
		return models.GotRE
	case "PE", "Presentation Error":
		return models.GotPE
	case "CE", "Compilation Error":
		return models.GotCE
	default:
		return models.GotRE // default to runtime error for unknown verdicts
	}
}

// CalculateStandardVerdict calculates verdict for standard (pass-fail) problems
func (vc *VerdictCalculator) CalculateStandardVerdict(results []TestResult) *FinalVerdict {
	var maxTime int64
	var maxMemory int64

	for _, result := range results {
		// Update max time and memory
		if result.Time > maxTime {
			maxTime = result.Time
		}
		if result.Memory > maxMemory {
			maxMemory = result.Memory
		}

		// If any test failed, return that verdict
		if result.Verdict != "OK" && result.Verdict != "AC" && result.Verdict != "Accepted" {
			testNum := result.TestNumber
			return &FinalVerdict{
				State:      MapSandboxVerdict(result.Verdict),
				Score:      0,
				MaxTime:    int32(maxTime / 1_000_000),     // convert ns to ms
				MaxMemory:  int32(maxMemory / 1024 / 1024), // convert bytes to MB
				Message:    fmt.Sprintf("Failed on test %d: %s", result.TestNumber, result.Message),
				FailedTest: &testNum,
			}
		}
	}

	// All tests passed
	return &FinalVerdict{
		State:     models.Accepted,
		Score:     100,
		MaxTime:   int32(maxTime / 1_000_000),
		MaxMemory: int32(maxMemory / 1024 / 1024),
		Message:   "All tests passed",
	}
}

// CalculateScoringVerdict calculates verdict for scoring problems with test groups
func (vc *VerdictCalculator) CalculateScoringVerdict(results []TestResult) *FinalVerdict {
	var maxTime int64
	var maxMemory int64
	totalScore := 0.0

	// Build map of test results
	testResults := make(map[int]TestResult)
	for _, result := range results {
		testResults[result.TestNumber] = result

		// Update max time and memory
		if result.Time > maxTime {
			maxTime = result.Time
		}
		if result.Memory > maxMemory {
			maxMemory = result.Memory
		}
	}

	// Calculate scores for each group
	for _, group := range vc.testsMetadata.Groups {
		// Check if dependencies are satisfied
		if !vc.checkGroupDependencies(group, testResults) {
			continue // skip this group
		}

		groupScore := vc.calculateGroupScore(group, testResults)
		totalScore += groupScore
	}

	// Calculate max possible score
	maxPossibleScore := 0
	for _, group := range vc.testsMetadata.Groups {
		maxPossibleScore += group.Points
	}

	// Normalize score to 0-100
	normalizedScore := int32(0)
	if maxPossibleScore > 0 {
		normalizedScore = int32((totalScore / float64(maxPossibleScore)) * 100)
	}

	return &FinalVerdict{
		State:     models.Accepted, // scoring problems always "accept" with a score
		Score:     normalizedScore,
		MaxTime:   int32(maxTime / 1_000_000),
		MaxMemory: int32(maxMemory / 1024 / 1024),
		Message:   fmt.Sprintf("Score: %d/%d points", int(totalScore), maxPossibleScore),
	}
}

// checkGroupDependencies checks if all group dependencies are satisfied
func (vc *VerdictCalculator) checkGroupDependencies(group problemformat.TestGroup, results map[int]TestResult) bool {
	for _, depGroupOrdinal := range group.DependsOn {
		// Find the dependency group
		var depGroup *problemformat.TestGroup
		for _, g := range vc.testsMetadata.Groups {
			if g.Ordinal == depGroupOrdinal {
				depGroup = &g
				break
			}
		}

		if depGroup == nil {
			continue
		}

		// Check if all tests in dependency group passed
		for testNum := depGroup.Tests[0]; testNum <= depGroup.Tests[1]; testNum++ {
			result, exists := results[testNum]
			if !exists || (result.Verdict != "OK" && result.Verdict != "AC" && result.Verdict != "Accepted") {
				return false // dependency not satisfied
			}
		}
	}
	return true
}

// calculateGroupScore calculates score for a single group
func (vc *VerdictCalculator) calculateGroupScore(group problemformat.TestGroup, results map[int]TestResult) float64 {
	startTest := group.Tests[0]
	endTest := group.Tests[1]
	totalTests := endTest - startTest + 1

	if group.PointsPolicy == "complete-group" {
		// All tests in group must pass to get points
		for testNum := startTest; testNum <= endTest; testNum++ {
			result, exists := results[testNum]
			if !exists || (result.Verdict != "OK" && result.Verdict != "AC" && result.Verdict != "Accepted") {
				return 0 // group failed
			}
		}
		return float64(group.Points) // all passed
	}

	// "each-test" policy: proportional scoring
	passedTests := 0
	totalTestScore := 0.0

	for testNum := startTest; testNum <= endTest; testNum++ {
		result, exists := results[testNum]
		if exists {
			if result.Verdict == "OK" || result.Verdict == "AC" || result.Verdict == "Accepted" {
				if result.Score != nil {
					totalTestScore += *result.Score // use checker-provided score
				} else {
					passedTests++ // full credit for this test
				}
			}
		}
	}

	// If checker provides scores, use those
	if totalTestScore > 0 {
		// Normalize checker scores to group points
		return (totalTestScore / float64(totalTests)) * float64(group.Points) / 100.0
	}

	// Otherwise, proportional to passed tests
	return (float64(passedTests) / float64(totalTests)) * float64(group.Points)
}

// CalculateInteractiveVerdict calculates verdict for interactive problems
func (vc *VerdictCalculator) CalculateInteractiveVerdict(results []TestResult) *FinalVerdict {
	// Interactive problems are similar to standard, but verdict comes from interactor
	var maxTime int64
	var maxMemory int64
	totalScore := 0.0
	hasScore := false

	for _, result := range results {
		// Update max time and memory
		if result.Time > maxTime {
			maxTime = result.Time
		}
		if result.Memory > maxMemory {
			maxMemory = result.Memory
		}

		// Check if interactor provides scores
		if result.Score != nil {
			hasScore = true
			totalScore += *result.Score
		}

		// If any test failed without score, return that verdict
		if result.Verdict != "OK" && result.Verdict != "AC" && result.Verdict != "Accepted" && result.Score == nil {
			testNum := result.TestNumber
			return &FinalVerdict{
				State:      MapSandboxVerdict(result.Verdict),
				Score:      0,
				MaxTime:    int32(maxTime / 1_000_000),
				MaxMemory:  int32(maxMemory / 1024 / 1024),
				Message:    fmt.Sprintf("Failed on test %d: %s", result.TestNumber, result.Message),
				FailedTest: &testNum,
			}
		}
	}

	// If scoring is used, calculate average
	if hasScore && len(results) > 0 {
		avgScore := totalScore / float64(len(results))
		return &FinalVerdict{
			State:     models.Accepted,
			Score:     int32(avgScore),
			MaxTime:   int32(maxTime / 1_000_000),
			MaxMemory: int32(maxMemory / 1024 / 1024),
			Message:   fmt.Sprintf("Score: %.1f points", avgScore),
		}
	}

	// All tests passed (no scoring)
	return &FinalVerdict{
		State:     models.Accepted,
		Score:     100,
		MaxTime:   int32(maxTime / 1_000_000),
		MaxMemory: int32(maxMemory / 1024 / 1024),
		Message:   "All tests passed",
	}
}

// Calculate determines final verdict based on problem type
func (vc *VerdictCalculator) Calculate(results []TestResult) *FinalVerdict {
	switch vc.problemType {
	case "scoring":
		return vc.CalculateScoringVerdict(results)
	case "interactive":
		return vc.CalculateInteractiveVerdict(results)
	default: // "pass-fail" or any other type defaults to standard
		return vc.CalculateStandardVerdict(results)
	}
}

// ConvertJudgeResultToTestResult converts sandbox JudgeResult to TestResult
func ConvertJudgeResultToTestResult(testNum int, result *sandbox.JudgeResult) TestResult {
	return TestResult{
		TestNumber: testNum,
		Verdict:    result.Verdict,
		Score:      result.Score,
		Time:       result.Time,
		Memory:     result.Memory,
		Message:    result.Message,
		Error:      result.CompileError + result.ExecutionError,
	}
}
