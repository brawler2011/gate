//go:build benchmark

package postgres

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// Every leaf owns its schema, including migration history. Never use application data.
func benchmarkPool(b *testing.B) *pgxpool.Pool {
	b.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BENCHMARK_DATABASE_URL")
	require.NotEmpty(b, dsn, "configure BENCHMARK_DATABASE_URL for a dedicated PostgreSQL 18 test database")
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(b, err)
	b.Cleanup(func() { require.NoError(b, admin.Close(ctx)) })
	var version int
	require.NoError(b, admin.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version))
	require.Equal(b, 18, version/10000, "benchmarks require PostgreSQL 18")
	_, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public")
	require.NoError(b, err)
	schema := "bench_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(b, err)
	b.Cleanup(func() {
		_, err := admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		require.NoError(b, err)
	})
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(b, err)
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	config.MaxConns, config.MinConns = 8, 8
	db := stdlib.OpenDB(*config.ConnConfig)
	b.Cleanup(func() { require.NoError(b, db.Close()) })
	_, file, _, ok := runtime.Caller(0)
	require.True(b, ok)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(filepath.Join(filepath.Dir(file), "../../migrations")))
	require.NoError(b, err)
	_, err = provider.Up(ctx)
	require.NoError(b, err)
	require.NoError(b, db.Close())
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

