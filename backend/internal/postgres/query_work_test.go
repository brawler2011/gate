//go:build integration && !benchmark

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// The query work check counts work instead of timing it, so durability is off.
// pg_stat_statements reports the pages each executed statement touched; parallel
// workers and autovacuum would make plans and page counts depend on timing.
var databaseCommand = []string{
	"postgres",
	"-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off",
	"-c", "autovacuum=off", "-c", "max_parallel_workers_per_gather=0",
	"-c", "shared_preload_libraries=pg_stat_statements", "-c", "pg_stat_statements.track_utility=off",
}

const queryWorkGolden = "testdata/query_work.json"

var updateQueryWork = flag.Bool("update", false, "rewrite "+queryWorkGolden+" with the measured query work")

// queryWorkLargeTables are the tables the seeds fill with up to 100,000 rows; a
// sequential scan of any of them is part of the work metrics.
var queryWorkLargeTables = map[string]bool{
	"outbox_events": true, "event_aggregates": true, "submissions": true,
	"users": true, "contest_members": true, "contest_problem_results": true,
}

// statementRecorder captures the statements a connection sends while recording.
type statementRecorder struct {
	mu         sync.Mutex
	recording  bool
	statements []recordedStatement
}

type recordedStatement struct {
	sql  string
	args []any
}

func (r *statementRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recording {
		r.statements = append(r.statements, recordedStatement{data.SQL, data.Args})
	}
	return ctx
}

func (r *statementRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (r *statementRecorder) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recording, r.statements = true, nil
}

func (r *statementRecorder) stop() []recordedStatement {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recording = false
	return r.statements
}

// workScenario is one operation of database_benchmark_test.go. A write runs in a
// transaction that is rolled back, so every scenario sees the same seeded data.
type workScenario struct {
	name  string
	write bool
	// setup prepares a write's transaction; its statements are not measured.
	setup func(ctx context.Context, tx pgx.Tx) error
	// run performs the operation; tx is nil for reads, which use the pool.
	run func(ctx context.Context, tx pgx.Tx) error
}

func TestQueryWork(t *testing.T) {
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, sharedDatabase(t))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, admin.Close(ctx)) })
	_, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_stat_statements")
	require.NoError(t, err)

	actual := map[string]queryWork{}
	for _, size := range []struct {
		name string
		rows int
	}{{"small", 1000}, {"large", 100000}} {
		t.Run(size.name, func(t *testing.T) {
			recorder := &statementRecorder{}
			config := migratedSchema(t)
			// One connection: reads run on the connection the warm-up pass prepared.
			config.MaxConns, config.MinConns = 1, 1
			config.ConnConfig.Tracer = recorder
			pool, err := pgxpool.NewWithConfig(ctx, config)
			require.NoError(t, err)
			t.Cleanup(pool.Close)
			seedOutbox(t, pool, size.rows)
			f := seedSubmissions(t, pool, size.rows)
			fixtureExec(t, pool, "VACUUM ANALYZE")
			for _, scenario := range workScenarios(pool, f) {
				group, operation, _ := strings.Cut(scenario.name, "/")
				actual[group+"/"+size.name+"/"+operation] = measureQueryWork(t, admin, pool, recorder, scenario)
			}
		})
	}
	if t.Failed() {
		return
	}

	if *updateQueryWork {
		data, err := json.MarshalIndent(actual, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(queryWorkGolden, append(data, '\n'), 0o644))
		t.Logf("wrote %s", queryWorkGolden)
		return
	}
	data, err := os.ReadFile(queryWorkGolden)
	require.NoError(t, err, "record the golden file with `%s`", queryWorkUpdate)
	var golden map[string]queryWork
	require.NoError(t, json.Unmarshal(data, &golden))
	for _, problem := range compareQueryWork(golden, actual, queryWorkTolerance) {
		t.Error(problem)
	}
}

