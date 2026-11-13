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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// The benchmarks keep fsync on because they measure writes. Commits do not wait
// for the WAL flush: fsync latency on a laptop is bimodal (~1 or ~10 ms) and
// says nothing about the code under test.
var databaseCommand = []string{"postgres", "-c", "synchronous_commit=off"}

// Every leaf owns its schema, including migration history.
func benchmarkPool(b *testing.B) *pgxpool.Pool {
	b.Helper()
	ctx := context.Background()
	config := migratedSchema(b)
	config.MaxConns, config.MinConns = 8, 8
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(b, err)
	b.Cleanup(pool.Close)
	// Establish all pool connections before timing, including concurrent scenarios.
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

// Fixed work avoids Go's duration calibration growing write datasets unpredictably.
// Callers must use -benchtime=100x (the task command supplies it).
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

func BenchmarkOutbox(b *testing.B) {
	for _, size := range []struct {
		name string
		rows int
	}{{"small", 1000}, {"large", 100000}} {
		b.Run(size.name, func(b *testing.B) {
			for _, scenario := range []string{"PickEvents", "MarkAsCompleted", "Process/workers4", "Process/workers16"} {
				b.Run(scenario, func(b *testing.B) {
					pool := benchmarkPool(b)
					seedOutbox(b, pool, size.rows)
					repo := NewOutboxRepo(pool)
					ctx := context.Background()
					// Warm the production pick/complete path; restore leases/status afterwards.
					warm, err := repo.PickEvents(ctx, 1, 300)
					require.NoError(b, err)
					require.Len(b, warm, 1)
					require.NoError(b, repo.MarkAsCompleted(ctx, warm[0].Id, warm[0].LeaseToken))
					fixtureExec(b, pool, `UPDATE outbox_events SET status='pending',lease_token=NULL,deadline_at=NULL,locked_at=NULL,processed_at=NULL WHERE id=$1`, warm[0].Id)
					token := uuid.New()
					var ids []uuid.UUID
					if scenario == "MarkAsCompleted" {
						fixtureExec(b, pool, `UPDATE outbox_events SET status='processing',lease_token=$1,deadline_at=now()+interval '1 hour' WHERE aggregate_version=2`, token)
						rows, err := pool.Query(ctx, "SELECT id FROM outbox_events WHERE aggregate_version=2 ORDER BY created_at LIMIT 100")
						require.NoError(b, err)
						ids, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
						require.NoError(b, err)
						require.Len(b, ids, 100)
					}
					workers := 1
					if scenario == "Process/workers4" {
						workers = 4
					}
					if scenario == "Process/workers16" {
						workers = 16
					}
					benchmarkWork(b, workers, func(i int) error {
						if scenario == "MarkAsCompleted" {
							return repo.MarkAsCompleted(ctx, ids[i], token)
						}
						events, err := repo.PickEvents(ctx, 1, 300)
						if err != nil {
							return err
						}
						if len(events) != 1 {
							return fmt.Errorf("pick returned %d events, want 1", len(events))
						}
						if scenario != "PickEvents" {
							return repo.MarkAsCompleted(ctx, events[0].Id, events[0].LeaseToken)
						}
						return nil
					})
					var count int
					switch scenario {
					case "PickEvents":
						require.NoError(b, pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE status='processing'").Scan(&count))
					case "MarkAsCompleted":
						require.NoError(b, pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_version=2 AND status='completed'").Scan(&count))
					default:
						require.NoError(b, pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE aggregate_version>1 AND status='completed'").Scan(&count))
					}
					require.Equal(b, b.N, count)
				})
			}
		})
	}
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
					// Warm writes on an isolated transaction so measured data/revision stay unchanged.
					tx, err := pool.Begin(ctx)
					require.NoError(b, err)
					creation := submissionsmodel.SubmissionCreation{ContestId: f.contest, ProblemId: f.problem, UserId: f.user, Solution: strings.Repeat("print(1)\n", 32), Language: submissionsmodel.Python}
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
			for _, scenario := range []string{"Standings", "SubmissionHistory"} {
				b.Run(scenario, func(b *testing.B) {
					pool := benchmarkPool(b)
					f := seedSubmissions(b, pool, size.rows)
					repo := NewContestsRepo(pool)
					ctx := context.Background()
					operation := func(_ int) error {
						if scenario == "Standings" {
							results, users, err := repo.GetContestScoreboardFromStandings(ctx, f.contest)
							if err != nil {
								return err
							}
							if len(results) != f.participants*10 || len(users) != f.participants {
								return fmt.Errorf("incomplete standings")
							}
						} else {
							subs, err := repo.GetSubmissionsForScoreboard(ctx, f.contest, f.user, f.problem)
							if err != nil {
								return err
							}
							if len(subs) != f.rows/(f.participants*10) {
								return fmt.Errorf("incomplete submission history")
							}
						}
						return nil
					}
					require.NoError(b, operation(0))
					benchmarkWork(b, 1, operation)
				})
			}
		})
	}
}

// Budgets come from five local PostgreSQL 18.3 measurements with headroom: 2x
// maximum time, half minimum throughput, 25% above Go allocations. The time and
// throughput budgets of writes were re-measured with synchronous_commit=off.
// Columns: maximum ns/op, minimum ops/s, maximum B/op, maximum allocs/op.
var databaseBenchmarkBudgets = map[string]struct{ nanos, ops, bytes, allocs float64 }{
	"BenchmarkOutbox/large/MarkAsCompleted":              {11719176, 85.35, 1664, 54},
	"BenchmarkOutbox/large/PickEvents":                   {1927474, 519.00, 4160, 87},
	"BenchmarkOutbox/large/Process/workers16":            {525668, 1902.50, 6976, 153},
	"BenchmarkOutbox/large/Process/workers4":             {753032, 1328.00, 6016, 142},
	"BenchmarkOutbox/small/MarkAsCompleted":              {552146, 1811.00, 1664, 54},
	"BenchmarkOutbox/small/PickEvents":                   {2094484, 477.45, 4160, 87},
	"BenchmarkOutbox/small/Process/workers16":            {831882, 1202.00, 6976, 153},
	"BenchmarkOutbox/small/Process/workers4":             {726704, 1376.00, 6080, 142},
	"BenchmarkScoreboardQueries/large/Standings":         {25889000, 38.62, 10681088, 158437},
	"BenchmarkScoreboardQueries/large/SubmissionHistory": {285000, 3509.5, 5248, 115},
	"BenchmarkScoreboardQueries/small/Standings":         {2575000, 388.45, 854144, 15905},
	"BenchmarkScoreboardQueries/small/SubmissionHistory": {232000, 4312.5, 3008, 89},
	"BenchmarkSubmissions/large/Create":                  {681892, 1466.50, 4032, 93},
	"BenchmarkSubmissions/large/Get":                     {667000, 1501.5, 6336, 97},
	"BenchmarkSubmissions/large/Latest":                  {15112000, 66.15, 128640, 1952},
	"BenchmarkSubmissions/large/UpdateRevision":          {564854, 1770.50, 1856, 44},
	"BenchmarkSubmissions/large/WriteRead/workers16":     {814294, 1228.00, 11776, 203},
	"BenchmarkSubmissions/large/WriteRead/workers4":      {833538, 1199.50, 10688, 192},
	"BenchmarkSubmissions/small/Create":                  {560530, 1784.00, 4032, 93},
	"BenchmarkSubmissions/small/Get":                     {846000, 1182.5, 6336, 97},
	"BenchmarkSubmissions/small/Latest":                  {6031000, 165.85, 128640, 1952},
	"BenchmarkSubmissions/small/UpdateRevision":          {540754, 1849.50, 1856, 44},
	"BenchmarkSubmissions/small/WriteRead/workers16":     {808556, 1237.00, 11776, 203},
	"BenchmarkSubmissions/small/WriteRead/workers4":      {844640, 1184.00, 10688, 192},
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
