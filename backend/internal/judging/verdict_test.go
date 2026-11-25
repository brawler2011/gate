package judging

import (
	"strings"
	"testing"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateStandardVerdict_WithTestDetails(t *testing.T) {
	vc := NewVerdictCalculator(testProblem())

	results := []TestResult{
		{
			TestNumber: 1,
			Verdict:    "OK",
			Time:       10_000_000,
			Memory:     2048 * 1024,
			Input:      "1 2\n",
			Output:     "3\n",
			Answer:     "3\n",
		},
		{
			TestNumber:    2,
			Verdict:       "WA",
			Time:          15_000_000,
			Memory:        3072 * 1024,
			Message:       "wrong answer: expected 5, got 4",
			Input:         "2 3\n",
			Output:        "4\n",
			Answer:        "5\n",
			CheckerOutput: "wrong answer: expected 5, got 4",
		},
	}

	verdict := vc.Calculate(award(testProblem(), results))
	require.NotNil(t, verdict)
	assert.Equal(t, submissionsmodel.GotWA, verdict.State)
	require.NotNil(t, verdict.FailedTest)
	assert.Equal(t, 2, *verdict.FailedTest)
	require.NotNil(t, verdict.TestDetails)
	assert.Len(t, verdict.TestDetails.Tests, 2)
	assert.Equal(t, int32(1), verdict.TestDetails.Tests[0].TestIndex)
	assert.Equal(t, "OK", verdict.TestDetails.Tests[0].Verdict)
	assert.Equal(t, int32(10), verdict.TestDetails.Tests[0].TimeMs)
	assert.Equal(t, int32(2048), verdict.TestDetails.Tests[0].MemoryKb)

	require.NotNil(t, verdict.TestDetails.FailedTestDetails)
	assert.Equal(t, int32(2), verdict.TestDetails.FailedTestDetails.TestIndex)
	assert.Equal(t, "2 3\n", verdict.TestDetails.FailedTestDetails.Input)
	assert.Equal(t, "4\n", verdict.TestDetails.FailedTestDetails.Output)
	assert.Equal(t, "5\n", verdict.TestDetails.FailedTestDetails.Answer)
	assert.Equal(t, "wrong answer: expected 5, got 4", verdict.TestDetails.FailedTestDetails.CheckerOutput)
	assert.False(t, verdict.TestDetails.FailedTestDetails.IsTruncated)
}

func TestCalculateStandardVerdict_TruncatesLargeInput(t *testing.T) {
	vc := NewVerdictCalculator(testProblem())

	largeInput := strings.Repeat("A", 200*1024)
	results := []TestResult{
		{
			TestNumber: 1,
			Verdict:    "WA",
			Time:       10_000_000,
			Memory:     2048 * 1024,
			Input:      largeInput,
			Output:     "output",
			Answer:     "answer",
		},
	}

	verdict := vc.Calculate(award(testProblem(), results))
	require.NotNil(t, verdict)
	require.NotNil(t, verdict.TestDetails)
	require.NotNil(t, verdict.TestDetails.FailedTestDetails)
	assert.True(t, verdict.TestDetails.FailedTestDetails.IsTruncated)
	assert.Len(t, verdict.TestDetails.FailedTestDetails.Input, maxTestDetailSize)
}

func TestVerdictCalculation(t *testing.T) {
	t.Run("StandardVerdict_AllPassed", func(t *testing.T) {
		calculator := NewVerdictCalculator(testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "OK", Time: 2000000, Memory: 2048 * 1024},
		}

		verdict := calculator.Calculate(award(testProblem(), results))
		assert.Equal(t, submissionsmodel.Accepted, verdict.State)
		assert.Equal(t, 100*points.Unit, verdict.Score)
		assert.Equal(t, int32(2), verdict.MaxTime)
	})

	t.Run("StandardVerdict_WrongAnswer", func(t *testing.T) {
		calculator := NewVerdictCalculator(testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "WA", Time: 2000000, Memory: 2048 * 1024, Message: "Wrong answer"},
		}

		verdict := calculator.Calculate(award(testProblem(), results))
		assert.Equal(t, submissionsmodel.GotWA, verdict.State)
		assert.Equal(t, points.Value(0), verdict.Score)
		assert.NotNil(t, verdict.FailedTest)
		assert.Equal(t, 2, *verdict.FailedTest)
	})

	t.Run("StandardVerdict_TimeLimitExceeded", func(t *testing.T) {
		calculator := NewVerdictCalculator(testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "TLE", Time: 10000000000, Memory: 1024 * 1024, Message: "Time limit exceeded"},
		}

		verdict := calculator.Calculate(award(testProblem(), results))
		assert.Equal(t, submissionsmodel.GotTL, verdict.State)
	})

	t.Run("StandardVerdict_InternalError", func(t *testing.T) {
		calculator := NewVerdictCalculator(testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "IE", Message: "Internal error reading test"},
		}

		verdict := calculator.Calculate(award(testProblem(), results))
		assert.Equal(t, submissionsmodel.GotIE, verdict.State)
		assert.Equal(t, points.Value(0), verdict.Score)
		assert.NotNil(t, verdict.FailedTest)
		assert.Equal(t, 1, *verdict.FailedTest)
	})

	t.Run("ScoringVerdict_EachAndCompleteWithDependency", func(t *testing.T) {
		calculator := NewVerdictCalculator(testProblemWithGroups())
		verdict := calculator.Calculate(award(testProblemWithGroups(), []TestResult{
			{TestNumber: 1, Verdict: "OK"}, {TestNumber: 2, Verdict: "WA"},
			{TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "OK"},
		}))
		assert.Equal(t, submissionsmodel.Partial, verdict.State)
		assert.Nil(t, verdict.FailedTest)
		// A failed dependency blocks group points even when that group's own tests pass.
		assert.Equal(t, points.Value(3333), verdict.Score)
		groups := verdict.TestDetails.Groups
		assert.Equal(t, []points.Value{3333, 3333, 0}, []points.Value{*groups[0].Score, *groups[1].Score, *groups[2].Score})

		verdict = calculator.Calculate(award(testProblemWithGroups(), []TestResult{
			{TestNumber: 1, Verdict: "OK"}, {TestNumber: 2, Verdict: "OK"},
			{TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "WA"},
		}))
		// The minimum of group2 is nothing when one test fails.
		assert.Equal(t, points.Value(6666), verdict.Score)

		verdict = calculator.Calculate(award(testProblemWithGroups(), []TestResult{
			{TestNumber: 1, Verdict: "OK"}, {TestNumber: 2, Verdict: "OK"},
			{TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "OK"},
		}))
		assert.Equal(t, submissionsmodel.Accepted, verdict.State)
		assert.Equal(t, points.Value(9666), verdict.Score)
	})
}

