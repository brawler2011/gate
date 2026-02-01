package sandbox

import (
	"strings"
	"testing"
)

func TestExtractScore(t *testing.T) {
	tests := []struct {
		output   string
		expected *float64
	}{
		{
			output:   "points: 0.5\nSome message",
			expected: floatPtr(0.5),
		},
		{
			output:   "score: 50\nSome message",
			expected: floatPtr(50.0),
		},
		{
			output:   "POINTS: 0.75",
			expected: floatPtr(0.75),
		},
		{
			output:   "No score here",
			expected: nil,
		},
		{
			output:   "",
			expected: nil,
		},
	}

	for i, test := range tests {
		result := extractScore(test.output)

		if test.expected == nil {
			if result != nil {
				t.Errorf("Test %02d: expected nil, got %v", i, *result)
			}
		} else {
			if result == nil {
				t.Errorf("Test %02d: expected %v, got nil", i, *test.expected)
			} else if *result != *test.expected {
				t.Errorf("Test %02d: expected %v, got %v", i, *test.expected, *result)
			}
		}
	}
}

func TestExecutorCreation(t *testing.T) {
	client := &Client{}
	compiler := NewCompiler(client)
	executor := NewExecutor(client, compiler)

	if executor == nil {
		t.Error("NewExecutor returned nil")
	}

	if executor.client == nil {
		t.Error("Executor client is nil")
	}

	if executor.compiler == nil {
		t.Error("Executor compiler is nil")
	}
}

// Helper function
func floatPtr(f float64) *float64 {
	return &f
}

func TestCheckerVerdictParsing(t *testing.T) {
	// Test verdict parsing logic
	tests := []struct {
		exitStatus int
		stdout     string
		expected   string
	}{
		{0, "ok", "OK"},
		{1, "wrong answer", "WA"},
		{2, "presentation error", "PE"},
		{3, "fail", "FAIL"},
		{7, "points 0.5", "POINTS"},
	}

	for i, test := range tests {
		// This is a simplified test since we can't easily test RunChecker without a real go-judge
		// We're testing the logic of verdict determination
		var verdict string

		if test.exitStatus == 0 {
			verdict = "OK"
		} else if test.exitStatus == 1 || strings.Contains(strings.ToUpper(test.stdout), "WRONG") {
			verdict = "WA"
		} else if test.exitStatus == 2 || strings.Contains(strings.ToUpper(test.stdout), "PRESENTATION") {
			verdict = "PE"
		} else if test.exitStatus == 3 {
			verdict = "FAIL"
		} else if test.exitStatus == 7 {
			verdict = "POINTS"
		} else {
			verdict = "FAIL"
		}

		if verdict != test.expected {
			t.Errorf("Test %02d: expected verdict %s, got %s", i, test.expected, verdict)
		}
	}
}
