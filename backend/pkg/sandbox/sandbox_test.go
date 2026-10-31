package sandbox

import "testing"

func TestExtractScore(t *testing.T) {
	tests := []struct {
		input    string
		expected *float64
	}{
		{
			input:    "points 0.833 Your solution is 83.3% optimal",
			expected: func(val float64) *float64 { return &val }(0.833),
		},
		{
			input:    "points 0.5\nSome other message",
			expected: func(val float64) *float64 { return &val }(0.5),
		},
		{
			input:    "0.833",
			expected: func(val float64) *float64 { return &val }(0.833),
		},
		{
			input:    "0.833 Your solution is 83.3% optimal",
			expected: func(val float64) *float64 { return &val }(0.833),
		},
		{
			input:    "Wrong Answer on test 1",
			expected: nil,
		},
	}

	for _, tc := range tests {
		res := parseScore(tc.input)
		if tc.expected == nil {
			if res != nil {
				t.Errorf("expected nil for input %q, got %f", tc.input, *res)
			}
		} else {
			if res == nil {
				t.Errorf("expected %f for input %q, got nil", *tc.expected, tc.input)
			} else if *res != *tc.expected {
				t.Errorf("expected %f for input %q, got %f", *tc.expected, tc.input, *res)
			}
		}
	}
}
