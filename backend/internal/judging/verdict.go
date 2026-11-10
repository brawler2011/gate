package judging

import (
	"fmt"
	"math"
	"regexp"
	"strconv"

	"github.com/brawler2011/gate/backend/pkg/points"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
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

const maxTestDetailSize = 128 * 1024

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
	genericLineRegex = regexp.MustCompile(`(?i)(?:line|строка|:)\s*(\d+)`)
)

func ParseErrorLine(lang submissionsmodel.LanguageName, text string) *int32 {
	if text == "" {
		return nil
	}

	var match []string
	switch lang {
	case submissionsmodel.Cpp:
		match = cppLineRegex.FindStringSubmatch(text)
	case submissionsmodel.Python:
		match = pythonLineRegex.FindStringSubmatch(text)
	case submissionsmodel.Golang:
		match = goLineRegex.FindStringSubmatch(text)
	default:
		match = cppLineRegex.FindStringSubmatch(text)
	}

	if len(match) == 0 {
		match = genericLineRegex.FindStringSubmatch(text)
	}

	for i := 1; i < len(match); i++ {
		if match[i] != "" {
			if lineNum, err := strconv.ParseInt(match[i], 10, 32); err == nil && lineNum > 0 {
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
	State       submissionsmodel.State
	Score       points.Value
	MaxTime     int32 // milliseconds
	MaxMemory   int32 // megabytes
	Message     string
	FailedTest  *int // test number where it failed (for fail-fast)
	TestDetails *submissionsmodel.SubmissionTestDetails
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
func MapSandboxVerdict(verdict string) submissionsmodel.State {
	switch verdict {
	case "OK", "AC", "Accepted":
		return submissionsmodel.Accepted
	case "WA", "Wrong Answer":
		return submissionsmodel.GotWA
	case "TLE", "Time Limit Exceeded":
		return submissionsmodel.GotTL
	case "MLE", "Memory Limit Exceeded":
		return submissionsmodel.GotML
	case "RE", "Runtime Error":
		return submissionsmodel.GotRE
	case "PE", "Presentation Error":
		return submissionsmodel.GotPE
	case "CE", "Compilation Error":
		return submissionsmodel.GotCE
	case "IE", "FAIL", "FileError", "DangerousSyscall", "InternalError", "Internal Error":
		return submissionsmodel.GotIE
	default:
		return submissionsmodel.GotIE
	}
}

func buildSubmissionTestDetails(results []TestResult) *submissionsmodel.SubmissionTestDetails {
	if len(results) == 0 {
		return nil
	}

	testItems := make([]submissionsmodel.TestDetailItem, 0, len(results))
	var failedTestDetails *submissionsmodel.FailedTestDetail
	var errorLine *int32

	for _, result := range results {
		timeMs := safeInt32(result.Time / 1_000_000)
		memoryKb := safeInt32(result.Memory / 1024)

		testItems = append(testItems, submissionsmodel.TestDetailItem{
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

			failedTestDetails = &submissionsmodel.FailedTestDetail{
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

	return &submissionsmodel.SubmissionTestDetails{
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
		State:       submissionsmodel.Accepted,
		Score:       vc.maxScore(),
		MaxTime:     safeInt32(maxTime / 1_000_000),
		MaxMemory:   safeInt32(maxMemory / 1024 / 1024),
		Message:     "All tests passed",
		TestDetails: buildSubmissionTestDetails(results),
	}
}

// CalculateScoringVerdict uses explicit test weights and complete dependency
// groups. No group score is divided by test count or rescaled to a percentage.
func (vc *VerdictCalculator) CalculateScoringVerdict(results []TestResult) *FinalVerdict {
	byTest := make(map[int]TestResult, len(results))
	var maxTime, maxMemory int64
	for _, r := range results {
		byTest[r.TestNumber] = r
		if r.Time > maxTime {
			maxTime = r.Time
		}
		if r.Memory > maxMemory {
			maxMemory = r.Memory
		}
	}
	passed := func(index int) bool {
		r, ok := byTest[index]
		return ok && (r.Verdict == "OK" || r.Verdict == "AC" || r.Verdict == "Accepted")
	}
	full := map[string]bool{}
	visited := map[string]bool{}
	var groupPassed func(string) bool
	groupPassed = func(name string) bool {
		if visited[name] {
			return full[name]
		}
		visited[name] = true
		group, ok := vc.problem.Subtasks[name]
		if !ok || len(group.Tests) == 0 {
			return false
		}
		for _, dep := range group.Dependencies {
			if !groupPassed(dep) {
				return false
			}
		}
		for _, test := range group.Tests {
			if !passed(test.Ordinal) {
				return false
			}
		}
		full[name] = true
		return true
	}
	var score points.Value
	for name, group := range vc.problem.Subtasks {
		allowed := true
		for _, dep := range group.Dependencies {
			if !groupPassed(dep) {
				allowed = false
				break
			}
		}
		if !allowed {
			continue
		}
		if group.Policy == "complete" {
			if groupPassed(name) {
				for _, test := range group.Tests {
					score += test.Points
				}
			}
		} else {
			for _, test := range group.Tests {
				if passed(test.Ordinal) {
					score += test.Points
				}
			}
		}
	}
	verdict := &FinalVerdict{State: submissionsmodel.Accepted, Score: score, MaxTime: safeInt32(maxTime / 1000000), MaxMemory: safeInt32(maxMemory / 1024 / 1024), Message: "Score: " + score.String(), TestDetails: buildSubmissionTestDetails(results)}
	for _, test := range collectFlatTests(vc.problem) {
		if !passed(test.TestIndex) {
			n := test.TestIndex
			verdict.FailedTest = &n
			if r, ok := byTest[n]; ok {
				verdict.State = MapSandboxVerdict(r.Verdict)
			} else {
				verdict.State = submissionsmodel.GotIE
			}
			break
		}
	}
	return verdict
}

func (vc *VerdictCalculator) CalculateInteractiveVerdict(results []TestResult) *FinalVerdict {
	return vc.CalculateStandardVerdict(results)
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

// maxScore is the sum of every explicit test weight: the score of a fully
// accepted solution.
func (vc *VerdictCalculator) maxScore() points.Value {
	var total points.Value
	for _, group := range vc.problem.Subtasks {
		for _, test := range group.Tests {
			total += test.Points
		}
	}
	return total
}
