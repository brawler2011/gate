package problemsmodel

import (
	"testing"

	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/stretchr/testify/require"
)

func TestCheckerTestCheck(t *testing.T) {
	half, quarter := points.Value(50), 0.25
	for _, c := range []struct {
		name     string
		verdict  string
		points   *points.Value
		got      string
		score    *float64
		mismatch string
	}{
		{"verdict matches", "WA", nil, "WA", nil, ""},
		{"verdict differs", "WA", nil, "OK", nil, "OK, expected WA"},
		{"any points", "POINTS", nil, "POINTS", &quarter, ""},
		{"points match", "POINTS", &half, "POINTS", new(0.5), ""},
		{"points differ", "POINTS", &half, "POINTS", &quarter, "0.25 points, expected 0.50 points"},
		{"no points printed", "POINTS", &half, "POINTS", nil, "no points, expected 0.50 points"},
	} {
		t.Run(c.name, func(t *testing.T) {
			test := DraftCheckerTest{Verdict: c.verdict, Points: c.points}
			require.Equal(t, c.mismatch, test.Check(c.got, c.score).Mismatch)
		})
	}
}
