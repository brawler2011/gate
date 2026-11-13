//go:build benchmark || integration

package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// postgresImage is pinned to an exact release: query plans and page counts may
// change between minor versions.
const postgresImage = "postgres:18.3-alpine"

// One PostgreSQL container serves the package run, started with databaseCommand,
// which the benchmark and the query work builds define for their own needs.
var (
	databaseOnce      sync.Once
	databaseContainer *tcpostgres.PostgresContainer
	databaseDSN       string
	databaseErr       error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if databaseContainer != nil {
		if err := databaseContainer.Terminate(context.Background(), testcontainers.StopTimeout(0)); err != nil {
			log.Printf("terminate postgres container: %v", err)
		}
	}
	os.Exit(code)
}

func startDatabase(ctx context.Context) (string, error) {
	container, err := tcpostgres.Run(ctx,
		postgresImage,
		tcpostgres.WithDatabase("gate_benchmarks"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithCmd(databaseCommand...),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if container != nil {
		databaseContainer = container
	}
	if err != nil {
		return "", fmt.Errorf("start postgres container: %w", err)
	}
	return container.ConnectionString(ctx, "sslmode=disable")
}

// sharedDatabase starts the package's container on first use and returns its DSN.
func sharedDatabase(tb testing.TB) string {
	tb.Helper()
	databaseOnce.Do(func() { databaseDSN, databaseErr = startDatabase(context.Background()) })
	require.NoError(tb, databaseErr)
	return databaseDSN
}

// migratedSchema creates a fresh schema with the migrated database layout, dropped
// when tb ends, and returns a pool configuration that uses it.
func migratedSchema(tb testing.TB) *pgxpool.Config {
	tb.Helper()
	ctx := context.Background()
	dsn := sharedDatabase(tb)
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(tb, err)
	tb.Cleanup(func() { require.NoError(tb, admin.Close(ctx)) })
	_, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public")
	require.NoError(tb, err)
	schema := "bench_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(tb, err)
	tb.Cleanup(func() {
		_, err := admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		require.NoError(tb, err)
	})
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(tb, err)
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	db := stdlib.OpenDB(*config.ConnConfig)
	defer func() { require.NoError(tb, db.Close()) }()
	_, file, _, ok := runtime.Caller(0)
	require.True(tb, ok)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(filepath.Join(filepath.Dir(file), "../../migrations")))
	require.NoError(tb, err)
	_, err = provider.Up(ctx)
	require.NoError(tb, err)
	return config
}

func fixtureExec(tb testing.TB, pool *pgxpool.Pool, query string, args ...any) {
	tb.Helper()
	_, err := pool.Exec(context.Background(), query, args...)
	require.NoError(tb, err)
}

func seedOutbox(tb testing.TB, pool *pgxpool.Pool, size int) {
	tb.Helper()
	fixtureExec(tb, pool, `INSERT INTO outbox_events(id,aggregate_id,aggregate_version,event_type,payload,headers,status,created_at)
 SELECT md5('event'||i)::uuid, md5('aggregate'||((i-1)/3))::uuid, (i-1)%3+1,
 'submission.updated', '{"submission_id":"00000000-0000-0000-0000-000000000001","revision":1}', '{}',
 CASE WHEN (i-1)%3=0 THEN 'completed'::outbox_event_status ELSE 'pending'::outbox_event_status END,
 '2026-01-01'::timestamptz + i*interval '1 millisecond' FROM generate_series(1,$1::int) i`, size)
	fixtureExec(tb, pool, `INSERT INTO event_aggregates(aggregate_id,version) SELECT aggregate_id,max(aggregate_version) FROM outbox_events GROUP BY aggregate_id`)
	fixtureExec(tb, pool, "VACUUM ANALYZE outbox_events")
}

type submissionBenchFixture struct {
	contest, user, problem, submission uuid.UUID
	participants, rows                 int
}

func seedSubmissions(tb testing.TB, pool *pgxpool.Pool, size int) submissionBenchFixture {
	tb.Helper()
	ctx := context.Background()
	participants := 100
	if size == 100000 {
		participants = 1000
	}
	f := submissionBenchFixture{participants: participants, rows: size}
	var org uuid.UUID
	require.NoError(tb, pool.QueryRow(ctx, `SELECT md5('contest')::uuid,md5('organization')::uuid`).Scan(&f.contest, &org))
	fixtureExec(tb, pool, `INSERT INTO organizations(id,login,name) VALUES ($1,'benchmark','Benchmark')`, org)
	fixtureExec(tb, pool, `INSERT INTO users(id,username,password_hash) SELECT md5('user'||i)::uuid,'bench'||i,'unused' FROM generate_series(1,$1::int) i`, participants)
	fixtureExec(tb, pool, `INSERT INTO contests(id,organization_id,title,login,settings,start_time,end_time)
 VALUES ($1,$2,'Benchmark','benchmark','{"freeze_status":"frozen","freeze_duration_minutes":60}', '2026-01-01 00:00Z','2026-01-01 02:00Z')`, f.contest, org)
	fixtureExec(tb, pool, `INSERT INTO problems(id,organization_id) SELECT md5('problem'||i)::uuid,$1 FROM generate_series(1,10) i`, org)
	fixtureExec(tb, pool, `INSERT INTO problem_drafts(problem_id,organization_id,title,short_name,type,time_limit_ms,memory_limit_mb)
 SELECT md5('problem'||i)::uuid,$1,'Problem '||i,'p'||i,'pass-fail',1000,256 FROM generate_series(1,10) i`, org)
	for i := 1; i <= 10; i++ {
		var problem uuid.UUID
		require.NoError(tb, pool.QueryRow(ctx, `SELECT md5('problem'||$1::int)::uuid`, i).Scan(&problem))
		pack, err := dbfixtures.InsertPackage(ctx, pool, problem, org, time.Time{})
		require.NoError(tb, err)
		fixtureExec(tb, pool, `INSERT INTO contest_problems(contest_id,problem_id,package_id,ordinal) VALUES ($1,$2,$3,$4)`, f.contest, problem, pack, i)
	}
	fixtureExec(tb, pool, `INSERT INTO contest_members(contest_id,user_id) SELECT $1,md5('user'||i)::uuid FROM generate_series(1,$2::int) i`, f.contest, participants)
	fixtureExec(tb, pool, `INSERT INTO submissions(id,contest_id,owner_id,problem_id,source,language,state,revision,created_at)
 SELECT md5('submission'||i)::uuid,$1,md5('user'||(((i-1)/10)%$3::int+1))::uuid,md5('problem'||((i-1)%10+1))::uuid,
 repeat('print(1)\n',32),30,CASE WHEN i%3=0 THEN 200 ELSE 106 END,1,
 '2026-01-01'::timestamptz + ((i-1)/($3::int*10))*interval '10 minutes'
 FROM generate_series(1,$2::int) i`, f.contest, size, participants)
	fixtureExec(tb, pool, `INSERT INTO contest_problem_results(contest_id,user_id,problem_id,solved,failed_attempts,first_ac_time,time_minutes)
 SELECT $1,md5('user'||u)::uuid,md5('problem'||p)::uuid,(u+p)%3=0,(u+p)%4,CASE WHEN (u+p)%3=0 THEN '2026-01-01 00:30Z'::timestamptz END,CASE WHEN (u+p)%3=0 THEN 30 END
 FROM generate_series(1,$2::int) u CROSS JOIN generate_series(1,10) p`, f.contest, participants)
	require.NoError(tb, pool.QueryRow(ctx, `SELECT md5('user1')::uuid,md5('problem1')::uuid,md5('submission1')::uuid`).Scan(&f.user, &f.problem, &f.submission))
	for _, table := range []string{"submissions", "contests", "users", "problems", "contest_problems", "contest_members", "contest_problem_results"} {
		fixtureExec(tb, pool, "VACUUM ANALYZE "+table)
	}
	return f
}
