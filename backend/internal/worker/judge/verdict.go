package judge

import (
	"fmt"
	"math"
	"regexp"
	"strconv"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
)

func safeInt32[T ~int | ~int64 | ~float64](v T) int32 {
	v64 := float64(v)
	if v64 > math.MaxInt32 {
		return math.MaxInt32
	}
	if v64 < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

const maxTestDetailSize = 128 * 1024 // 128 KB

func truncateDetailString(s string) (string, bool) {
	if len(s) > maxTestDetailSize {
		return s[:maxTestDetailSize], true
	}
	return s, false
}

var (
	cppLineRegex     = regexp.MustCompile(`(?i)(?:foo\.cc|solution\.cpp|\.cpp|\.cc|\.cxx|\.h|\.hpp):(\d+):(?:\d+:)?\s*(?:error|fatal error|runtime error|warning|note):?`)
	pythonLineRegex  = regexp.MustCompile(`(?i)(?:File\s+["'][^"']*["'],\s+line\s+(\d+)|line\s+(\d+))`)
	goLineRegex      = regexp.MustCompile(`(?i)(?:foo\.go|\.go|main\.go):(\d+):(?:\d+:)?`)
	javaLineRegex    = regexp.MustCompile(`(?i)(?:Main\.java|\.java):(\d+):\s*error:?`)
	genericLineRegex = regexp.MustCompile(`(?i)(?:line|строка|:)\s*(\d+)`)
)

func ParseErrorLine(lang models.LanguageName, text string) *int32 {
	if text == "" {
		return nil
	}

	var match []string
	switch lang {
	case models.Cpp:
		match = cppLineRegex.FindStringSubmatch(text)
	case models.Python:
		match = pythonLineRegex.FindStringSubmatch(text)
	case models.Golang:
		match = goLineRegex.FindStringSubmatch(text)
	default:
		match = cppLineRegex.FindStringSubmatch(text)
	}

	if len(match) == 0 {
		match = genericLineRegex.FindStringSubmatch(text)
	}

	for i := 1; i < len(match); i++ {
		if match[i] != "" {
			if lineNum, err := strconv.Atoi(match[i]); err == nil && lineNum > 0 {
				res := int32(lineNum)
				return &res
			}
		}
	}

	return nil
}

// TestResult represents the result of a single test
type TestResult struct {
	TestNumber    int
	Verdict       string
	Score         *float64
	Time          int64 // nanoseconds
	Memory        int64 // bytes
	Message       string
	Error         string
	Input         string
	Output        string
	Answer        string
	CheckerOutput string
	IsTruncated   bool
	ErrorLine     *int32
}

// FinalVerdict represents the final judging result
type FinalVerdict struct {
	State       models.State
	Score       int32
	MaxTime     int32 // milliseconds
	MaxMemory   int32 // megabytes
	Message     string
	FailedTest  *int // test number where it failed (for fail-fast)
	TestDetails *models.SubmissionTestDetails
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
	case "IE", "FAIL", "FileError", "DangerousSyscall", "InternalError", "Internal Error":
		return models.GotIE
	default:
		return models.GotIE
	}
}