func workScenarios(pool *pgxpool.Pool, f submissionBenchFixture) []workScenario {
	outbox := NewOutboxRepo(pool)
	submissions := NewSubmissionsRepo(pool)
	contests := NewContestsRepo(pool)
	creation := submissionsmodel.SubmissionCreation{ContestId: f.contest, ProblemId: f.problem, UserId: f.user, Solution: strings.Repeat("print(1)\n", 32), Language: submissionsmodel.Python}
	order := int32(-1)
	filter := submissionsmodel.SubmissionsFilter{ContestId: &f.contest, Page: 1, PageSize: 50, Order: &order}
	// The second event of the first aggregate: its first version is completed.
	var leased, token uuid.UUID
	return []workScenario{
		{name: "Outbox/PickEvents", write: true, run: func(ctx context.Context, tx pgx.Tx) error {
			events, err := outbox.WithTx(tx).PickEvents(ctx, 1, 300)
			if err == nil && len(events) != 1 {
				err = fmt.Errorf("pick returned %d events, want 1", len(events))
			}
			return err
		}},
		{name: "Outbox/MarkAsCompleted", write: true,
			setup: func(ctx context.Context, tx pgx.Tx) error {
				token = uuid.New()
				return tx.QueryRow(ctx, `UPDATE outbox_events SET status='processing',lease_token=$1,deadline_at=now()+interval '1 hour'
 WHERE id=md5('event2')::uuid RETURNING id`, token).Scan(&leased)
			},
			run: func(ctx context.Context, tx pgx.Tx) error {
				return outbox.WithTx(tx).MarkAsCompleted(ctx, leased, token)
			}},
		{name: "Submissions/Create", write: true, run: func(ctx context.Context, tx pgx.Tx) error {
			id, err := submissions.WithTx(tx).CreateSubmission(ctx, &creation)
			if err == nil && id == uuid.Nil {
				err = errors.New("create returned nil ID")
			}
			return err
		}},
		{name: "Submissions/UpdateRevision", write: true, run: func(ctx context.Context, tx pgx.Tx) error {
			change, err := submissions.WithTx(tx).UpdateSubmission(ctx, f.submission, &submissionsmodel.SubmissionUpdate{State: submissionsmodel.Accepted})
			if err == nil && change.Revision != 2 {
				err = fmt.Errorf("revision %d, want 2", change.Revision)
			}
			return err
		}},
		{name: "Submissions/Get", run: func(ctx context.Context, _ pgx.Tx) error {
			sub, err := submissions.GetSubmission(ctx, f.submission)
			if err == nil && sub.ID != f.submission {
				err = errors.New("unexpected submission")
			}
			return err
		}},
		{name: "Submissions/Latest", run: func(ctx context.Context, _ pgx.Tx) error {
			items, total, err := submissions.ListSubmissions(ctx, filter)
			if err == nil && (len(items) != 50 || int(total) != f.rows) {
				err = errors.New("incomplete submission snapshot")
			}
			return err
		}},
		{name: "ScoreboardQueries/Standings", run: func(ctx context.Context, _ pgx.Tx) error {
			results, users, err := contests.GetContestScoreboardFromStandings(ctx, f.contest)
			if err == nil && (len(results) != f.participants*10 || len(users) != f.participants) {
				err = errors.New("incomplete standings")
			}
			return err
		}},
		{name: "ScoreboardQueries/SubmissionHistory", run: func(ctx context.Context, _ pgx.Tx) error {
			subs, err := contests.GetSubmissionsForScoreboard(ctx, f.contest, f.user, f.problem)
			if err == nil && len(subs) != f.rows/(f.participants*10) {
				err = errors.New("incomplete submission history")
			}
			return err
		}},
	}
}

// measureQueryWork runs the scenario twice, so the measured pass finds prepared
// statements and warm catalog caches, and returns the measured pass's work.
func measureQueryWork(t *testing.T, admin *pgx.Conn, pool *pgxpool.Pool, recorder *statementRecorder, scenario workScenario) queryWork {
	t.Helper()
	ctx := context.Background()
	var work queryWork
	for _, measured := range []bool{false, true} {
		func() {
			var tx pgx.Tx
			var explainer interface {
				Query(context.Context, string, ...any) (pgx.Rows, error)
			} = pool
			if scenario.write {
				var err error
				tx, err = pool.Begin(ctx)
				require.NoError(t, err)
				defer func() { require.NoError(t, tx.Rollback(ctx)) }()
				explainer = tx
				if scenario.setup != nil {
					require.NoError(t, scenario.setup(ctx, tx), scenario.name)
				}
			}
			if measured {
				_, err := admin.Exec(ctx, "SELECT pg_stat_statements_reset()")
				require.NoError(t, err)
			}
			recorder.start()
			require.NoError(t, scenario.run(ctx, tx), scenario.name)
			statements := recorder.stop()
			if !measured {
				return
			}
			require.NoError(t, admin.QueryRow(ctx, `SELECT coalesce(sum(shared_blks_hit+shared_blks_read),0)::bigint FROM pg_stat_statements
 WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND query NOT LIKE '%pg_stat_statements%'`).Scan(&work.Pages))
			for _, statement := range statements {
				if isTransactionControl(statement.sql) {
					if strings.HasPrefix(strings.ToLower(strings.TrimSpace(statement.sql)), "begin") {
						work.Transactions++
					}
					continue
				}
				work.Statements++
				sorts, seqScans := explainPlanShape(t, explainer, statement)
				work.Sorts += sorts
				work.SeqScans += seqScans
			}
		}()
	}
	// Rolled-back writes leave dead tuples behind; vacuum them so the next
	// scenario sees the seeded state.
	if scenario.write {
		fixtureExec(t, pool, "VACUUM")
	}
	return work
}

func isTransactionControl(sql string) bool {
	word, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(sql)), " ")
	switch word {
	case "begin", "start", "commit", "end", "rollback", "savepoint", "release":
		return true
	}
	return false
}

// explainPlanShape counts the Sort nodes and the sequential scans of large tables
// in the plan of a captured statement. EXPLAIN without ANALYZE does not run it.
func explainPlanShape(t *testing.T, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, statement recordedStatement) (sorts, seqScans int64) {
	t.Helper()
	rows, err := q.Query(context.Background(), "EXPLAIN (FORMAT JSON) "+statement.sql, statement.args...)
	require.NoError(t, err)
	plans, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[[]struct {
		Plan planNode `json:"Plan"`
	}])
	require.NoError(t, err)
	var visit func(planNode)
	visit = func(node planNode) {
		switch node.NodeType {
		case "Sort", "Incremental Sort":
			sorts++
		case "Seq Scan":
			if queryWorkLargeTables[node.RelationName] {
				seqScans++
			}
		}
		for _, child := range node.Plans {
			visit(child)
		}
	}
	for _, plan := range plans {
		visit(plan.Plan)
	}
	return sorts, seqScans
}

type planNode struct {
	NodeType     string     `json:"Node Type"`
	RelationName string     `json:"Relation Name"`
	Plans        []planNode `json:"Plans"`
}