// Groups take their tests and subgroups in the order of the first tests; stop_on_failure drops what follows a failure.
func TestScoringNestedGroups(t *testing.T) {
	test := func(ordinal int, score points.Value) gfmt.Test {
		return gfmt.Test{Ordinal: ordinal, Points: score * points.Unit}
	}
	p := &gfmt.Problem{Type: "standard", Grading: "points", Testset: gfmt.Group{Groups: []gfmt.Group{
		{Name: "a", StopOnFailure: true, Tests: []gfmt.Test{test(1, 10), test(2, 10), test(5, 10)},
			Groups: []gfmt.Group{{Name: "b", Aggregate: "min", Tests: []gfmt.Test{test(3, 5), test(4, 5)}}}},
		{Name: "c", Aggregate: "avg", Tests: []gfmt.Test{test(6, 10), test(7, 20)}},
		{Name: "empty", Aggregate: "max"},
	}}}
	verdicts := map[int]string{1: "OK", 2: "OK", 3: "WA", 4: "OK", 5: "OK", 6: "OK", 7: "TLE"}
	var results []TestResult
	for ordinal := 1; ordinal <= 7; ordinal++ {
		results = append(results, TestResult{TestNumber: ordinal, Verdict: verdicts[ordinal]})
	}
	verdict := NewVerdictCalculator(p).Calculate(award(p, results))
	assert.Equal(t, submissionsmodel.Partial, verdict.State)
	var report []string
	for _, g := range verdict.TestDetails.Groups {
		report = append(report, g.Name+" "+g.Score.String()+"/"+g.Points.String())
	}
	// a stops at b before test 5; c averages 10 and 0; the empty group neither scores nor counts.
	assert.Equal(t, []string{" 25.00/50.00", "a 20.00/35.00", "b 0.00/5.00", "c 5.00/15.00"}, report)
	assert.Equal(t, 25*points.Unit, verdict.Score)
}

// Leaving out results after a failure never lifts an average above its maximum.
func TestScoringStopKeepsMaximum(t *testing.T) {
	p := &gfmt.Problem{Type: "standard", Grading: "points", Testset: gfmt.Group{Aggregate: "avg", StopOnFailure: true, Tests: []gfmt.Test{
		{Ordinal: 1, Points: 100 * points.Unit}, {Ordinal: 2, Points: points.Unit}, {Ordinal: 3, Points: points.Unit},
	}}}
	verdict := NewVerdictCalculator(p).Calculate(award(p, []TestResult{{TestNumber: 1, Verdict: "OK"}, {TestNumber: 2, Verdict: "WA"}, {TestNumber: 3, Verdict: "OK"}}))
	assert.Equal(t, points.Value(3400), verdict.Score)
}

func TestPretestsScoreOnlyPretests(t *testing.T) {
	p := testProblemWithGroups()
	for g := range p.Testset.All() {
		for i := range g.Tests {
			g.Tests[i].Pretest = g.Tests[i].Ordinal == 1
		}
	}
	restricted, ok := p.Pretests()
	require.True(t, ok)
	require.Len(t, restricted.Testset.Groups, 1, "group2 has no pretests")
	require.Equal(t, []gfmt.GroupTest{{Group: &restricted.Testset.Groups[0], Test: p.Testset.Groups[0].Tests[0]}}, restricted.Tests())
	verdict := NewVerdictCalculator(restricted).Calculate(award(restricted, []TestResult{{TestNumber: 1, Verdict: "OK"}}))
	assert.Equal(t, submissionsmodel.Accepted, verdict.State)
	assert.Equal(t, points.Value(3333), verdict.Score)

	for g := range p.Testset.All() {
		for i := range g.Tests {
			g.Tests[i].Pretest = true
		}
	}
	_, ok = p.Pretests()
	require.False(t, ok, "pretests covering every test judge like a full run")
}