func buildSubmissionTestDetails(results []TestResult) *models.SubmissionTestDetails {
	if len(results) == 0 {
		return nil
	}

	testItems := make([]models.TestDetailItem, 0, len(results))
	var failedTestDetails *models.FailedTestDetail
	var errorLine *int32

	for _, result := range results {
		timeMs := safeInt32(result.Time / 1_000_000)
		memoryKb := safeInt32(result.Memory / 1024)

		testItems = append(testItems, models.TestDetailItem{
			TestIndex: safeInt32(result.TestNumber),
			Verdict:   result.Verdict,
			TimeMs:    timeMs,
			MemoryKb:  memoryKb,
		})

		if result.Verdict != "OK" && result.Verdict != "AC" && result.Verdict != "Accepted" && failedTestDetails == nil {
			inStr, inTrunc := truncateDetailString(result.Input)
			outStr, outTrunc := truncateDetailString(result.Output)
			ansStr, ansTrunc := truncateDetailString(result.Answer)
			chkStr, chkTrunc := truncateDetailString(result.CheckerOutput)
			errStr, errTrunc := truncateDetailString(result.Message)

			isTruncated := result.IsTruncated || inTrunc || outTrunc || ansTrunc || chkTrunc || errTrunc

			failedTestDetails = &models.FailedTestDetail{
				TestIndex:     safeInt32(result.TestNumber),
				Input:         inStr,
				Output:        outStr,
				Answer:        ansStr,
				CheckerOutput: chkStr,
				ErrorMessage:  errStr,
				IsTruncated:   isTruncated,
			}
			if result.ErrorLine != nil {
				errorLine = result.ErrorLine
			}
		}
	}

	return &models.SubmissionTestDetails{
		ErrorLine:         errorLine,
		Tests:             testItems,
		FailedTestDetails: failedTestDetails,
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
				State:       MapSandboxVerdict(result.Verdict),
				Score:       0,
				MaxTime:     safeInt32(maxTime / 1_000_000),     // convert ns to ms
				MaxMemory:   safeInt32(maxMemory / 1024 / 1024), // convert bytes to MB
				Message:     fmt.Sprintf("Failed on test %d: %s", result.TestNumber, result.Message),
				FailedTest:  &testNum,
				TestDetails: buildSubmissionTestDetails(results),
			}
		}
	}

	return &FinalVerdict{
		State:       models.Accepted,
		Score:       100,
		MaxTime:     safeInt32(maxTime / 1_000_000),
		MaxMemory:   safeInt32(maxMemory / 1024 / 1024),
		Message:     "All tests passed",
		TestDetails: buildSubmissionTestDetails(results),
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
		if !vc.checkSubtaskDependencies(sub, testResults) {
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
		normalizedScore = safeInt32((totalScore / float64(maxPossibleScore)) * 100)
	}

	return &FinalVerdict{
		State:       models.Accepted, // scoring problems always "Accepted" with a score
		Score:       normalizedScore,
		MaxTime:     safeInt32(maxTime / 1_000_000),
		MaxMemory:   safeInt32(maxMemory / 1024 / 1024),
		Message:     fmt.Sprintf("Score: %d/%d points", int(totalScore), maxPossibleScore),
		TestDetails: buildSubmissionTestDetails(results),
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

func (vc *VerdictCalculator) checkSubtaskDependencies(subtask gfmt.Subtask, results map[int]TestResult) bool {
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
				State:       MapSandboxVerdict(result.Verdict),
				Score:       0,
				MaxTime:     safeInt32(maxTime / 1_000_000),
				MaxMemory:   safeInt32(maxMemory / 1024 / 1024),
				Message:     fmt.Sprintf("Failed on test %d: %s", result.TestNumber, result.Message),
				FailedTest:  &testNum,
				TestDetails: buildSubmissionTestDetails(results),
			}
		}
	}

	if hasScore && len(results) > 0 {
		avgScore := totalScore / float64(len(results))
		return &FinalVerdict{
			State:       models.Accepted,
			Score:       safeInt32(avgScore),
			MaxTime:     safeInt32(maxTime / 1_000_000),
			MaxMemory:   safeInt32(maxMemory / 1024 / 1024),
			Message:     fmt.Sprintf("Score: %.1f points", avgScore),
			TestDetails: buildSubmissionTestDetails(results),
		}
	}

	return &FinalVerdict{
		State:       models.Accepted,
		Score:       100,
		MaxTime:     safeInt32(maxTime / 1_000_000),
		MaxMemory:   safeInt32(maxMemory / 1024 / 1024),
		Message:     "All tests passed",
		TestDetails: buildSubmissionTestDetails(results),
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
