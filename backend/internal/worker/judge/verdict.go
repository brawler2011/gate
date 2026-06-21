package judge

import (
	"fmt"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
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
	problemType string
	problem     *gfmt.Problem
}

// NewVerdictCalculator creates a new verdict calculator
func NewVerdictCalculator(problemType string, problem *gfmt.Problem) *VerdictCalculator {
	return &VerdictCalculator{
		problemType: problemType,
		problem:     problem,
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
		if result.Time > maxTime {
			maxTime = result.Time
		}
		if result.Memory > maxMemory {
			maxMemory = result.Memory
		}

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

	testResults := make(map[int]TestResult)
	for _, result := range results {
		testResults[result.TestNumber] = result
		if result.Time > maxTime {
			maxTime = result.Time
		}
		if result.Memory > maxMemory {
			maxMemory = result.Memory
		}
	}

	for subName, sub := range vc.problem.Subtasks {
		if !vc.checkSubtaskDependencies(subName, sub, testResults) {
			continue
		}
		totalScore += vc.calculateSubtaskScore(subName, sub, testResults)
	}

	maxPossibleScore := 0
	for _, sub := range vc.problem.Subtasks {
		maxPossibleScore += sub.Points
	}

	normalizedScore := int32(0)
	if maxPossibleScore > 0 {
		normalizedScore = int32((totalScore / float64(maxPossibleScore)) * 100)
	}

	return &FinalVerdict{
		State:     models.Accepted, // scoring problems always "Accepted" with a score
		Score:     normalizedScore,
		MaxTime:   int32(maxTime / 1_000_000),
		MaxMemory: int32(maxMemory / 1024 / 1024),
		Message:   fmt.Sprintf("Score: %d/%d points", int(totalScore), maxPossibleScore),
	}
}

func (vc *VerdictCalculator) getSubtaskTestIndexes(subName string) []int {
	var idxs []int
	flat := collectFlatTests(vc.problem)
	for _, ft := range flat {
		if ft.SubtaskName == subName {
			idxs = append(idxs, ft.TestIndex)
		}
	}
	return idxs
}

func (vc *VerdictCalculator) checkSubtaskDependencies(subName string, subtask gfmt.Subtask, results map[int]TestResult) bool {
	for _, depName := range subtask.Dependencies {
		depTests := vc.getSubtaskTestIndexes(depName)
		for _, testNum := range depTests {
			res, exists := results[testNum]
			if !exists || (res.Verdict != "OK" && res.Verdict != "AC" && res.Verdict != "Accepted") {
				return false
			}
		}
	}
	return true
}

func (vc *VerdictCalculator) calculateSubtaskScore(subName string, subtask gfmt.Subtask, results map[int]TestResult) float64 {
	testIdxs := vc.getSubtaskTestIndexes(subName)
	if len(testIdxs) == 0 {
		return 0
	}

	if subtask.Policy == "complete" {
		for _, testNum := range testIdxs {
			res, exists := results[testNum]
			if !exists || (res.Verdict != "OK" && res.Verdict != "AC" && res.Verdict != "Accepted") {
				return 0
			}
		}
		return float64(subtask.Points)
	}

	passedTests := 0
	totalTestScore := 0.0
	for _, testNum := range testIdxs {
		res, exists := results[testNum]
		if exists && (res.Verdict == "OK" || res.Verdict == "AC" || res.Verdict == "Accepted") {
			if res.Score != nil {
				totalTestScore += *res.Score
			} else {
				passedTests++
			}
		}
	}

	if totalTestScore > 0 {
		return (totalTestScore / float64(len(testIdxs))) * float64(subtask.Points) / 100.0
	}
	return (float64(passedTests) / float64(len(testIdxs))) * float64(subtask.Points)
}

// CalculateInteractiveVerdict calculates verdict for interactive problems
func (vc *VerdictCalculator) CalculateInteractiveVerdict(results []TestResult) *FinalVerdict {
	var maxTime int64
	var maxMemory int64
	totalScore := 0.0
	hasScore := false

	for _, result := range results {
		if result.Time > maxTime {
			maxTime = result.Time
		}
		if result.Memory > maxMemory {
			maxMemory = result.Memory
		}

		if result.Score != nil {
			hasScore = true
			totalScore += *result.Score
		}

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
	default:
		return vc.CalculateStandardVerdict(results)
	}
}