func testProblem() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "standard", Grading: "pass-fail",
		Testset: gfmt.Group{Groups: []gfmt.Group{{Name: "samples", Tests: []gfmt.Test{
			{Ordinal: 1, Points: 40 * points.Unit, Input: "01.in"},
			{Ordinal: 2, Points: 60 * points.Unit, Input: "02.in"},
		}}}},
	}
}

// group2 scores 30 only when it and group1 pass completely.
func testProblemWithGroups() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "standard", Grading: "points",
		Testset: gfmt.Group{Groups: []gfmt.Group{
			{Name: "group1", Tests: []gfmt.Test{{Ordinal: 1, Points: 3333, Input: "01.in"}, {Ordinal: 2, Points: 3333, Input: "02.in"}}},
			{Name: "group2", Aggregate: "min", Requires: []string{"group1"}, Tests: []gfmt.Test{{Ordinal: 3, Points: 3000, Input: "03.in"}, {Ordinal: 4, Points: 3000, Input: "04.in"}}},
		}},
	}
}

// award settles results as the judge does before grading.
func award(p *gfmt.Problem, results []TestResult) []TestResult {
	byOrdinal := map[int]gfmt.Test{}
	for _, t := range p.Tests() {
		byOrdinal[t.Test.Ordinal] = t.Test
	}
	for i := range results {
		results[i].Award(byOrdinal[results[i].TestNumber].Points, p.Grading)
	}
	return results
}

func TestAwardTakesJuryPointsAsPolygon(t *testing.T) {
	score := func(f float64) *float64 { return &f }
	for name, c := range map[string]struct {
		grading string
		score   *float64
		verdict string
		points  points.Value
	}{
		"partial":           {"points", score(2.5), "POINTS", 250},
		"all points pass":   {"points", score(4), "OK", 400},
		"zero points":       {"points", score(0), "POINTS", 0},
		"above the test":    {"points", score(4.01), "FAIL", 0},
		"negative":          {"points", score(-1), "FAIL", 0},
		"no number":         {"points", nil, "FAIL", 0},
		"pass-fail problem": {"pass-fail", score(2), "FAIL", 0},
	} {
		t.Run(name, func(t *testing.T) {
			r := TestResult{Verdict: "POINTS", Score: c.score}
			r.Award(4*points.Unit, c.grading)
			assert.Equal(t, c.verdict, r.Verdict)
			assert.Equal(t, c.points, r.Points)
		})
	}
}

// Jury points count in groups like any score; a test short of its points has failed for requirements and stops.
func TestPartialPointsGradeLikeCodeforces(t *testing.T) {
	p := testProblemWithGroups()
	partial := func(f float64) TestResult { return TestResult{Verdict: "POINTS", Score: &f} }
	results := []TestResult{{TestNumber: 1, Verdict: "OK"}, partial(10), {TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "OK"}}
	results[1].TestNumber = 2
	verdict := NewVerdictCalculator(p).Calculate(award(p, results))
	assert.Equal(t, submissionsmodel.Partial, verdict.State)
	assert.Nil(t, verdict.FailedTest, "a partial result names no test")
	// group1 sums 33.33 and 10; group2 requires group1, which test 2 failed.
	assert.Equal(t, points.Value(4333), verdict.Score)
	assert.Equal(t, points.Value(1000), *verdict.TestDetails.Tests[1].Points)
	assert.Equal(t, "POINTS", verdict.TestDetails.Tests[1].Verdict)

	results = award(p, []TestResult{{TestNumber: 1, Verdict: "WA"}, {TestNumber: 2, Verdict: "TLE"}, {TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "OK"}})
	verdict = NewVerdictCalculator(p).Calculate(results)
	assert.Equal(t, submissionsmodel.GotWA, verdict.State, "nothing scored, so the first failure decides")
	require.NotNil(t, verdict.FailedTest)
	assert.Equal(t, 1, *verdict.FailedTest)

	// The best of two tests reaches full score with one of them failed.
	best := &gfmt.Problem{Grading: "points", Testset: gfmt.Group{Aggregate: "max", Tests: []gfmt.Test{{Ordinal: 1, Points: points.Unit}, {Ordinal: 2, Points: points.Unit}}}}
	verdict = NewVerdictCalculator(best).Calculate(award(best, []TestResult{{TestNumber: 1, Verdict: "WA"}, {TestNumber: 2, Verdict: "OK"}}))
	assert.Equal(t, submissionsmodel.Accepted, verdict.State)
	assert.Equal(t, points.Unit, verdict.Score)
}
