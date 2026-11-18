package judging

import (
	"strings"
	"testing"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The catalog's error_line patterns against real compiler and runtime output.
func TestParseErrorLine(t *testing.T) {
	catalog, err := sandbox.LoadConfig("../../../judge/langs.toml")
	require.NoError(t, err)
	tests := []struct {
		name     string
		language string
		text     string
		expected *int32
	}{
		{"C gcc error", "c", "foo.c: In function 'main':\nfoo.c:4:5: error: expected ';' before 'return'", ptrInt32(4)},
		{"C++ gcc error", "cpp17", "foo.cc: In function 'int main()':\nfoo.cc:15:23: error: expected ';' before '}' token", ptrInt32(15)},
		{"Python traceback", "python3", "Traceback (most recent call last):\n  File \"/w/foo.py\", line 12, in <module>\n    print(1/0)\nZeroDivisionError: division by zero", ptrInt32(12)},
		{"PyPy syntax error", "pypy3", "  File \"foo.py\", line 7\n    def test(\n             ^\nSyntaxError: unexpected EOF while parsing", ptrInt32(7)},
		{"Go compiler error", "go", "# command-line-arguments\n./foo.go:28:2: undefined: fmt.Printlnn", ptrInt32(28)},
		{"Java compiler error", "java", "Main.java:5: error: ';' expected\n        int x = 1\n                 ^\n1 error", ptrInt32(5)},
		{"Java exception", "java", "Exception in thread \"main\" java.lang.ArithmeticException: / by zero\n\tat Main.main(Main.java:9)", ptrInt32(9)},
		{"Rust compiler error", "rust", "error[E0425]: cannot find value `y` in this scope\n --> foo.rs:3:13\n  |\n3 |     let x = y;", ptrInt32(3)},
		{"Node.js exception", "node", "/w/foo.js:6\n    throw new Error('boom');\n    ^\n\nError: boom\n    at Object.<anonymous> (/w/foo.js:6:11)", ptrInt32(6)},
		{"Generic line fallback", "cpp17", "Runtime error at line 99", ptrInt32(99)},
		{"Empty output", "cpp17", "", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lang, ok := catalog.Langs[tc.language]
			require.True(t, ok)
			result := ParseErrorLine(lang.ErrorLine, tc.text)
			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tc.expected, *result)
			}
		})
	}
}

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

	largeInput := strings.Repeat("A", 200*1024) // 200 KB
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

func ptrInt32(v int32) *int32 {
	return &v
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
		assert.Equal(t, int32(2), verdict.MaxTime) // 2ms
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
		// Only the passed test of "each" group1 counts; group2 is blocked by the
		// failed dependency even though its own tests passed.
		assert.Equal(t, points.Value(3333), verdict.Score)

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

func testProblem() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "pass-fail",
		Subtasks: map[string]gfmt.Subtask{
			"samples": {
				Policy: "complete",
				Tests: []gfmt.Test{
					{Ordinal: 1, Points: 40 * points.Unit, Manual: "01.in"},
					{Ordinal: 2, Points: 60 * points.Unit, Manual: "02.in"},
				},
			},
		},
	}
}

// testProblemWithGroups: group1 awards each passed test; group2 awards its
// weights only when complete and only if group1 is fully passed.
func testProblemWithGroups() *gfmt.Problem {
	return &gfmt.Problem{
		Type: "scoring",
		Subtasks: map[string]gfmt.Subtask{
			"group1": {
				Policy: "each",
				Tests: []gfmt.Test{
					{Ordinal: 1, Points: 3333, Manual: "01.in"},
					{Ordinal: 2, Points: 3333, Manual: "02.in"},
				},
			},
			"group2": {
				Policy:       "complete",
				Dependencies: []string{"group1"},
				Tests: []gfmt.Test{
					{Ordinal: 3, Points: 1000, Manual: "03.in"},
					{Ordinal: 4, Points: 2000, Manual: "04.in"},
				},
			},
		},
	}
}
