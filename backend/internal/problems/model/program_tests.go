package problemsmodel

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/google/uuid"
)

// CheckerVerdicts are the verdicts a checker test can expect.
var CheckerVerdicts = []string{"OK", "WA", "PE", "FAIL", "POINTS"}

// DraftCheckerTest expects the checker to judge Output against Answer for Input with Verdict, and Points when set.
type DraftCheckerTest struct {
	ID      uuid.UUID     `json:"id"`
	Input   uuid.UUID     `json:"input"`
	Output  uuid.UUID     `json:"output"`
	Answer  uuid.UUID     `json:"answer"`
	Verdict string        `json:"verdict"`
	Points  *points.Value `json:"points"`
}

// DraftValidatorTest expects the validator to accept Input exactly when Valid; a non-empty Group is passed as --group.
type DraftValidatorTest struct {
	ID    uuid.UUID `json:"id"`
	Input uuid.UUID `json:"input"`
	Valid bool      `json:"valid"`
	Group string    `json:"group"`
}

// ProgramTestResult is what a build saw on a checker or validator test; Verdict is "valid" or "invalid" for validators.
type ProgramTestResult struct {
	ID       uuid.UUID     `json:"id"`
	Verdict  string        `json:"verdict"`
	Points   *points.Value `json:"points"`
	Mismatch string        `json:"mismatch"`
}

// Check fills the result of a checker run; score is the checker's own number for POINTS.
func (t *DraftCheckerTest) Check(verdict string, score *float64) ProgramTestResult {
	result := ProgramTestResult{ID: t.ID, Verdict: verdict}
	if score != nil {
		value := points.Value(math.Round(*score * 100))
		result.Points = &value
	}
	switch {
	case verdict != t.Verdict:
		result.Mismatch = fmt.Sprintf("%s, expected %s", verdict, t.Verdict)
	case t.Points != nil && (result.Points == nil || *result.Points != *t.Points):
		got := "no points"
		if result.Points != nil {
			got = result.Points.String() + " points"
		}
		result.Mismatch = fmt.Sprintf("%s, expected %s points", got, t.Points)
	}
	return result
}

func (t *DraftValidatorTest) Check(valid bool) ProgramTestResult {
	verdict := map[bool]string{true: "valid", false: "invalid"}
	result := ProgramTestResult{ID: t.ID, Verdict: verdict[valid]}
	if valid != t.Valid {
		result.Mismatch = fmt.Sprintf("%s, expected %s", verdict[valid], verdict[t.Valid])
	}
	return result
}

func (d *Draft) validateProgramTests(v *validation, q DraftQuotas) {
	ids := map[uuid.UUID]bool{}
	count := 0
	id := func(at string, test uuid.UUID) {
		if test == uuid.Nil || ids[test] {
			v.fail(at+"/id", "ID is missing or not unique")
		}
		ids[test] = true
		count++
	}
	object := func(at string, oid uuid.UUID) {
		if oid == uuid.Nil {
			v.fail(at, "file has no content")
		}
	}
	if d.Checker != nil {
		for i, t := range d.Checker.Tests {
			at := fmt.Sprintf("/checker/tests/%d", i)
			id(at, t.ID)
			object(at+"/input", t.Input)
			object(at+"/output", t.Output)
			object(at+"/answer", t.Answer)
			if !slices.Contains(CheckerVerdicts, t.Verdict) {
				v.fail(at+"/verdict", "invalid verdict %q", t.Verdict)
			}
			if t.Points != nil && (t.Verdict != "POINTS" || *t.Points < 0 || *t.Points > points.Max) {
				v.fail(at+"/points", "expected points need the POINTS verdict and a valid score")
			}
		}
	}
	for i, validator := range d.Validators {
		for j, t := range validator.Tests {
			at := fmt.Sprintf("/validators/%d/tests/%d", i, j)
			id(at, t.ID)
			object(at+"/input", t.Input)
			if len(t.Group) > 255 || !utf8.ValidString(t.Group) || strings.ContainsRune(t.Group, 0) {
				v.fail(at+"/group", "invalid group %q", t.Group)
			}
		}
	}
	if count > q.Tests {
		v.fail("", "more than %d checker and validator tests", q.Tests)
	}
}
