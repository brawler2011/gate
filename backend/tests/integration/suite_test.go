//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	handlers "github.com/gate149/gate/backend/internal/transport/rest/core"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/gate149/gate/backend/tests/mocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/mock/gomock"
)

type IntegrationTestSuite struct {
	suite.Suite
	ctx         context.Context
	pgContainer *postgres.PostgresContainer
	dbPool      *pgxpool.Pool
	handler     http.Handler
	testServer  *httptest.Server
	client      *corev1.ClientWithResponses

	ctrl *gomock.Controller

	mockNats *mocks.MockNatsPublisher

	// Repositories (for direct DB access in tests)
	usersRepo         *pg.UsersRepo
	contestsRepo      *pg.ContestsRepo
	organizationsRepo interfaces.OrganizationsRepo
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.ctx = context.Background()

	// 1. Start Postgres Container
	var err error
	s.pgContainer, err = postgres.Run(s.ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("tester"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(5*time.Second),
		),
	)
	s.Require().NoError(err)

	connStr, err := s.pgContainer.ConnectionString(s.ctx, "sslmode=disable")
	s.Require().NoError(err)

	// 2. Connect to DB
	s.dbPool, err = pkg.NewPostgresDB(connStr)
	s.Require().NoError(err)

	// 3. Run Migrations
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)
	migrationsPath := filepath.Join(basepath, "../../migrations")

	db, err := sql.Open("pgx", connStr)
	s.Require().NoError(err)
	defer db.Close()

	err = goose.SetDialect("postgres")
	s.Require().NoError(err)
	err = goose.Up(db, migrationsPath)
	s.Require().NoError(err)

}

func (s *IntegrationTestSuite) TearDownSuite() {
	if s.dbPool != nil {
		s.dbPool.Close()
	}
	if s.pgContainer != nil {
		if err := s.pgContainer.Terminate(s.ctx); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}
}

func (s *IntegrationTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockNats = mocks.NewMockNatsPublisher(s.ctrl)

	s.initApp()
}

func (s *IntegrationTestSuite) TearDownTest() {
	s.ctrl.Finish()
	// Clean up tables (in reverse dependency order with CASCADE)
	_, err := s.dbPool.Exec(s.ctx, "TRUNCATE TABLE organizations CASCADE")
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) initApp() {
	// Repositories
	s.usersRepo = pg.NewUsersRepo(s.dbPool)
	s.contestsRepo = pg.NewContestsRepo(s.dbPool)
	s.organizationsRepo = pg.NewOrganizationsRepo(s.dbPool)
	problemsRepo := pg.NewProblemsRepo(s.dbPool)
	submissionsRepo := pg.NewSubmissionsRepo(s.dbPool)
	outboxRepo := pg.NewOutboxRepo(s.dbPool)
	teamsRepo := pg.NewTeamsRepo(s.dbPool)
	blogsRepo := pg.NewBlogsRepo(s.dbPool)
	txManager := pg.NewTransactor(s.dbPool)
	vcsService := vcs.NewInMemoryS3Service("integration-workshop")

	// UseCases
	usersUC := usecase.NewUsersUseCase(s.usersRepo, outboxRepo, txManager)
	problemsUC := usecase.NewProblemsUseCase(problemsRepo)
	contestsUC := usecase.NewContestsUseCase(s.contestsRepo)
	permissionsUC := usecase.NewPermissionsUseCase(s.contestsRepo, usersUC, problemsRepo, teamsRepo, s.organizationsRepo)
	submissionsUC := usecase.NewSubmissionsUseCase(submissionsRepo, contestsUC, problemsUC, outboxRepo, txManager)
	organizationsUC := usecase.NewOrganizationsUseCase(s.organizationsRepo, s.usersRepo, permissionsUC, txManager)
	teamsUC := usecase.NewTeamsUseCase(teamsRepo, s.organizationsRepo, s.usersRepo, permissionsUC, txManager)
	blogsUC := usecase.NewBlogsUseCase(blogsRepo, nil, "")
	workshopUC := usecase.NewWorkshopUseCase(problemsRepo, vcsService, nil, txManager)

	// Handler
	coreServer := handlers.NewCoreServer(
		contestsUC,
		permissionsUC,
		submissionsUC,
		usersUC,
		problemsUC,
		organizationsUC,
		teamsUC,
		workshopUC,
		blogsUC,
		nil, // avatarsUC - not needed for integration tests
		nil, // importUC - not needed for integration tests
		nil, // publishUC - not needed for integration tests
		nil, // natsJS - not needed for integration tests
	)

	// Strict Handler
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	strictHandler := corev1.NewStrictHandlerWithOptions(coreServer, []corev1.StrictMiddlewareFunc{
		middleware.AuthzStrictMiddleware(permissionsUC, submissionsUC),
	}, corev1.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
		ResponseErrorHandlerFunc: middleware.ResponseErrorHandler(logger),
	})

	mux := http.NewServeMux()
	corev1.HandlerFromMux(strictHandler, mux)

	// Wrap with test middleware
	s.handler = s.testMiddleware(mux)

	// Initialize Client
	var err error
	s.client, err = corev1.NewClientWithResponses("http://test-server", corev1.WithHTTPClient(&http.Client{
		Transport: &testTransport{handler: s.handler},
	}))
	s.Require().NoError(err)
}

type testTransport struct {
	handler http.Handler
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

func (s *IntegrationTestSuite) testMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("X-Test-User-ID")
		if userIDStr != "" {
			uid, err := uuid.Parse(userIDStr)
			if err == nil {
				user, err := s.usersRepo.GetUserById(r.Context(), uid)
				if err == nil {
					ctx := context.WithValue(r.Context(), "user", user)
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
