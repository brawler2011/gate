package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/brawler2011/gate/backend/config"
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	access "github.com/brawler2011/gate/backend/internal/access"
	blogs "github.com/brawler2011/gate/backend/internal/blogs"
	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/coremodules"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	identity "github.com/brawler2011/gate/backend/internal/identity"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	inbox "github.com/brawler2011/gate/backend/internal/messaging/inbox"
	judgeworker "github.com/brawler2011/gate/backend/internal/messaging/judge"
	outbox "github.com/brawler2011/gate/backend/internal/messaging/outbox"
	pubsub "github.com/brawler2011/gate/backend/internal/messaging/pubsub"
	notifications "github.com/brawler2011/gate/backend/internal/notifications"
	organizations "github.com/brawler2011/gate/backend/internal/organizations"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	handlers "github.com/brawler2011/gate/backend/internal/transport/rest/core"
	wsobserver "github.com/brawler2011/gate/backend/internal/transport/ws/observer"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

const submissionsRingBufferSize = 10000

const serviceShutdownTimeout = 5 * time.Second

const (
	defaultS3Region         = "us-east-1"
	defaultS3AvatarBucket   = "avatars"
	defaultS3PackageBucket  = "problem-packages"
	defaultS3WorkshopBucket = "problem-workspaces"
	defaultS3BlogBucket     = "blog-images"
)

type appService interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type serviceFunc struct {
	name  string
	start func(ctx context.Context) error
	stop  func(ctx context.Context) error
}

func (s serviceFunc) Name() string {
	return s.name
}

func (s serviceFunc) Start(ctx context.Context) error {
	return s.start(ctx)
}

func (s serviceFunc) Stop(ctx context.Context) error {
	return s.stop(ctx)
}

func newService(
	name string,
	start func(ctx context.Context) error,
	stop func(ctx context.Context) error,
) appService {
	return serviceFunc{
		name:  name,
		start: start,
		stop:  stop,
	}
}

func newHTTPService(name string, server *http.Server) appService {
	return newService(
		name,
		func(context.Context) error {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("http server ListenAndServe: %w", err)
			}
			return nil
		},
		func(ctx context.Context) error {
			if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("http server Shutdown: %w", err)
			}
			return nil
		},
	)
}

