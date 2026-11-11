//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	"github.com/alicebob/miniredis/v2"
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	access "github.com/brawler2011/gate/backend/internal/access"
	blogs "github.com/brawler2011/gate/backend/internal/blogs"
	contests "github.com/brawler2011/gate/backend/internal/contests"
	"github.com/brawler2011/gate/backend/internal/coremodules"
	identity "github.com/brawler2011/gate/backend/internal/identity"
	notifications "github.com/brawler2011/gate/backend/internal/notifications"
	organizations "github.com/brawler2011/gate/backend/internal/organizations"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	handlers "github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/tests/mocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ogen-go/ogen/validate"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
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
	usersRepo         *pgstore.UsersRepo
	contestsRepo      *pgstore.ContestsRepo
	organizationsRepo *pgstore.OrganizationsRepo
	teamsRepo         *pgstore.TeamsRepo
	problemsRepo      *pgstore.ProblemsRepo
	storage           storage.Storage
	objectStore       *storage.S3Storage
	bucket            string
	problemDrafts     *pgstore.ProblemDraftsRepo
	rateRedis         *miniredis.Miniredis
	limiter           *ratelimit.Limiter
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
		if err := terminate(s.pgContainer); err != nil {
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
	_, err := s.dbPool.Exec(s.ctx, "TRUNCATE TABLE organizations, users CASCADE")
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) initApp() {
	s.rateRedis = miniredis.RunT(s.T())
	rateClient := redis.NewClient(&redis.Options{Addr: s.rateRedis.Addr(), MaxRetries: -1})
	s.T().Cleanup(func() { _ = rateClient.Close() })
	s.limiter = ratelimit.New(rateClient)
	// Repositories
	s.usersRepo = pgstore.NewUsersRepo(s.dbPool)
	s.contestsRepo = pgstore.NewContestsRepo(s.dbPool)
	s.organizationsRepo = pgstore.NewOrganizationsRepo(s.dbPool)
	s.problemsRepo = pgstore.NewProblemsRepo(s.dbPool)
	s.teamsRepo = pgstore.NewTeamsRepo(s.dbPool)
	submissionsRepo := pgstore.NewSubmissionsRepo(s.dbPool)
	blogsRepo := pgstore.NewBlogsRepo(s.dbPool)
	tempStoragePath := s.T().TempDir()
	testStorage := storage.NewLocalStorage(tempStoragePath)
	s.storage = testStorage

	authRepo := pgstore.NewAuthRepo(s.dbPool)
	emailService := &email.LogEmailService{}

	// UseCases
	usersUC := identity.NewUsersUseCase(s.usersRepo, pgstore.NewIdentityTransactions(s.dbPool), emailService)
	authUC := identity.NewAuthUseCase(s.usersRepo, authRepo, pgstore.NewIdentityTransactions(s.dbPool), emailService, s.limiter)
	packagesRepo := pgstore.NewPackagesRepo(s.dbPool)
	permissionsUC := access.NewMetadataPermissions(access.NewPermissionsUseCase(s.contestsRepo, s.usersRepo, s.problemsRepo, s.teamsRepo, s.organizationsRepo), pgstore.NewPolicyFactsRepo(s.dbPool))
	quotas := problemsmodel.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 100, Tests: 100, Components: 100, Subtasks: 100}
	problemDrafts := pgstore.NewProblemDraftsRepo(s.dbPool, quotas, time.Minute)
	// Problem objects live in a real S3-compatible store, as in production.
	objectStore, bucket := newSeaweedBackedS3Storage(s.T())
	s.objectStore, s.bucket, s.problemDrafts = objectStore, bucket, problemDrafts
	packageContent := &problems.PackageContent{Repo: problemDrafts, Store: objectStore, Bucket: bucket, TempDir: diskTempDir(s.T()), MaxArchiveBytes: 1 << 30, MaxEntries: 1000}
	draftUC := &problems.DraftUseCase{DraftQuotas: quotas, Organizations: s.organizationsRepo, Templates: packagesRepo, PackageContent: packageContent, Repo: problemDrafts, Problems: s.problemsRepo, Permissions: permissionsUC, Store: objectStore, Bucket: bucket, TempDir: diskTempDir(s.T()), UploadTTL: time.Minute, ObjectBytes: 32 << 20, BuildOptions: testBuildOptions(s.T())}
	problemsUC := problems.NewProblemsUseCase(s.problemsRepo, pgstore.NewProblemTransactions(s.dbPool), s.organizationsRepo, permissionsUC, draftUC, packagesRepo, packageContent)
	packagesUC := &problems.PackageUseCase{Repo: packagesRepo, Content: packageContent, Permissions: permissionsUC}
	contestsUC := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), permissionsUC)
	submissionsUC := submissions.NewSubmissionsUseCase(submissionsRepo, s.contestsRepo, s.problemsRepo, pgstore.NewSubmissionTransactions(s.dbPool), permissionsUC)
	notificationsRepo := pgstore.NewNotificationsRepo(s.dbPool)
	notificationsUC := notifications.NewNotificationsUseCase(notificationsRepo, s.usersRepo, emailService)
	contestsUC.SetNotificationsUC(notificationsUC)

	organizationsUC := organizations.NewOrganizationsUseCase(s.organizationsRepo, s.usersRepo, permissionsUC, pgstore.NewOrganizationTransactions(s.dbPool), notificationsUC)
	teamsUC := organizations.NewTeamsUseCase(s.teamsRepo, permissionsUC, pgstore.NewTeamTransactions(s.dbPool), permissionsUC)
	blogsUC := blogs.NewBlogsUseCase(pgstore.NewBlogTransactions(s.dbPool), nil, "")

	avatarsUC := identity.NewAvatarsUseCase(s.usersRepo, pgstore.NewIdentityTransactions(s.dbPool), testStorage, "avatars")

	draftsUC := contests.NewDraftsUseCase(pgstore.NewDraftTransactions(s.dbPool), pgstore.NewDraftsRepo(s.dbPool), permissionsUC)

	announcementsRepo := pgstore.NewAnnouncementsRepo(s.dbPool)
	announcementsUC := contests.NewAnnouncementsUseCase(pgstore.NewContestEventsTransactions(s.dbPool))
	clarificationsRepo := pgstore.NewClarificationsRepo(s.dbPool)
	clarificationsUC := contests.NewClarificationsUseCase(pgstore.NewContestEventsTransactions(s.dbPool))
	communicationReader := contests.NewCommunicationReader(announcementsRepo, clarificationsRepo, permissionsUC)
	contestDetails := contests.NewDetailsReader(s.contestsRepo, packageContent, permissionsUC, nil, s.usersRepo)

	// Handler
	coreServer := handlers.NewCoreServer(
		coremodules.Identity{
			AuthUC:      authUC,
			UsersUC:     usersUC,
			UsersReader: s.usersRepo,
			AvatarsUC:   avatarsUC,
		},
		coremodules.Organizations{
			OrganizationsUC: organizationsUC,
			TeamsUC:         teamsUC,
		},
		coremodules.Contests{
			ContestsUC:           contestsUC,
			PermissionsUC:        permissionsUC,
			ContestsReader:       contests.NewReader(s.contestsRepo, permissionsUC),
			ContestDetails:       contestDetails,
			DraftsUC:             draftsUC,
			DraftsReader:         draftsUC,
			AnnouncementsUC:      announcementsUC,
			AnnouncementsReader:  communicationReader,
			ClarificationsUC:     clarificationsUC,
			ClarificationsReader: communicationReader,
		},
		coremodules.Problems{
			ProblemsUC:     problemsUC,
			ProblemsReader: problems.NewReader(s.problemsRepo, permissionsUC, s.organizationsRepo, problemDrafts),
			ProblemDraftUC: draftUC,
			PackagesReader: packagesUC,
		},
		coremodules.Submissions{SubmissionsUC: submissionsUC},
		coremodules.Notifications{
			NotificationsUC:     notificationsUC,
			NotificationsReader: notificationsRepo,
		},
		coremodules.Blogs{
			BlogsUC:     blogsUC,
			BlogsReader: blogsRepo,
		},
		nil,
		false,
		access.NewCapabilities(pgstore.NewPolicyFactsRepo(s.dbPool)),
	)
	langs, err := sandbox.LoadConfig("../../../judge/langs.toml")
	s.Require().NoError(err)
	coreServer.WithLanguages(langs.Langs)

	// Server setup
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	secHandler := middleware.NewSecurityHandler()
	authzMw := middleware.AuthzMiddleware()
	server, err := corev1.NewServer(
		coreServer,
		secHandler,
		corev1.WithMiddleware(authzMw),
		corev1.WithErrorHandler(middleware.ResponseErrorHandler(logger)),
	)
	s.Require().NoError(err)

	s.transport = &testTransport{}
	s.handler = middleware.RequestLoggerMiddleware(logger, func(r *http.Request) string {
		if route, ok := server.FindRoute(r.Method, r.URL.Path); ok {
			return route.PathPattern()
		}
		return "unmatched"
	})(
		middleware.AuthMiddleware(authUC)(
			middleware.UsersMiddleware(s.usersRepo)(
				s.testMiddleware(
					middleware.RateLimit(s.limiter, "192.0.2.254")(server),
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
					ctx := accessmodel.WithUser(r.Context(), user)
					r = r.WithContext(ctx)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
