package judging

import (
	"slices"
	"strings"
	"testing"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateStandardVerdict_WithTestDetails(t *testing.T) {
	vc := NewVerdictCalculator("standard", &gfmt.Problem{})

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

	verdict := vc.CalculateStandardVerdict(results)
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
	vc := NewVerdictCalculator("standard", &gfmt.Problem{})

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

	verdict := vc.CalculateStandardVerdict(results)
	require.NotNil(t, verdict)
	require.NotNil(t, verdict.TestDetails)
	require.NotNil(t, verdict.TestDetails.FailedTestDetails)
	assert.True(t, verdict.TestDetails.FailedTestDetails.IsTruncated)
	assert.Len(t, verdict.TestDetails.FailedTestDetails.Input, maxTestDetailSize)
}

func TestVerdictCalculation(t *testing.T) {
	t.Run("StandardVerdict_AllPassed", func(t *testing.T) {
		calculator := NewVerdictCalculator("pass-fail", testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "OK", Time: 2000000, Memory: 2048 * 1024},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, submissionsmodel.Accepted, verdict.State)
		assert.Equal(t, 100*points.Unit, verdict.Score)
		assert.Equal(t, int32(2), verdict.MaxTime)
	})

	t.Run("StandardVerdict_WrongAnswer", func(t *testing.T) {
		calculator := NewVerdictCalculator("pass-fail", testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "OK", Time: 1000000, Memory: 1024 * 1024},
			{TestNumber: 2, Verdict: "WA", Time: 2000000, Memory: 2048 * 1024, Message: "Wrong answer"},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, submissionsmodel.GotWA, verdict.State)
		assert.Equal(t, points.Value(0), verdict.Score)
		assert.NotNil(t, verdict.FailedTest)
		assert.Equal(t, 2, *verdict.FailedTest)
	})

	t.Run("StandardVerdict_TimeLimitExceeded", func(t *testing.T) {
		calculator := NewVerdictCalculator("pass-fail", testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "TLE", Time: 10000000000, Memory: 1024 * 1024, Message: "Time limit exceeded"},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, submissionsmodel.GotTL, verdict.State)
	})

	t.Run("StandardVerdict_InternalError", func(t *testing.T) {
		calculator := NewVerdictCalculator("pass-fail", testProblem())

		results := []TestResult{
			{TestNumber: 1, Verdict: "IE", Message: "Internal error reading test"},
		}

		verdict := calculator.Calculate(results)
		assert.Equal(t, submissionsmodel.GotIE, verdict.State)
		assert.Equal(t, points.Value(0), verdict.Score)
		assert.NotNil(t, verdict.FailedTest)
		assert.Equal(t, 1, *verdict.FailedTest)
	})

	t.Run("ScoringVerdict_EachAndCompleteWithDependency", func(t *testing.T) {
		calculator := NewVerdictCalculator("scoring", testProblemWithGroups())
		verdict := calculator.Calculate([]TestResult{
			{TestNumber: 1, Verdict: "OK"}, {TestNumber: 2, Verdict: "WA"},
			{TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "OK"},
		})
		// A partial solution reports the first failing test but keeps its points.
		assert.Equal(t, submissionsmodel.GotWA, verdict.State)
		require.NotNil(t, verdict.FailedTest)
		assert.Equal(t, 2, *verdict.FailedTest)
		// A failed dependency blocks group points even when that group's own tests pass.
		assert.Equal(t, points.Value(3333), verdict.Score)
		assert.Equal(t, []points.Value{3333, 0}, []points.Value{*verdict.TestDetails.Subtasks[0].Score, *verdict.TestDetails.Subtasks[1].Score})

		verdict = calculator.Calculate([]TestResult{
			{TestNumber: 1, Verdict: "OK"}, {TestNumber: 2, Verdict: "OK"},
			{TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "WA"},
		})
		// "complete" group2 awards nothing when one test fails; weights are not rescaled.
		assert.Equal(t, points.Value(6666), verdict.Score)

		verdict = calculator.Calculate([]TestResult{
			{TestNumber: 1, Verdict: "OK"}, {TestNumber: 2, Verdict: "OK"},
			{TestNumber: 3, Verdict: "OK"}, {TestNumber: 4, Verdict: "OK"},
		})
		assert.Equal(t, submissionsmodel.Accepted, verdict.State)
		assert.Equal(t, points.Value(9666), verdict.Score)
	})
}

func TestPretestsScoreOnlyPretests(t *testing.T) {
	p := testProblemWithGroups()
	for name, ordinals := range map[string][]int{"group1": {1}, "group2": {}} {
		s := p.Subtasks[name]
		for i := range s.Tests {
			s.Tests[i].Pretest = slices.Contains(ordinals, s.Tests[i].Ordinal)
		}
		p.Subtasks[name] = s
	}
	restricted, ok := p.Pretests()
	require.True(t, ok)
	require.Equal(t, []FlatTest{{SubtaskName: "group1", TestIndex: 1, Test: p.Subtasks["group1"].Tests[0]}}, collectFlatTests(restricted))
	verdict := NewVerdictCalculator("scoring", restricted).Calculate([]TestResult{{TestNumber: 1, Verdict: "OK"}})
	assert.Equal(t, submissionsmodel.Accepted, verdict.State)
	assert.Equal(t, points.Value(3333), verdict.Score)

	for name, s := range p.Subtasks {
		for i := range s.Tests {
			s.Tests[i].Pretest = true
		}
		p.Subtasks[name] = s
	}
	_, ok = p.Pretests()
	require.False(t, ok, "pretests covering every test judge like a full run")
}

func testProblem() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "pass-fail",
		Subtasks: map[string]gfmt.Subtask{
			"samples": {
				Policy: "complete",
				Tests: []gfmt.Test{
					{Ordinal: 1, Points: 40 * points.Unit, Input: "01.in"},
					{Ordinal: 2, Points: 60 * points.Unit, Input: "02.in"},
				},
			},
		},
	}
}

func testProblemWithGroups() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "scoring",
		Subtasks: map[string]gfmt.Subtask{
			"group1": {
				Policy: "each",
				Tests: []gfmt.Test{
					{Ordinal: 1, Points: 3333, Input: "01.in"},
					{Ordinal: 2, Points: 3333, Input: "02.in"},
				},
			},
			"group2": {
				Policy:       "complete",
				Dependencies: []string{"group1"},
				Tests: []gfmt.Test{
					{Ordinal: 3, Points: 1000, Input: "03.in"},
					{Ordinal: 4, Points: 2000, Input: "04.in"},
				},
			},
		},
	}
}
