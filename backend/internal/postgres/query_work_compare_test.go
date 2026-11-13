package postgres

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// queryWork is the database work of one operation. Every field is deterministic
// for a fixed seed, schema and PostgreSQL version, unlike time.
type queryWork struct {
	// Statements and Transactions count what the code sends; BEGIN, COMMIT and
	// ROLLBACK are transaction control, not statements.
	Statements   int64 `json:"statements"`
	Transactions int64 `json:"transactions"`
	// Pages is shared hit + read: their split depends on the cache, the sum does not.
	Pages int64 `json:"pages"`
	// Sorts and SeqScans count plan nodes; SeqScans only on the seeded large tables.
	Sorts    int64 `json:"sorts"`
	SeqScans int64 `json:"seq_scans"`
}

func (w queryWork) metrics() []struct {
	name  string
	value int64
} {
	return []struct {
		name  string
		value int64
	}{
		{"statements", w.Statements},
		{"transactions", w.Transactions},
		{"pages", w.Pages},
		{"sorts", w.Sorts},
		{"seq_scans", w.SeqScans},
	}
}

// queryWorkTolerance is the relative change of a metric that still matches the
// golden value, in either direction. Larger regressions fail; larger improvements
// fail too, so the golden file never keeps slack that hides a later regression.
const queryWorkTolerance = 0.03

const queryWorkUpdate = "task golden:update"

// compareQueryWork returns one message per metric outside the tolerance and per
// scenario present on one side only.
func compareQueryWork(golden, actual map[string]queryWork, tolerance float64) []string {
	var problems []string
	names := make([]string, 0, len(golden)+len(actual))
	for name := range golden {
		names = append(names, name)
	}
	for name := range actual {
		if _, ok := golden[name]; !ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		want, inGolden := golden[name]
		got, measured := actual[name]
		switch {
		case !inGolden:
			problems = append(problems, fmt.Sprintf("%s: no golden values; record them with `%s`", name, queryWorkUpdate))
			continue
		case !measured:
			problems = append(problems, fmt.Sprintf("%s: golden values for a scenario that no longer runs; remove them with `%s`", name, queryWorkUpdate))
			continue
		}
		gotMetrics := got.metrics()
		for i, metric := range want.metrics() {
			g, a := metric.value, gotMetrics[i].value
			diff := float64(a - g)
			if math.Abs(diff) <= tolerance*float64(g) {
				continue
			}
			change := "+Inf%"
			if g != 0 {
				change = fmt.Sprintf("%+.1f%%", 100*diff/float64(g))
			}
			if diff > 0 {
				problems = append(problems, fmt.Sprintf("%s: %s regressed: golden %d, actual %d (%s, tolerance ±%g%%); if the extra work is intended, update the golden file with `%s`",
					name, metric.name, g, a, change, 100*tolerance, queryWorkUpdate))
			} else {
				problems = append(problems, fmt.Sprintf("%s: %s improved: golden %d, actual %d (%s, tolerance ±%g%%); the golden file is stale, lower it with `%s`",
					name, metric.name, g, a, change, 100*tolerance, queryWorkUpdate))
			}
		}
	}
	return problems
}

func TestCompareQueryWork(t *testing.T) {
	golden := map[string]queryWork{"Submissions/large/Latest": {Statements: 2, Transactions: 1, Pages: 1000, Sorts: 0, SeqScans: 0}}
	with := func(change func(*queryWork)) map[string]queryWork {
		w := golden["Submissions/large/Latest"]
		change(&w)
		return map[string]queryWork{"Submissions/large/Latest": w}
	}
	for _, tc := range []struct {
		name   string
		actual map[string]queryWork
		want   []string // substrings of the single expected message; none means a match
	}{
		{"identical", golden, nil},
		{"within tolerance above", with(func(w *queryWork) { w.Pages = 1030 }), nil},
		{"within tolerance below", with(func(w *queryWork) { w.Pages = 970 }), nil},
		{"regression by 30%", with(func(w *queryWork) { w.Pages = 1300 }),
			[]string{"Submissions/large/Latest: pages regressed: golden 1000, actual 1300 (+30.0%", "task golden:update"}},
		{"improvement beyond tolerance", with(func(w *queryWork) { w.Pages = 700 }),
			[]string{"Submissions/large/Latest: pages improved: golden 1000, actual 700 (-30.0%", "stale", "task golden:update"}},
		{"new sort node", with(func(w *queryWork) { w.Sorts = 1 }),
			[]string{"sorts regressed: golden 0, actual 1 (+Inf%"}},
		{"extra statement", with(func(w *queryWork) { w.Statements = 3 }),
			[]string{"statements regressed: golden 2, actual 3 (+50.0%"}},
		{"new scenario", map[string]queryWork{"Submissions/large/Latest": golden["Submissions/large/Latest"], "New": {}},
			[]string{"New: no golden values"}},
		{"removed scenario", map[string]queryWork{},
			[]string{"Submissions/large/Latest: golden values for a scenario that no longer runs"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems := compareQueryWork(golden, tc.actual, queryWorkTolerance)
			if tc.want == nil {
				require.Empty(t, problems)
				return
			}
			require.Len(t, problems, 1)
			for _, part := range tc.want {
				require.Contains(t, problems[0], part)
			}
		})
	}
}
