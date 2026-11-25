package judging

import (
	"fmt"
	"math"

	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
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
	TestNumber int
	Sample     bool
	Public     bool
	Group      string
	Verdict    string
	// Score is the number a checker or interactor gave with POINTS; Award turns it into Points.
	Score         *float64
	Points        points.Value
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

func isOK(verdict string) bool { return verdict == "OK" || verdict == "AC" || verdict == "Accepted" }

// Award settles what the test earned: OK earns its points, and POINTS the jury's own number as in Polygon,
// which passes the test only when it is all of them. Points on a pass-fail problem or out of range are a jury failure.
func (r *TestResult) Award(max points.Value, grading string) {
	r.Points = 0
	switch {
	case isOK(r.Verdict):
		r.Points = max
	case r.Verdict != "POINTS":
	case grading != spec.GradingPoints:
		r.Verdict, r.Message = "FAIL", "the jury gave points on a pass-fail problem"
	case r.Score == nil:
		r.Verdict, r.Message = "FAIL", "the jury gave no points with the POINTS verdict"
	default:
		earned, ok := points.FromFloat(*r.Score)
		switch {
		case !ok || *r.Score > max.Float64() || earned > max:
			r.Verdict, r.Message = "FAIL", fmt.Sprintf("the jury gave %g points, the test has %s", *r.Score, max)
		case earned == max:
			r.Verdict, r.Points = "OK", earned
		default:
			r.Points = earned
		}
	}
}

type FinalVerdict struct {
	State       submissionsmodel.State
	Score       points.Value
	MaxTime     int32 // milliseconds
	MaxMemory   int32 // megabytes
	Message     string
	FailedTest  *int
	Pretests    bool
	TestDetails *submissionsmodel.SubmissionTestDetails
	Calibration calibration.Calibration
}

// VerdictCalculator grades awarded test results of a problem.
type VerdictCalculator struct {
	problem *gfmt.Problem
}

func NewVerdictCalculator(problem *gfmt.Problem) *VerdictCalculator {
	return &VerdictCalculator{problem: problem}
}

func MapSandboxVerdict(verdict string) submissionsmodel.State {
	switch verdict {
	case "OK", "AC", "Accepted":
		return submissionsmodel.Accepted
	case "WA", "Wrong Answer", "POINTS":
		return submissionsmodel.GotWA
	case "TLE", "Time Limit Exceeded":
		return submissionsmodel.GotTL
	case "MLE", "Memory Limit Exceeded":
		return submissionsmodel.GotML
	case "OLE":
		return submissionsmodel.GotOL
	case "RE", "Runtime Error":
		return submissionsmodel.GotRE
	case "PE", "Presentation Error":
		return submissionsmodel.GotPE
	case "CE", "Compilation Error":
		return submissionsmodel.GotCE
	default:
		return submissionsmodel.GotIE
	}
}

func buildSubmissionTestDetails(results []TestResult, scored bool) *submissionsmodel.SubmissionTestDetails {
	if len(results) == 0 {
		return nil
	}

	testItems := make([]submissionsmodel.TestDetailItem, 0, len(results))
	var failedTestDetails *submissionsmodel.FailedTestDetail

	for _, result := range results {
		item := submissionsmodel.TestDetailItem{
			TestIndex: safeInt32(result.TestNumber),
			Verdict:   result.Verdict,
			TimeMs:    safeInt32(result.Time / 1_000_000),
			MemoryKb:  safeInt32(result.Memory / 1024),
			Sample:    result.Sample,
			Group:     result.Group,
		}
		if scored {
			item.Points = &result.Points
		}
		testItems = append(testItems, item)

		if !isOK(result.Verdict) && failedTestDetails == nil {
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

// Calculate grades awarded results. A pass-fail problem takes the verdict of the first failed test.
// A points problem is accepted at full score and partial above zero, as on Codeforces; at zero it takes the first failed test's verdict.
func (vc *VerdictCalculator) Calculate(results []TestResult) *FinalVerdict {
	byTest := make(map[int]TestResult, len(results))
	var maxTime, maxMemory int64
	for _, r := range results {
		byTest[r.TestNumber] = r
		maxTime = max(maxTime, r.Time)
		maxMemory = max(maxMemory, r.Memory)
	}
	scored := vc.problem.Grading == spec.GradingPoints
	verdict := &FinalVerdict{State: submissionsmodel.Accepted, MaxTime: safeInt32(maxTime / 1_000_000), MaxMemory: safeInt32(maxMemory / 1024 / 1024),
		TestDetails: buildSubmissionTestDetails(results, scored)}
	var failed *gfmt.Test
	for _, t := range vc.problem.Tests() {
		if r, ok := byTest[t.Test.Ordinal]; !ok || !isOK(r.Verdict) {
			failed = &t.Test
			break
		}
	}
	earned := gfmt.Full
	if scored {
		earned = func(t gfmt.Test) (points.Value, bool) {
			r, ok := byTest[t.Ordinal]
			return r.Points, ok && isOK(r.Verdict)
		}
	}
	score, groups := vc.problem.Grade(earned)
	max, _ := vc.problem.Grade(gfmt.Full)
	if verdict.TestDetails != nil {
		verdict.TestDetails.Groups = groupResults(groups, scored)
	}
	switch {
	case failed == nil || scored && max > 0 && score == max:
		verdict.Score, verdict.Message = max, "All tests passed"
	case scored && score > 0:
		verdict.State, verdict.Score, verdict.Message = submissionsmodel.Partial, score, "Score: "+score.String()
	default:
		n := failed.Ordinal
		verdict.FailedTest = &n
		verdict.State, verdict.Message = submissionsmodel.GotIE, fmt.Sprintf("Test %d was not judged", n)
		if r, ok := byTest[n]; ok {
			verdict.State, verdict.Message = MapSandboxVerdict(r.Verdict), fmt.Sprintf("Failed on test %d: %s", n, r.Message)
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
