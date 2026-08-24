//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	handlers "github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/tests/mocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ogen-go/ogen/validate"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/mock/gomock"
)

type contextKey string

const (
	testUserHeaderKey contextKey = "test_user_id"
	testCookieKey     contextKey = "test_cookie"
)

func withTestUser(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, testUserHeaderKey, userID.String())
}

func withTestCookie(ctx context.Context, sessionID uuid.UUID) context.Context {
	return context.WithValue(ctx, testCookieKey, sessionID.String())
}

type testSecuritySource struct{}

func (testSecuritySource) CookieAuth(ctx context.Context, operationName corev1.OperationName) (corev1.CookieAuth, error) {
	return corev1.CookieAuth{}, nil
}

type IntegrationTestSuite struct {
	suite.Suite
	ctx         context.Context
	pgContainer *postgres.PostgresContainer
	dbPool      *pgxpool.Pool
	handler     http.Handler
	transport   *testTransport
	testServer  *httptest.Server
	client      *corev1.Client

	ctrl *gomock.Controller

	mockNats *mocks.MockPublisher

	// Repositories (for direct DB access in tests)
	usersRepo         *pg.UsersRepo
	contestsRepo      *pg.ContestsRepo
	organizationsRepo interfaces.OrganizationsRepo
	teamsRepo         interfaces.TeamsRepo
	problemsRepo      *pg.ProblemsRepo
}

func (s *IntegrationTestSuite) getStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var scErr *validate.UnexpectedStatusCodeError
	if errors.As(err, &scErr) {
		return scErr.StatusCode
	}
	return 0
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
	s.mockNats = mocks.NewMockPublisher(s.ctrl)

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
	s.problemsRepo = pg.NewProblemsRepo(s.dbPool)
	s.teamsRepo = pg.NewTeamsRepo(s.dbPool)
	submissionsRepo := pg.NewSubmissionsRepo(s.dbPool)
	outboxRepo := pg.NewOutboxRepo(s.dbPool)
	blogsRepo := pg.NewBlogsRepo(s.dbPool)
	txManager := pg.NewTransactor(s.dbPool)
	tempStoragePath := s.T().TempDir()
	testStorage := storage.NewLocalStorage(tempStoragePath)
	workspaceStorage := usecase.NewWorkspaceStorage(testStorage, "integration-workshop")

	authRepo := pg.NewAuthRepo(s.dbPool)
	emailService := &email.LogEmailService{}

	// UseCases
	usersUC := usecase.NewUsersUseCase(s.usersRepo, s.contestsRepo, outboxRepo, txManager, authRepo, emailService)
	authUC := usecase.NewAuthUseCase(s.usersRepo, authRepo, txManager, emailService)
	problemsUC := usecase.NewProblemsUseCase(s.problemsRepo, s.organizationsRepo)
	contestsUC := usecase.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, submissionsRepo)
	permissionsUC := usecase.NewPermissionsUseCase(s.contestsRepo, usersUC, s.problemsRepo, s.teamsRepo, s.organizationsRepo)
	submissionsUC := usecase.NewSubmissionsUseCase(submissionsRepo, contestsUC, problemsUC, outboxRepo, txManager)
	notificationsRepo := pg.NewNotificationsRepo(s.dbPool)
	notificationsUC := usecase.NewNotificationsUseCase(notificationsRepo, s.usersRepo, emailService)
	contestsUC.SetNotificationsUC(notificationsUC)
	contestsUC.SetUsersRepo(s.usersRepo)
	organizationsUC := usecase.NewOrganizationsUseCase(s.organizationsRepo, s.usersRepo, permissionsUC, txManager, notificationsUC)
	teamsUC := usecase.NewTeamsUseCase(s.teamsRepo, s.organizationsRepo, s.usersRepo, permissionsUC, txManager)
	blogsUC := usecase.NewBlogsUseCase(blogsRepo, nil, "")
	workshopUC := usecase.NewWorkshopUseCase(s.problemsRepo, workspaceStorage, nil, txManager)

	avatarsUC := usecase.NewAvatarsUseCase(s.usersRepo, testStorage, "avatars")

	packagesRepo := pg.NewPackagesRepo(s.dbPool)
	importUC := usecase.NewProblemImportUseCase(s.problemsRepo, workspaceStorage)
	publishUC := usecase.NewProblemPublishUseCase(s.problemsRepo, packagesRepo, workspaceStorage, testStorage, "packages")

	draftsRepo := pg.NewDraftsRepo(s.dbPool)
	draftsUC := usecase.NewDraftsUseCase(draftsRepo, contestsUC, permissionsUC, txManager)

	announcementsRepo := pg.NewAnnouncementsRepo(s.dbPool)
	announcementsUC := usecase.NewAnnouncementsUseCase(announcementsRepo, contestsUC, outboxRepo, txManager)
	clarificationsRepo := pg.NewClarificationsRepo(s.dbPool)
	clarificationsUC := usecase.NewClarificationsUseCase(clarificationsRepo, announcementsRepo, contestsUC, outboxRepo, txManager)

	// Handler
	coreServer := handlers.NewCoreServer(
		authUC,
		contestsUC,
		permissionsUC,
		submissionsUC,
		usersUC,
		problemsUC,
		organizationsUC,
		teamsUC,
		workshopUC,
		blogsUC,
		avatarsUC,
		importUC,
		publishUC,
		draftsUC,
		notificationsUC,
		announcementsUC,
		clarificationsUC,
		nil, // natsJS - not needed for integration tests
		nil, // bookletCompiler - not needed for integration tests
	)

	// Server setup
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	secHandler := middleware.NewSecurityHandler()
	authzMw := middleware.AuthzMiddleware(permissionsUC, submissionsUC, s.organizationsRepo, s.contestsRepo)
	server, err := corev1.NewServer(
		coreServer,
		secHandler,
		corev1.WithMiddleware(authzMw),
		corev1.WithErrorHandler(middleware.ResponseErrorHandler(logger)),
	)
	s.Require().NoError(err)

	s.transport = &testTransport{}
	s.handler = middleware.RequestLoggerMiddleware(logger)(
		middleware.AuthMiddleware(authUC)(
			middleware.UsersMiddleware(usersUC)(
				s.testMiddleware(
					middleware.ResponseWriterMiddleware(server),
				),
			),
		),
	)
	s.transport.handler = s.handler

	s.client, err = corev1.NewClient("http://test-server", testSecuritySource{}, corev1.WithClient(&http.Client{
		Transport: s.transport,
	}))
	s.Require().NoError(err)
}

type testTransport struct {
	handler      http.Handler
	lastResponse *http.Response
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil && req.ContentLength == 0 {
		req.ContentLength = -1
	}
	if uid, ok := req.Context().Value(testUserHeaderKey).(string); ok && uid != "" {
		req.Header.Set("X-Test-User-ID", uid)
	}
	if sessionID, ok := req.Context().Value(testCookieKey).(string); ok && sessionID != "" {
		req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
	}
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, req)
	resp := w.Result()
	t.lastResponse = resp
	return resp, nil
}

func (s *IntegrationTestSuite) testMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("X-Test-User-ID")
		if userIDStr != "" {
			uid, err := uuid.Parse(userIDStr)
			if err == nil {
				user, err := s.usersRepo.GetUserById(r.Context(), uid)
				if err == nil {
					ctx := middleware.WithUser(r.Context(), user)
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