func benchExec(b *testing.B, pool *pgxpool.Pool, query string, args ...any) {
	b.Helper()
	_, err := pool.Exec(context.Background(), query, args...)
	require.NoError(b, err)
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

func seedOutbox(b *testing.B, pool *pgxpool.Pool, size int) {
	b.Helper()
	benchExec(b, pool, `INSERT INTO outbox_events(id,aggregate_id,aggregate_version,event_type,payload,headers,status,created_at)
 SELECT md5('event'||i)::uuid, md5('aggregate'||((i-1)/3))::uuid, (i-1)%3+1,
 'submission.updated', '{"submission_id":"00000000-0000-0000-0000-000000000001","revision":1}', '{}',
 CASE WHEN (i-1)%3=0 THEN 'completed'::outbox_event_status ELSE 'pending'::outbox_event_status END,
 '2026-01-01'::timestamptz + i*interval '1 millisecond' FROM generate_series(1,$1::int) i`, size)
	benchExec(b, pool, `INSERT INTO event_aggregates(aggregate_id,version) SELECT aggregate_id,max(aggregate_version) FROM outbox_events GROUP BY aggregate_id`)
	benchExec(b, pool, "VACUUM ANALYZE outbox_events")
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
					benchExec(b, pool, `UPDATE outbox_events SET status='pending',lease_token=NULL,deadline_at=NULL,locked_at=NULL,processed_at=NULL WHERE id=$1`, warm[0].Id)
					token := uuid.New()
					var ids []uuid.UUID
					if scenario == "MarkAsCompleted" {
						benchExec(b, pool, `UPDATE outbox_events SET status='processing',lease_token=$1,deadline_at=now()+interval '1 hour' WHERE aggregate_version=2`, token)
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

type submissionBenchFixture struct {
	contest, user, problem, submission uuid.UUID
	participants, rows                 int
}

func seedSubmissions(b *testing.B, pool *pgxpool.Pool, size int) submissionBenchFixture {
	b.Helper()
	participants := 100
	if size == 100000 {
		participants = 1000
	}
	f := submissionBenchFixture{contest: uuid.New(), participants: participants, rows: size}
	org := uuid.New()
	benchExec(b, pool, `INSERT INTO organizations(id,login,name) VALUES ($1,'benchmark','Benchmark')`, org)
	benchExec(b, pool, `INSERT INTO users(id,username,password_hash) SELECT md5('user'||i)::uuid,'bench'||i,'unused' FROM generate_series(1,$1::int) i`, participants)
	benchExec(b, pool, `INSERT INTO contests(id,organization_id,title,login,settings,start_time,end_time)
 VALUES ($1,$2,'Benchmark','benchmark','{"freeze_status":"frozen","freeze_duration_minutes":60}', '2026-01-01 00:00Z','2026-01-01 02:00Z')`, f.contest, org)
	benchExec(b, pool, `INSERT INTO problems(id,organization_id,title,short_name) SELECT md5('problem'||i)::uuid,$1,'Problem '||i,'p'||i FROM generate_series(1,10) i`, org)
	benchExec(b, pool, `INSERT INTO problem_packages(id,problem_id,organization_id,package_hash,status,version)
 SELECT md5('package'||i)::uuid,md5('problem'||i)::uuid,$1,md5('package'||i)||md5('package'||i),'ready',1 FROM generate_series(1,10) i`, org)
	benchExec(b, pool, `INSERT INTO contest_problems(contest_id,problem_id,package_id,ordinal)
 SELECT $1,md5('problem'||i)::uuid,md5('package'||i)::uuid,i FROM generate_series(1,10) i`, f.contest)
	benchExec(b, pool, `INSERT INTO contest_members(contest_id,user_id) SELECT $1,md5('user'||i)::uuid FROM generate_series(1,$2::int) i`, f.contest, participants)
	benchExec(b, pool, `INSERT INTO submissions(id,contest_id,owner_id,problem_id,source,language,state,revision,created_at)
 SELECT md5('submission'||i)::uuid,$1,md5('user'||(((i-1)/10)%$3::int+1))::uuid,md5('problem'||((i-1)%10+1))::uuid,
 repeat('print(1)\n',32),30,CASE WHEN i%3=0 THEN 200 ELSE 106 END,1,
 '2026-01-01'::timestamptz + ((i-1)/($3::int*10))*interval '10 minutes'
 FROM generate_series(1,$2::int) i`, f.contest, size, participants)
	benchExec(b, pool, `INSERT INTO contest_problem_results(contest_id,user_id,problem_id,solved,failed_attempts,first_ac_time,time_minutes)
 SELECT $1,md5('user'||u)::uuid,md5('problem'||p)::uuid,(u+p)%3=0,(u+p)%4,CASE WHEN (u+p)%3=0 THEN '2026-01-01 00:30Z'::timestamptz END,CASE WHEN (u+p)%3=0 THEN 30 END
 FROM generate_series(1,$2::int) u CROSS JOIN generate_series(1,10) p`, f.contest, participants)
	require.NoError(b, pool.QueryRow(context.Background(), `SELECT md5('user1')::uuid,md5('problem1')::uuid,md5('submission1')::uuid`).Scan(&f.user, &f.problem, &f.submission))
	for _, table := range []string{"submissions", "contests", "users", "problems", "contest_problems", "contest_members", "contest_problem_results"} {
		benchExec(b, pool, "VACUUM ANALYZE "+table)
	}
	return f
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

// Initial fixed budgets come from five local PostgreSQL 18.3 measurements with
// headroom: 2x maximum time, half minimum throughput, 25% above Go allocations.
// Columns: maximum ns/op, minimum ops/s, maximum B/op, maximum allocs/op.
var databaseBenchmarkBudgets = map[string]struct{ nanos, ops, bytes, allocs float64 }{
	"BenchmarkOutbox/large/MarkAsCompleted":              {38657000, 25.87, 1664, 54},
	"BenchmarkOutbox/large/PickEvents":                   {20507000, 48.76, 4160, 87},
	"BenchmarkOutbox/large/Process/workers16":            {1431000, 699.5, 6976, 153},
	"BenchmarkOutbox/large/Process/workers4":             {2048000, 488.35, 6016, 142},
	"BenchmarkOutbox/small/MarkAsCompleted":              {1844000, 542.5, 1664, 54},
	"BenchmarkOutbox/small/PickEvents":                   {3064000, 326.39, 4160, 87},
	"BenchmarkOutbox/small/Process/workers16":            {1310000, 763.5, 6976, 153},
	"BenchmarkOutbox/small/Process/workers4":             {2247000, 445.05, 6080, 142},
	"BenchmarkScoreboardQueries/large/Standings":         {25889000, 38.62, 10681088, 158437},
	"BenchmarkScoreboardQueries/large/SubmissionHistory": {285000, 3509.5, 5248, 115},
	"BenchmarkScoreboardQueries/small/Standings":         {2575000, 388.45, 854144, 15905},
	"BenchmarkScoreboardQueries/small/SubmissionHistory": {232000, 4312.5, 3008, 89},
	"BenchmarkSubmissions/large/Create":                  {1853000, 539.5, 4032, 93},
	"BenchmarkSubmissions/large/Get":                     {667000, 1501.5, 6336, 97},
	"BenchmarkSubmissions/large/Latest":                  {15112000, 66.15, 128640, 1952},
	"BenchmarkSubmissions/large/UpdateRevision":          {17129000, 58.4, 1856, 44},
	"BenchmarkSubmissions/large/WriteRead/workers16":     {2695000, 371.1, 11776, 203},
	"BenchmarkSubmissions/large/WriteRead/workers4":      {19958000, 50.1, 10688, 192},
	"BenchmarkSubmissions/small/Create":                  {22326000, 44.79, 4032, 93},
	"BenchmarkSubmissions/small/Get":                     {846000, 1182.5, 6336, 97},
	"BenchmarkSubmissions/small/Latest":                  {6031000, 165.85, 128640, 1952},
	"BenchmarkSubmissions/small/UpdateRevision":          {21702000, 46.08, 1856, 44},
	"BenchmarkSubmissions/small/WriteRead/workers16":     {2922000, 342.35, 11776, 203},
	"BenchmarkSubmissions/small/WriteRead/workers4":      {2225000, 449.55, 10688, 192},
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
