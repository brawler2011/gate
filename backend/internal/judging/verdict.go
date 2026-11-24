package judging

import (
	"fmt"
	"math"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"

	"github.com/brawler2011/gate/backend/pkg/calibration"
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

type TestResult struct {
	TestNumber    int
	Sample        bool
	Public        bool
	Group         string
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
}

type FinalVerdict struct {
	State       submissionsmodel.State
	Score       points.Value
	MaxTime     int32 // milliseconds
	MaxMemory   int32 // megabytes
	Message     string
	FailedTest  *int // first failing test in fail-fast mode
	Pretests    bool
	TestDetails *submissionsmodel.SubmissionTestDetails
	Calibration calibration.Calibration
}

type VerdictCalculator struct {
	problemType string
	problem     *gfmt.Problem
}

func NewVerdictCalculator(problemType string, problem *gfmt.Problem) *VerdictCalculator {
	return &VerdictCalculator{
		problemType: problemType,
		problem:     problem,
	}
}

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

	for _, result := range results {
		timeMs := safeInt32(result.Time / 1_000_000)
		memoryKb := safeInt32(result.Memory / 1024)

		testItems = append(testItems, submissionsmodel.TestDetailItem{
			TestIndex: safeInt32(result.TestNumber),
			Verdict:   result.Verdict,
			TimeMs:    timeMs,
			MemoryKb:  memoryKb,
			Sample:    result.Sample,
			Group:     result.Group,
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
				Public:        result.Public,
				Input:         inStr,
				Output:        outStr,
				Answer:        ansStr,
				CheckerOutput: chkStr,
				ErrorMessage:  errStr,
				IsTruncated:   isTruncated,
			}
		}
	}

	return &submissionsmodel.SubmissionTestDetails{
		Tests:             testItems,
		FailedTestDetails: failedTestDetails,
	}
}

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
				MaxTime:     safeInt32(maxTime / 1_000_000),
				MaxMemory:   safeInt32(maxMemory / 1024 / 1024),
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

// CalculateScoringVerdict grades the testset; the state is that of the first failed test, whatever the score.
func (vc *VerdictCalculator) CalculateScoringVerdict(results []TestResult) *FinalVerdict {
	byTest := make(map[int]TestResult, len(results))
	var maxTime, maxMemory int64
	for _, r := range results {
		byTest[r.TestNumber] = r
		maxTime = max(maxTime, r.Time)
		maxMemory = max(maxMemory, r.Memory)
	}
	passed := func(ordinal int) bool {
		r, ok := byTest[ordinal]
		return ok && (r.Verdict == "OK" || r.Verdict == "AC" || r.Verdict == "Accepted")
	}
	score, groups := vc.problem.Grade(passed)
	verdict := &FinalVerdict{State: submissionsmodel.Accepted, Score: score, MaxTime: safeInt32(maxTime / 1000000), MaxMemory: safeInt32(maxMemory / 1024 / 1024), Message: "Score: " + score.String(), TestDetails: buildSubmissionTestDetails(results)}
	if verdict.TestDetails != nil {
		verdict.TestDetails.Groups = groupResults(groups, true)
	}
	for _, test := range vc.problem.Tests() {
		if !passed(test.Test.Ordinal) {
			n := test.Test.Ordinal
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

// groupResults carries feedback policies for the report, and scores when scored is set.
func groupResults(groups []gfmt.GroupResult, scored bool) []submissionsmodel.GroupResult {
	results := make([]submissionsmodel.GroupResult, 0, len(groups))
	for _, g := range groups {
		result := submissionsmodel.GroupResult{Name: g.Group.Name, Feedback: g.Group.Feedback}
		if scored {
			result.Score, result.Points = &g.Score, &g.Max
		}
		results = append(results, result)
	}
	return results
}

func (vc *VerdictCalculator) Calculate(results []TestResult) *FinalVerdict {
	if spec.Scored(vc.problemType) {
		return vc.CalculateScoringVerdict(results)
	}
	verdict := vc.CalculateStandardVerdict(results)
	if verdict.TestDetails != nil {
		_, groups := vc.problem.Grade(func(int) bool { return true })
		verdict.TestDetails.Groups = groupResults(groups, false)
	}
	return verdict
}

func (vc *VerdictCalculator) maxScore() points.Value {
	score, _ := vc.problem.Grade(func(int) bool { return true })
	return score
}