func runApp(envFile string) error {
	cfg, err := loadConfig(envFile)
	if err != nil {
		return err
	}

	logger := slog.Default()

	logger.Info("connecting to postgres")
	pool, err := pkg.NewPostgresDB(cfg.GetPostgresDSN())
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("successfully connected to postgres")

	usersRepo := pgstore.NewUsersRepo(pool)
	outboxRepo := pgstore.NewOutboxRepo(pool)
	orgsRepo := pgstore.NewOrganizationsRepo(pool)
	teamsRepo := pgstore.NewTeamsRepo(pool)
	problemsRepo := pgstore.NewProblemsRepo(pool)
	contestsRepo := pgstore.NewContestsRepo(pool)
	blogsRepo := pgstore.NewBlogsRepo(pool)
	submissionsRepo := pgstore.NewSubmissionsRepo(pool)
	packagesRepo := pgstore.NewPackagesRepo(pool)
	notificationsRepo := pgstore.NewNotificationsRepo(pool)
	announcementsRepo := pgstore.NewAnnouncementsRepo(pool)
	clarificationsRepo := pgstore.NewClarificationsRepo(pool)
	logger.Info("successfully initialized repositories")

	var store storage.Storage
	if cfg.StorageType == "s3" || (cfg.StorageType != "local" && cfg.S3Endpoint != "") {
		store = storage.NewS3Storage(storage.S3Config{
			Endpoint:  cfg.S3Endpoint,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
			Region:    defaultS3Region,
		})
		logger.Info("successfully initialized S3 storage client")
	} else {
		store = storage.NewLocalStorage(cfg.LocalStoragePath)
		logger.Info("successfully initialized Local filesystem storage client", slog.String("path", cfg.LocalStoragePath))
	}

	for bucket, name := range map[string]string{
		defaultS3AvatarBucket:   "avatar",
		defaultS3PackageBucket:  "package",
		defaultS3WorkshopBucket: "workshop",
		defaultS3BlogBucket:     "blog",
	} {
		if err := store.EnsureBucket(context.Background(), bucket); err != nil {
			return fmt.Errorf("ensure %s bucket %q: %w", name, bucket, err)
		}
	}

	workspaceStorage := problems.NewWorkspaceStorage(store, defaultS3WorkshopBucket, problems.NewWorkspaceCatalog(problemsRepo, pgstore.NewProblemTransactions(pool)))

	authRepo := pgstore.NewAuthRepo(pool)
	emailService := email.NewEmailService(cfg.Env, cfg.ResendAPIKey, cfg.EmailFrom, cfg.AppBaseURL)

	usersUC := identity.NewUsersUseCase(usersRepo, pgstore.NewIdentityTransactions(pool), emailService)
	authUC := identity.NewAuthUseCase(usersRepo, authRepo, pgstore.NewIdentityTransactions(pool), emailService)
	notificationsUC := notifications.NewNotificationsUseCase(notificationsRepo, usersRepo, emailService)

	avatarsUC := identity.NewAvatarsUseCase(usersRepo, pgstore.NewIdentityTransactions(pool), store, defaultS3AvatarBucket)
	permissionsUC := access.NewMetadataPermissions(access.NewPermissionsUseCase(contestsRepo, usersRepo, problemsRepo, teamsRepo, orgsRepo), pgstore.NewPolicyFactsRepo(pool))
	problemImportUC := problems.NewProblemImportUseCase(problemsRepo, workspaceStorage, pgstore.NewProblemTransactions(pool), permissionsUC)
	problemPublishUC := problems.NewProblemPublishUseCase(problemsRepo, packagesRepo, workspaceStorage, store, defaultS3PackageBucket, pgstore.NewProblemTransactions(pool), permissionsUC)
	problemsUC := problems.NewProblemsUseCase(problemsRepo, workspaceStorage, pgstore.NewProblemTransactions(pool), problemPublishUC, orgsRepo, permissionsUC)
	contestsUC := contests.NewContestsUseCase(contestsRepo, orgsRepo, pgstore.NewContestCommandTransactions(pool), permissionsUC)
	contestsUC.SetNotificationsUC(notificationsUC)

	blogsUC := blogs.NewBlogsUseCase(pgstore.NewBlogTransactions(pool), store, defaultS3BlogBucket)
	orgsUC := organizations.NewOrganizationsUseCase(orgsRepo, usersRepo, permissionsUC, pgstore.NewOrganizationTransactions(pool), notificationsUC)
	teamsUC := organizations.NewTeamsUseCase(teamsRepo, permissionsUC, pgstore.NewTeamTransactions(pool), permissionsUC)
	submissionsUC := submissions.NewSubmissionsUseCase(submissionsRepo, contestsRepo, problemsRepo, pgstore.NewSubmissionTransactions(pool), permissionsUC)
	draftsUC := contests.NewDraftsUseCase(pgstore.NewDraftTransactions(pool), pgstore.NewDraftsRepo(pool), permissionsUC)
	announcementsUC := contests.NewAnnouncementsUseCase(pgstore.NewContestEventsTransactions(pool))
	clarificationsUC := contests.NewClarificationsUseCase(pgstore.NewContestEventsTransactions(pool))
	communicationReader := contests.NewCommunicationReader(announcementsRepo, clarificationsRepo, permissionsUC)
	logger.Info("successfully initialized use cases")

	judgeTempDir, err := prepareJudgeTempDir(cfg.JudgeTempDir)
	if err != nil {
		return fmt.Errorf("prepare judge temp directory: %w", err)
	}

	sandboxClient, err := sandbox.NewClient(cfg.GoJudgeGRPCAddr)
	if err != nil {
		return fmt.Errorf("initialize sandbox client: %w", err)
	}
	defer sandboxClient.Close()
	logger.Info("successfully initialized sandbox client", slog.String("addr", cfg.GoJudgeGRPCAddr))

	sandboxCfg, err := sandbox.LoadConfig("languages.yaml")
	if err != nil {
		return fmt.Errorf("failed to load sandbox config: %w", err)
	}

	sandboxInstance := sandbox.NewSandbox(sandboxClient, sandboxCfg)
	contestDetails := contests.NewDetailsReader(contestsRepo, problemsRepo, problems.NewStatementReader(workspaceStorage, packagesRepo, problemPublishUC), permissionsUC, sandboxInstance, usersRepo)
	workshopUC := problems.NewWorkshopUseCase(workspaceStorage, sandboxInstance, permissionsUC)

	natsJS, err := pkg.NewNatsJetStream(cfg.GetNatsURL())
	if err != nil {
		return fmt.Errorf("create nats jetstream connection: %w", err)
	}
	logger.Info("successfully initialized nats jetstream")

	if err := pkg.EnsureSubmissionsStream(context.Background(), natsJS); err != nil {
		return fmt.Errorf("ensure submissions stream: %w", err)
	}
	if err := pkg.EnsureContestsStream(context.Background(), natsJS); err != nil {
		return fmt.Errorf("ensure contests stream: %w", err)
	}
	logger.Info("SUBMISSIONS and CONTESTS streams ready")

	outboxDispatcher := outbox.NewEventDispatcher()
	outboxDispatcher.Register(eventdata.OutboxEventSubmissionCreated, pubsub.NewSubmissionCreatedPublisher(natsJS))
	outboxDispatcher.Register(eventdata.OutboxEventSubmissionCompleted, pubsub.NewSubmissionCreatedPublisher(natsJS))
	contestEventsPublisher := pubsub.NewContestEventsPublisher(natsJS)
	outboxDispatcher.Register(eventdata.OutboxEventContestAnnouncementCreated, contestEventsPublisher)
	outboxDispatcher.Register(eventdata.OutboxEventContestAnnouncementDeleted, contestEventsPublisher)
	outboxDispatcher.Register(eventdata.OutboxEventContestClarificationCreated, contestEventsPublisher)
	outboxDispatcher.Register(eventdata.OutboxEventContestClarificationAnswered, contestEventsPublisher)
	outboxWorker := outbox.NewOutboxWorker(outboxDispatcher, outboxRepo)
	inboxRepo := pgstore.NewInbox(pool)
	inboxPruner := inbox.NewPruner(inboxRepo, cfg.GetInboxRetention(), inbox.DefaultPruneInterval)

	judgeUC := judging.NewJudgeUseCase(
		pgstore.NewJudgeJobs(pool), pgstore.NewJudgeTransactions(pool),
		submissionsRepo,
		store,
		defaultS3PackageBucket,
		judgeTempDir,
		sandboxInstance,
		judgeworker.NewEventPublisher(natsJS),
	)

	judgeWorker, err := judgeworker.NewJudgeWorker(context.Background(), natsJS, judgeUC, cfg.JudgeWorkerCount)
	if err != nil {
		return fmt.Errorf("create judge worker: %w", err)
	}

	submissionDispatcher := outbox.NewEventDispatcher()
	observerHub := wsobserver.NewHub(submissionsRingBufferSize, contestsRepo, contestsUC)

	for _, event := range []submissionsmodel.SubmissionEventType{
		submissionsmodel.SubmissionEventCreated,
		submissionsmodel.SubmissionEventQueued,
		submissionsmodel.SubmissionEventCompilingStarted,
		submissionsmodel.SubmissionEventTestingStarted,
		submissionsmodel.SubmissionEventTestStarted,
		submissionsmodel.SubmissionEventCompleted,
	} {
		submissionDispatcher.Register(event, observerHub)
	}

	submissionsSub, err := pubsub.NewSubmissionsSub(context.Background(), natsJS, submissionDispatcher)
	if err != nil {
		return fmt.Errorf("create submissions subscriber: %w", err)
	}

	contestsDispatcher := outbox.NewEventDispatcher()
	contestsObserverHub := wsobserver.NewContestsHub(submissionsRingBufferSize)

	for _, event := range []string{
		contestsmodel.ContestEventAnnouncementCreated,
		contestsmodel.ContestEventAnnouncementDeleted,
		contestsmodel.ContestEventClarificationCreated,
		contestsmodel.ContestEventClarificationAnswered,
	} {
		contestsDispatcher.Register(event, contestsObserverHub)
	}

	contestsSub, err := pubsub.NewContestsSub(context.Background(), natsJS, contestsDispatcher)
	if err != nil {
		return fmt.Errorf("create contests subscriber: %w", err)
	}

	observer := wsobserver.NewObserver(observerHub, contestsObserverHub, newObserverMiddleware(usersRepo, authUC), authUC, permissionsUC, contestsRepo)

	publicMux := http.NewServeMux()
	publicMux.Handle("/ws/", observer.Handler())

	coreServer := handlers.NewCoreServer(
		coremodules.Identity{
			AuthUC:      authUC,
			UsersUC:     usersUC,
			UsersReader: usersRepo,
			AvatarsUC:   avatarsUC,
		},
		coremodules.Organizations{
			OrganizationsUC: orgsUC,
			TeamsUC:         teamsUC,
		},
		coremodules.Contests{
			ContestsUC:           contestsUC,
			PermissionsUC:        permissionsUC,
			ContestsReader:       contests.NewReader(contestsRepo, permissionsUC),
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
			ProblemsReader: problems.NewReader(problemsRepo, permissionsUC, orgsRepo, workshopUC),
			WorkshopUC:     workshopUC,
			ImportUC:       problemImportUC,
			PublishUC:      problemPublishUC,
			PackagesReader: problemPublishUC,
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
		natsJS,
		cfg.Env != "local",
	)

	secHandler := middleware.NewSecurityHandler()
	authzMw := middleware.AuthzMiddleware(permissionsUC, submissionsUC, orgsRepo, contestsRepo)
	apiServer, err := corev1.NewServer(
		coreServer,
		secHandler,
		corev1.WithMiddleware(authzMw),
		corev1.WithErrorHandler(middleware.ResponseErrorHandler(logger)),
	)
	if err != nil {
		return fmt.Errorf("create core api server: %w", err)
	}

	apiHandler := middleware.AuthMiddleware(authUC)(
		middleware.UsersMiddleware(usersRepo)(
			middleware.RequestLimits(apiServer, handlers.RequestBodyLimit, middleware.ResponseErrorHandler(logger)),
		),
	)

	publicMux.Handle("/api/", http.StripPrefix("/api", apiHandler))
	publicMux.Handle("/health", apiHandler)
	publicMux.Handle("/", spaHandler("dist"))

	publicServer := &http.Server{
		Addr: cfg.Address,
		Handler: middleware.RequestLoggerMiddleware(logger, func(r *http.Request) string {
			if strings.HasPrefix(r.URL.Path, "/ws/") {
				return "/ws/*"
			}
			if route, ok := apiServer.FindRoute(r.Method, strings.TrimPrefix(r.URL.Path, "/api")); ok {
				return route.PathPattern()
			}
			return "unmatched"
		})(publicMux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	services := []appService{
		newService(
			"outbox worker",
			func(ctx context.Context) error {
				outboxWorker.Start(ctx)
				return nil
			},
			func(context.Context) error {
				return nil
			},
		),
		newService(
			"judge worker",
			judgeWorker.Start,
			func(context.Context) error {
				judgeWorker.Stop()
				return nil
			},
		),
		newService(
			"submissions subscriber",
			submissionsSub.Start,
			func(context.Context) error {
				submissionsSub.Stop()
				return nil
			},
		),
		newService(
			"contests subscriber",
			contestsSub.Start,
			func(context.Context) error {
				contestsSub.Stop()
				return nil
			},
		),
		newService(
			"session cleanup worker",
			func(ctx context.Context) error {
				ticker := time.NewTicker(10 * time.Minute)
				defer ticker.Stop()

				// Run first cleanup immediately on startup
				if err := authUC.CleanupExpiredSessions(ctx); err != nil {
					slog.Error("failed to cleanup expired sessions", "error", err)
				}

				for {
					select {
					case <-ctx.Done():
						return nil
					case <-ticker.C:
						if err := authUC.CleanupExpiredSessions(ctx); err != nil {
							slog.Error("failed to cleanup expired sessions", "error", err)
						}
					}
				}
			},
			func(context.Context) error {
				return nil
			},
		),
		newService(
			"inbox receipts pruner",
			func(ctx context.Context) error {
				inboxPruner.Start(ctx)
				return nil
			},
			func(context.Context) error {
				return nil
			},
		),
		newHTTPService("public server", publicServer),
	}

	return runServices(ctx, logger, services)
}

func loadConfig(envFile string) (config.Config, error) {
	var cfg config.Config

	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return cfg, fmt.Errorf("load env file %s: %w", envFile, err)
		}
	}

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	return cfg, nil
}

func newObserverMiddleware(users middleware.UsersUC, authUC middleware.AuthUC) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return middleware.AuthMiddleware(authUC)(
			middleware.UsersMiddleware(users)(next),
		)
	}
}

