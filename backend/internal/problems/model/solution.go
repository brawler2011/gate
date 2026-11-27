package problemsmodel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/points"
)

// SolutionVerdicts are the test verdicts a build can give a solution.
var SolutionVerdicts = []string{"OK", "WA", "PE", "TLE", "MLE", "OLE", "RE", "FAIL", "POINTS", "CE"}

// DraftSolution is expected, as in Polygon, to get a verdict from Verdicts or OK on every test and,
// unless Verdicts has OK, to fail at least one test; CE expects a compilation error instead. Empty Verdicts keep it out of the build.
type DraftSolution struct {
	DraftNamedProgram
	Verdicts []string      `json:"verdicts"`
	Points   *points.Value `json:"points"`
}

func (s *DraftSolution) Runs() bool { return len(s.Verdicts) > 0 }

// MustPass and MustTimeOut name the solutions whose run times a time rule relates to the limit.
func (s *DraftSolution) MustPass() bool    { return slices.Equal(s.Verdicts, []string{"OK"}) }
func (s *DraftSolution) MustTimeOut() bool { return slices.Equal(s.Verdicts, []string{"TLE"}) }

type TestVerdict struct {
	Ordinal int
	Verdict string
}

// Mismatch explains how the verdicts of all tests, in test order, and the score miss the expectation; empty when they match.
func (s *DraftSolution) Mismatch(compiled bool, tests []TestVerdict, score *points.Value) string {
	if !compiled {
		if slices.Contains(s.Verdicts, "CE") {
			return ""
		}
		return "compilation error, expected " + s.expected()
	}
	failed := false
	for _, t := range tests {
		if t.Verdict == "OK" {
			continue
		}
		failed = true
		if !slices.Contains(s.Verdicts, t.Verdict) {
			return fmt.Sprintf("test %d: %s, expected %s", t.Ordinal, t.Verdict, s.expected())
		}
	}
	if !failed && !slices.Contains(s.Verdicts, "OK") {
		return "passed every test, expected " + s.expected()
	}
	if s.Points != nil && score != nil && *score != *s.Points {
		return fmt.Sprintf("scored %s, expected %s", score, s.Points)
	}
	return ""
}

func (s *DraftSolution) expected() string {
	return strings.Join(s.Verdicts, " or ")
}

// SolutionResult is what a build saw of a solution; Test is the first test whose verdict is not OK.
type SolutionResult struct {
	Name     string        `json:"name"`
	Verdict  string        `json:"verdict"`
	Test     *int          `json:"test"`
	TimeMs   int64         `json:"time_ms"`
	Points   *points.Value `json:"points"`
	Mismatch string        `json:"mismatch"`
}
