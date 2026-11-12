//go:build benchmark

package contests_test

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

// Grouped, already-loaded rows isolate Go projection cost from PostgreSQL and
// from the scanning/sorting behavior of the general correctness-test fixture.
type benchmarkScoreboardRepo struct {
	*scoreboardRepo
	standings []contestsmodel.ContestProblemResult
	batch     []contestsmodel.ScoreboardSubmission
}

func (r *benchmarkScoreboardRepo) GetContestScoreboardFromStandings(context.Context, uuid.UUID) ([]contestsmodel.ContestProblemResult, map[uuid.UUID]string, error) {
	return r.standings, r.users, nil
}

func (r *benchmarkScoreboardRepo) GetContestScoreboardSubmissions(context.Context, uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error) {
	return r.batch, nil
}

func BenchmarkScoreboardProjection(b *testing.B) {
	for _, size := range []struct {
		name  string
		users int
	}{{"small", 100}, {"large", 1000}} {
		b.Run(size.name, func(b *testing.B) {
			for _, unfrozen := range []bool{false, true} {
				name := "Frozen"
				if unfrozen {
					name = "Live"
				}
				b.Run(name, func(b *testing.B) {
					start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
					end := start.Add(2 * time.Hour)
					r := &benchmarkScoreboardRepo{scoreboardRepo: &scoreboardRepo{
						contest: contestsmodel.Contest{ID: uuid.New(), StartTime: &start, EndTime: &end, Settings: map[string]interface{}{"freeze_status": contestsmodel.FreezeStatusAuto, "freeze_duration_minutes": 60}},
						users:   map[uuid.UUID]string{},
					}}
					for p := 0; p < 10; p++ {
						r.problems = append(r.problems, contestsmodel.ContestProblem{ContestID: r.contest.ID, ProblemID: uuid.New(), Ordinal: p + 1, Title: fmt.Sprintf("Problem %d", p), ShortName: fmt.Sprintf("p%d", p)})
					}
					for u := 0; u < size.users; u++ {
						user := uuid.New()
						r.users[user] = fmt.Sprintf("user%d", u)
						for _, p := range r.problems {
							acTime := start.Add(90 * time.Minute)
							minutes := int32(90)
							r.standings = append(r.standings, contestsmodel.ContestProblemResult{ContestID: r.contest.ID, UserID: user, ProblemID: p.ProblemID, Solved: true, FailedAttempts: 1, FirstACTime: &acTime, TimeMinutes: &minutes})
							r.batch = append(r.batch,
								contestsmodel.ScoreboardSubmission{UserID: user, ProblemID: p.ProblemID, SubmissionForScoreboard: contestsmodel.SubmissionForScoreboard{State: submissionsmodel.GotWA, CreatedAt: start.Add(10 * time.Minute)}},
								contestsmodel.ScoreboardSubmission{UserID: user, ProblemID: p.ProblemID, SubmissionForScoreboard: contestsmodel.SubmissionForScoreboard{State: submissionsmodel.Accepted, CreatedAt: acTime}},
							)
						}
					}
					uc := contests.NewContestsUseCase(r, nil, nil, nil)
					// Read inside the freeze window; the fixed contest dates are in the past.
					ctx := accessmodel.WithReadTime(context.Background(), start.Add(90*time.Minute))
					operation := func() {
						result, err := uc.ProjectScoreboard(ctx, r.contest.ID, unfrozen)
						if err != nil {
							b.Fatal(err)
						}
						if len(result.Items) != size.users || len(result.Problems) != 10 {
							b.Fatal("incomplete scoreboard")
						}
						// Distinguish both paths so an accidental freeze bypass cannot look faster.
						item := result.Items[0]
						if unfrozen && item.ProblemsSolved != 10 {
							b.Fatal("live results lost solved problems")
						}
						if !unfrozen && (item.ProblemsSolved != 0 || len(item.ProblemResults) != 10 || item.ProblemResults[0].PendingAttempts != 1) {
							b.Fatal("frozen results lost pending attempts")
						}
					}
					operation()
					var memory runtime.MemStats
					runtime.ReadMemStats(&memory)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						operation()
					}
					b.StopTimer()
					checkScoreboardBenchmarkBudget(b, memory)
				})
			}
		})
	}
}

// Initial fixed budgets come from five local PostgreSQL 18.3 measurements with
// headroom: 2x maximum time, half minimum throughput, 25% above Go allocations.
// Columns: maximum ns/op, minimum ops/s, maximum B/op, maximum allocs/op.
var scoreboardBenchmarkBudgets = map[string]struct{ nanos, ops, bytes, allocs float64 }{
	"BenchmarkScoreboardProjection/large/Frozen": {3827000, 261.3, 1253312, 5013},
	"BenchmarkScoreboardProjection/large/Live":   {3740000, 267.45, 3991552, 6288},
	"BenchmarkScoreboardProjection/small/Frozen": {315000, 3177.5, 127360, 513},
	"BenchmarkScoreboardProjection/small/Live":   {332000, 3013.0, 390272, 649},
}

func checkScoreboardBenchmarkBudget(b *testing.B, before runtime.MemStats) {
	b.Helper()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	result := testing.BenchmarkResult{N: b.N, T: b.Elapsed(), MemBytes: after.TotalAlloc - before.TotalAlloc, MemAllocs: after.Mallocs - before.Mallocs}
	throughput := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(throughput, "ops/s")
	if flag.Lookup("test.benchtime").Value.String() != "100x" {
		b.Fatal("budget checks require -benchtime=100x")
	}
	budget, ok := scoreboardBenchmarkBudgets[b.Name()]
	if !ok {
		b.Fatal("missing performance budget for " + b.Name())
	}
	// Go first runs a one-operation pilot; only the full measurement is checked.
	if b.N == 1 {
		return
	}
	if b.N != 100 {
		b.Fatal("budget checks require 100 operations")
	}
	if value := float64(result.NsPerOp()); value > budget.nanos {
		b.Errorf("ns/op %.0f exceeds budget %.0f", value, budget.nanos)
	}
	if throughput < budget.ops {
		b.Errorf("ops/s %.2f is below budget %.2f", throughput, budget.ops)
	}
	if value := float64(result.AllocedBytesPerOp()); value > budget.bytes {
		b.Errorf("B/op %.0f exceeds budget %.0f", value, budget.bytes)
	}
	if value := float64(result.AllocsPerOp()); value > budget.allocs {
		b.Errorf("allocs/op %.0f exceeds budget %.0f", value, budget.allocs)
	}
}
