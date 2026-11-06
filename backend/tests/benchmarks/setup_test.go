//go:build integration
// +build integration

package benchmarks

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	benchCtx         context.Context
	dbPool           *pgxpool.Pool
	outboxRepo       *pgstore.OutboxRepo
	submissionsRepo  *pgstore.SubmissionsRepo
	contestsRepo     *pgstore.ContestsRepo

	testOrgID        uuid.UUID
	testAdminID      uuid.UUID
	testContestID    uuid.UUID
	testProblemID    uuid.UUID
	testPackageID    uuid.UUID
	testParticipantID uuid.UUID
)

func isBenchmarkRunning() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.bench") || strings.HasPrefix(arg, "-bench") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 && parts[1] != "" {
				return true
			}
			if len(parts) == 1 {
				return true
			}
		}
	}
	return false
}

func TestMain(m *testing.M) {
	if !isBenchmarkRunning() {
		// No benchmarks requested, exit immediately without provisioning DB
		os.Exit(m.Run())
	}

	benchCtx = context.Background()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("GATE_TEST_DB_DSN")
	}

	var container *postgres.PostgresContainer
	if dsn == "" {
		var err error
		container, err = postgres.Run(benchCtx,
			"postgres:18-alpine",
			postgres.WithDatabase("gate_bench"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword("postgres"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(15*time.Second),
			),
		)
		if err != nil {
			log.Fatalf("failed to start postgres container: %v", err)
		}
		defer func() {
			if err := container.Terminate(benchCtx); err != nil {
				log.Printf("failed to terminate container: %v", err)
			}
		}()

		dsn, err = container.ConnectionString(benchCtx, "sslmode=disable")
		if err != nil {
			log.Fatalf("failed to get container connection string: %v", err)
		}
	}

	// Run goose migrations
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)
	migrationsPath := filepath.Join(basepath, "../../migrations")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("failed to open sql connection for migrations: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.Up(db, migrationsPath); err != nil {
		log.Fatalf("failed to run goose migrations: %v", err)
	}

	// Connect pgx pool
	dbPool, err = pkg.NewPostgresDB(dsn)
	if err != nil {
		log.Fatalf("failed to initialize pgx pool: %v", err)
	}
	defer dbPool.Close()

	// Initialize repositories
	outboxRepo = pgstore.NewOutboxRepo(dbPool)
	submissionsRepo = pgstore.NewSubmissionsRepo(dbPool)
	contestsRepo = pgstore.NewContestsRepo(dbPool)

	// Seed baseline fixtures
	if err := seedFixtures(benchCtx); err != nil {
		log.Fatalf("failed to seed baseline fixtures: %v", err)
	}

	code := m.Run()
	os.Exit(code)
}

func seedFixtures(ctx context.Context) error {
	testOrgID = uuid.New()
	testAdminID = uuid.New()
	testContestID = uuid.New()
	testProblemID = uuid.New()
	testPackageID = uuid.New()
	testParticipantID = uuid.New()

	// 1. Create Admin User
	_, err := dbPool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash, role) VALUES ($1, $2, $3, 'admin')`,
		testAdminID, "bench_admin", "hash",
	)
	if err != nil {
		return fmt.Errorf("insert admin user: %w", err)
	}

	// 2. Create Organization
	_, err = dbPool.Exec(ctx,
		`INSERT INTO organizations (id, login, name) VALUES ($1, $2, $3)`,
		testOrgID, "bench_org_"+strings.ReplaceAll(uuid.NewString()[:8], "-", ""), "Bench Org",
	)
	if err != nil {
		return fmt.Errorf("insert org: %w", err)
	}

	// 3. Create Contest
	_, err = dbPool.Exec(ctx,
		`INSERT INTO contests (id, organization_id, owner_id, visibility, title, login, description, settings)
		 VALUES ($1, $2, $3, 'public', 'Benchmark Contest', $4, 'desc', '{}')`,
		testContestID, testOrgID, testAdminID, "bench_contest_"+strings.ReplaceAll(uuid.NewString()[:8], "-", ""),
	)
	if err != nil {
		return fmt.Errorf("insert contest: %w", err)
	}

	// 4. Create Problem
	_, err = dbPool.Exec(ctx,
		`INSERT INTO problems (id, organization_id, owner_id, visibility, title, short_name, time_limit_ms, memory_limit_mb)
		 VALUES ($1, $2, $3, 'public', 'Benchmark Problem', 'bench_p', 1000, 256)`,
		testProblemID, testOrgID, testAdminID,
	)
	if err != nil {
		return fmt.Errorf("insert problem: %w", err)
	}

	// 5. Create Problem Package
	packageHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = dbPool.Exec(ctx,
		`INSERT INTO problem_packages (id, problem_id, organization_id, package_hash, status, version)
		 VALUES ($1, $2, $3, $4, 'ready', 1)`,
		testPackageID, testProblemID, testOrgID, packageHash,
	)
	if err != nil {
		return fmt.Errorf("insert problem package: %w", err)
	}

	// 6. Link Problem to Contest
	_, err = dbPool.Exec(ctx,
		`INSERT INTO contest_problems (contest_id, problem_id, package_id, ordinal)
		 VALUES ($1, $2, $3, 1)`,
		testContestID, testProblemID, testPackageID,
	)
	if err != nil {
		return fmt.Errorf("insert contest problem: %w", err)
	}

	// 7. Create Participant User
	_, err = dbPool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash, role) VALUES ($1, $2, $3, 'user')`,
		testParticipantID, "bench_user_"+strings.ReplaceAll(uuid.NewString()[:8], "-", ""), "hash",
	)
	if err != nil {
		return fmt.Errorf("insert participant user: %w", err)
	}

	// 8. Register Participant in Contest
	_, err = dbPool.Exec(ctx,
		`INSERT INTO contest_members (contest_id, user_id, role) VALUES ($1, $2, 'participant')`,
		testContestID, testParticipantID,
	)
	if err != nil {
		return fmt.Errorf("insert contest member: %w", err)
	}

	return nil
}