func prepareJudgeTempDir(tempDir string) (string, error) {
	if tempDir == "" {
		tempDir = filepath.Join(os.TempDir(), "judge")
	}

	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("create judge temp directory: %w", err)
	}

	return tempDir, nil
}

func runServices(ctx context.Context, logger *slog.Logger, services []appService) error {
	g, runCtx := errgroup.WithContext(ctx)
	shutdownErrCh := make(chan error, 1)

	for _, svc := range services {
		svc := svc
		g.Go(func() error {
			logger.Info("starting service", slog.String("service", svc.Name()))
			if err := svc.Start(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("%s: %w", svc.Name(), err)
			}
			return nil
		})
	}

	go func() {
		<-runCtx.Done()
		if errors.Is(ctx.Err(), context.Canceled) {
			logger.Info("shutdown signal received")
		}
		shutdownErrCh <- stopServices(logger, services)
	}()

	runErr := g.Wait()
	shutdownErr := <-shutdownErrCh

	logger.Info("shutdown complete")
	return errors.Join(runErr, shutdownErr)
}

func stopServices(logger *slog.Logger, services []appService) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), serviceShutdownTimeout)
	defer cancel()

	var shutdownErrs []error
	for i := len(services) - 1; i >= 0; i-- {
		svc := services[i]
		logger.Info("stopping service", slog.String("service", svc.Name()))
		if err := svc.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("%s: %w", svc.Name(), err))
		}
	}

	return errors.Join(shutdownErrs...)
}
