//go:build benchmark

package postgres

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Disable synchronous commit to keep host WAL flush latency out of write benchmarks.
var databaseCommand = []string{"postgres", "-c", "synchronous_commit=off"}

func benchmarkPool(b *testing.B) *pgxpool.Pool {
	b.Helper()
	ctx := context.Background()
	config := migratedSchema(b)
	config.MaxConns, config.MinConns = 8, 8
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(b, err)
	b.Cleanup(pool.Close)
	func() {
		var conns []*pgxpool.Conn
		defer func() {
			for _, conn := range conns {
				conn.Release()
			}
		}()
		for range 8 {
			conn, err := pool.Acquire(ctx)
			require.NoError(b, err)
			conns = append(conns, conn)
		}
	}()
	var settings string
	require.NoError(b, pool.QueryRow(ctx, `SELECT json_build_object('version',version(),'shared_buffers',current_setting('shared_buffers'),'fsync',current_setting('fsync'),'synchronous_commit',current_setting('synchronous_commit'),'max_connections',current_setting('max_connections'))::text`).Scan(&settings))
	b.Logf("PostgreSQL: %s; pool=8; GOMAXPROCS=%d", settings, runtime.GOMAXPROCS(0))
	return pool
}

// Fixed -benchtime=100x prevents calibration from growing write datasets unpredictably.
func benchmarkWork(b *testing.B, workers int, operation func(int) error) {
	b.Helper()
	require.LessOrEqual(b, b.N, 100, "use -benchtime=100x or fewer iterations")
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	b.ReportAllocs()
	b.ResetTimer()
	if workers == 1 {
		for i := 0; i < b.N; i++ {
			if err := operation(i); err != nil {
				b.Fatal(err)
			}
		}
	} else {
		var next atomic.Int64
		var wg sync.WaitGroup
		failures := make(chan error, workers)
		for range workers {
			wg.Go(func() {
				for {
					i := int(next.Add(1) - 1)
					if i >= b.N {
						return
					}
					if err := operation(i); err != nil {
						failures <- err
						return
					}
				}
			})
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			require.NoError(b, err)
		}
	}
	b.StopTimer()
	checkDatabaseBenchmarkBudget(b, memory)
}

func BenchmarkSubmissions(b *testing.B) {
	for _, size := range []struct {
		name string
		rows int
	}{{"small", 1000}, {"large", 100000}} {
		b.Run(size.name, func(b *testing.B) {
			for _, scenario := range []string{"Create", "UpdateRevision", "Get", "Latest", "WriteRead/workers4", "WriteRead/workers16"} {
				b.Run(scenario, func(b *testing.B) {
					pool := benchmarkPool(b)
					f := seedSubmissions(b, pool, size.rows)
					repo := NewSubmissionsRepo(pool)
					ctx := context.Background()
					order := int32(-1)
					filter := submissionsmodel.SubmissionsFilter{ContestId: &f.contest, Page: 1, PageSize: 50, Order: &order}
					_, err := repo.GetSubmission(ctx, f.submission)
					require.NoError(b, err)
					list, count, err := repo.ListSubmissions(ctx, filter)
					require.NoError(b, err)
					require.Len(b, list, 50)
					require.EqualValues(b, size.rows, count)
					tx, err := pool.Begin(ctx)
					require.NoError(b, err)
					creation := submissionsmodel.SubmissionCreation{Kind: submissionsmodel.KindContest, ContestId: f.contest, ProblemId: f.problem, UserId: f.user, Solution: strings.Repeat("print(1)\n", 32), Language: "python3"}
					_, err = repo.WithTx(tx).CreateSubmission(ctx, &creation)
					require.NoError(b, err)
					_, err = repo.WithTx(tx).UpdateSubmission(ctx, f.submission, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.Accepted})
					require.NoError(b, err)
					require.NoError(b, tx.Rollback(ctx))
					workers := 1
					if scenario == "WriteRead/workers4" {
						workers = 4
					}
					if scenario == "WriteRead/workers16" {
						workers = 16
					}
					benchmarkWork(b, workers, func(_ int) error {
						switch scenario {
						case "Create":
							id, err := repo.CreateSubmission(ctx, &creation)
							if err != nil {
								return err
							}
							if id == uuid.Nil {
								return fmt.Errorf("create returned nil ID")
							}
						case "UpdateRevision":
							change, err := repo.UpdateSubmission(ctx, f.submission, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.Accepted})
							if err != nil {
								return err
							}
							if change.Revision <= 1 {
								return fmt.Errorf("revision did not advance")
							}
						case "Get":
							sub, err := repo.GetSubmission(ctx, f.submission)
							if err != nil {
								return err
							}
							if sub.ID != f.submission {
								return fmt.Errorf("unexpected submission")
							}
						case "Latest":
							items, total, err := repo.ListSubmissions(ctx, filter)
							if err != nil {
								return err
							}
							if len(items) != 50 || int(total) != size.rows {
								return fmt.Errorf("incomplete submission snapshot")
							}
						default:
							id, err := repo.CreateSubmission(ctx, &creation)
							if err != nil {
								return err
							}
							sub, err := repo.GetSubmission(ctx, id)
							if err != nil {
								return err
							}
							if sub.ID != id || sub.Revision != 1 {
								return fmt.Errorf("invalid inserted submission")
							}
						}
						return nil
					})
					var total int
					var revision int64
					require.NoError(b, pool.QueryRow(ctx, "SELECT count(*) FROM submissions").Scan(&total))
					require.NoError(b, pool.QueryRow(ctx, "SELECT SUM(revision) FROM submissions").Scan(&revision))
					expectedRows, expectedRevision := size.rows, int64(size.rows)
					if scenario == "Create" || workers > 1 {
						expectedRows += b.N
						expectedRevision += int64(b.N)
					}
					if scenario == "UpdateRevision" {
						expectedRevision += int64(b.N)
					}
					require.Equal(b, expectedRows, total)
					require.Equal(b, expectedRevision, revision)
				})
			}
		})
	}
}

