package problemsmodel

import (
	"testing"

	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/stretchr/testify/require"
)

func TestSolutionMismatch(t *testing.T) {
	forty, thirty := points.Value(4000), points.Value(3000)
	ok, wa, tl := TestVerdict{1, "OK"}, TestVerdict{2, "WA"}, TestVerdict{3, "TLE"}
	for _, c := range []struct {
		name     string
		verdicts []string
		points   *points.Value
		compiled bool
		tests    []TestVerdict
		score    *points.Value
		mismatch string
	}{
		{"accepted", []string{"OK"}, nil, true, []TestVerdict{ok, ok}, nil, ""},
		{"accepted fails", []string{"OK"}, nil, true, []TestVerdict{ok, wa}, nil, "test 2: WA, expected OK"},
		{"wrong answer passes", []string{"WA"}, nil, true, []TestVerdict{ok, ok}, nil, "passed every test, expected WA"},
		{"wrong answer gets TL", []string{"WA"}, nil, true, []TestVerdict{wa, tl}, nil, "test 3: TLE, expected WA"},
		{"TL or OK accepted", []string{"TLE", "OK"}, nil, true, []TestVerdict{ok}, nil, ""},
		{"TL or OK exceeds", []string{"TLE", "OK"}, nil, true, []TestVerdict{ok, tl}, nil, ""},
		{"points match", []string{"WA"}, &forty, true, []TestVerdict{ok, wa}, &forty, ""},
		{"points differ", []string{"WA"}, &forty, true, []TestVerdict{ok, wa}, &thirty, "scored 30.00, expected 40.00"},
		{"compilation error expected", []string{"CE"}, nil, false, nil, nil, ""},
		{"compilation error", []string{"OK"}, nil, false, nil, nil, "compilation error, expected OK"},
		{"compiles, expected CE", []string{"CE"}, nil, true, []TestVerdict{ok}, nil, "passed every test, expected CE"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := DraftSolution{Verdicts: c.verdicts, Points: c.points}
			require.Equal(t, c.mismatch, s.Mismatch(c.compiled, c.tests, c.score))
		})
	}
}
