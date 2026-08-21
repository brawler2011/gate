package judge

import (
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseErrorLine(t *testing.T) {
	tests := []struct {
		name     string
		lang     models.LanguageName
		text     string
		expected *int32
	}{
		{
			name:     "C++ gcc error",
			lang:     models.Cpp,
			text:     "foo.cc:15:23: error: expected ';' before '}' token",
			expected: ptrInt32(15),
		},
		{
			name:     "C++ clang error",
			lang:     models.Cpp,
			text:     "solution.cpp:42:5: fatal error: 'iostream' file not found",
			expected: ptrInt32(42),
		},
		{
			name:     "Python traceback",
			lang:     models.Python,
			text:     "Traceback (most recent call last):\n  File \"foo.py\", line 12, in <module>\n    print(1/0)\nZeroDivisionError: division by zero",
			expected: ptrInt32(12),
		},
		{
			name:     "Python syntax error",
			lang:     models.Python,
			text:     "  File \"foo.py\", line 7\n    def test(\n             ^\nSyntaxError: unexpected EOF while parsing",
			expected: ptrInt32(7),
		},
		{
			name:     "Go compiler error",
			lang:     models.Golang,
			text:     "foo.go:28:2: undefined: fmt.Printlnn",
			expected: ptrInt32(28),
		},
		{
			name:     "Generic line fallback",
			lang:     models.Cpp,
			text:     "Runtime error at line 99",
			expected: ptrInt32(99),
		},
		{
			name:     "Empty output",
			lang:     models.Cpp,
			text:     "",
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseErrorLine(tc.lang, tc.text)
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
	assert.Equal(t, models.GotWA, verdict.State)
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