func BenchmarkScoreboardQueries(b *testing.B) {
	for _, size := range []struct {
		name string
		rows int
	}{{"small", 1000}, {"large", 100000}} {
		b.Run(size.name, func(b *testing.B) {
			for _, scenario := range []string{"Contest", "User"} {
				b.Run(scenario, func(b *testing.B) {
					pool := benchmarkPool(b)
					f := seedSubmissions(b, pool, size.rows)
					repo := NewContestsRepo(pool)
					ctx := context.Background()
					operation := func(_ int) error {
						if scenario == "Contest" {
							return contestScoreboardQueries(ctx, repo, f)
						}
						return userScoreboardQueries(ctx, repo, f)
					}
					require.NoError(b, operation(0))
					benchmarkWork(b, 1, operation)
				})
			}
		})
	}
}

var databaseBenchmarkBudgets = map[string]struct{ nanos, ops, bytes, allocs float64 }{
	"BenchmarkScoreboardQueries/large/Contest":       {240000000, 4.17, 80000000, 1010000},
	"BenchmarkScoreboardQueries/large/User":          {420000, 2381.0, 70000, 1100},
	"BenchmarkScoreboardQueries/small/Contest":       {2100000, 476.2, 650000, 10800},
	"BenchmarkScoreboardQueries/small/User":          {215000, 4651.0, 9000, 165},
	"BenchmarkSubmissions/large/Create":              {681892, 1466.50, 4032, 93},
	"BenchmarkSubmissions/large/Get":                 {667000, 1501.5, 6336, 97},
	"BenchmarkSubmissions/large/Latest":              {15112000, 66.15, 128640, 1952},
	"BenchmarkSubmissions/large/UpdateRevision":      {564854, 1770.50, 1856, 44},
	"BenchmarkSubmissions/large/WriteRead/workers16": {814294, 1228.00, 11776, 203},
	"BenchmarkSubmissions/large/WriteRead/workers4":  {833538, 1199.50, 10688, 192},
	"BenchmarkSubmissions/small/Create":              {560530, 1784.00, 4032, 93},
	"BenchmarkSubmissions/small/Get":                 {846000, 1182.5, 6336, 97},
	"BenchmarkSubmissions/small/Latest":              {6031000, 165.85, 128640, 1952},
	"BenchmarkSubmissions/small/UpdateRevision":      {540754, 1849.50, 1856, 44},
	"BenchmarkSubmissions/small/WriteRead/workers16": {808556, 1237.00, 11776, 203},
	"BenchmarkSubmissions/small/WriteRead/workers4":  {844640, 1184.00, 10688, 192},
}

func checkDatabaseBenchmarkBudget(b *testing.B, before runtime.MemStats) {
	b.Helper()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	result := testing.BenchmarkResult{N: b.N, T: b.Elapsed(), MemBytes: after.TotalAlloc - before.TotalAlloc, MemAllocs: after.Mallocs - before.Mallocs}
	throughput := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(throughput, "ops/s")
	if flag.Lookup("test.benchtime").Value.String() != "100x" {
		b.Fatal("budget checks require -benchtime=100x")
	}
	budget, ok := databaseBenchmarkBudgets[b.Name()]
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
