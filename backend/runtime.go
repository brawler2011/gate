package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	handlers "github.com/brawler2011/gate/backend/internal/transport/rest/core"
	wsobserver "github.com/brawler2011/gate/backend/internal/transport/ws/observer"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
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
	defaultS3ObjectBucket   = "problem-objects"
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
	pool, err := pkg.NewPostgresDB(cfg.PostgresDSN)
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
		defaultS3AvatarBucket: "avatar",
		defaultS3BlogBucket:   "blog",
	} {
		if err := store.EnsureBucket(context.Background(), bucket); err != nil {
			return fmt.Errorf("ensure %s bucket %q: %w", name, bucket, err)
		}
	}

	redisClient, err := pkg.NewRedisClient(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("create rate limiter redis client: %w", err)
	}
	defer func() { _ = redisClient.Close() }()
	limiter := ratelimit.New(redisClient)
	authRepo := pgstore.NewAuthRepo(pool)
	emailService := email.NewEmailService(cfg.Env, cfg.ResendAPIKey, cfg.EmailFrom, cfg.AppBaseURL)

	usersUC := identity.NewUsersUseCase(usersRepo, pgstore.NewIdentityTransactions(pool), emailService)
	authUC := identity.NewAuthUseCase(usersRepo, authRepo, pgstore.NewIdentityTransactions(pool), emailService, limiter)
	notificationsUC := notifications.NewNotificationsUseCase(notificationsRepo, usersRepo, emailService)

	avatarsUC := identity.NewAvatarsUseCase(usersRepo, pgstore.NewIdentityTransactions(pool), store, defaultS3AvatarBucket)
	permissionsUC := access.NewMetadataPermissions(access.NewPermissionsUseCase(contestsRepo, usersRepo, problemsRepo, teamsRepo, orgsRepo), pgstore.NewPolicyFactsRepo(pool))
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

	sandboxClient, err := sandbox.NewClient(cfg.GoJudgeGRPCAddr)
	if err != nil {
		return fmt.Errorf("initialize sandbox client: %w", err)
	}
	defer sandboxClient.Close()
	logger.Info("successfully initialized sandbox client", slog.String("addr", cfg.GoJudgeGRPCAddr))

	sandboxCfg, err := sandbox.LoadConfig("judge/langs.toml")
	if err != nil {
		return fmt.Errorf("failed to load sandbox config: %w", err)
	}

	sandboxInstance := sandbox.NewSandbox(sandboxClient, sandboxCfg)
	problemPolicy, err := config.LoadProblemPolicy(cfg.ProblemPolicyPath)
	if err != nil {
		return fmt.Errorf("load problem policy: %w", err)
	}
	if err := os.MkdirAll(cfg.ProblemWorkDir, 0700); err != nil {
		return fmt.Errorf("create problem work directory: %w", err)
	}
	fileTransport, err := sandbox.NewFileTransport(cfg.GoJudgeHTTPURL, &http.Client{Timeout: problemPolicy.UploadTTL})
	if err != nil {
		return err
	}
	objectStore := store
	if err := objectStore.EnsureBucket(context.Background(), defaultS3ObjectBucket); err != nil {
		return err
	}
	problemDrafts := pgstore.NewProblemDraftsRepo(pool, problemPolicy.Draft, problemPolicy.OperationLease)
	toolchain := problemsmodel.Toolchain{Digest: cfg.GoJudgeToolchainDigest, Architecture: cfg.GoJudgeArchitecture, ABI: cfg.GoJudgeABI, Languages: sandboxCfg.Langs}
	buildOptions := problemsmodel.BuildOptions{Toolchain: toolchain, Limits: problemPolicy.Limits, MaxAttempts: problemPolicy.Attempts, Duration: problemPolicy.Duration, Lease: problemPolicy.Lease}
	draftUC := &problems.DraftUseCase{DraftQuotas: problemPolicy.Draft, Organizations: orgsRepo, Templates: packagesRepo, Repo: problemDrafts, Problems: problemsRepo, Permissions: permissionsUC, Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: cfg.ProblemWorkDir, UploadTTL: problemPolicy.UploadTTL, ObjectBytes: int64(problemPolicy.Limits.ObjectBytes), BuildOptions: buildOptions}
	pipeline := &builder.Pipeline{Repo: problemDrafts, Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: cfg.ProblemWorkDir, Client: sandboxClient, Files: fileTransport, Environment: toolchain, UploadTTL: problemPolicy.UploadTTL, Logger: logger}
	buildWorker := &builder.Worker{Pipeline: pipeline, Parallelism: problemPolicy.Parallelism, Lease: problemPolicy.Lease, Poll: problemPolicy.Poll}
	problemGC := &builder.GC{Repo: problemDrafts, Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: cfg.ProblemWorkDir, ObjectRetention: problemPolicy.ObjectRetention, BuildRetention: problemPolicy.BuildRetention, PackageRetention: problemPolicy.PackageRetention, Interval: problemPolicy.GCInterval, Logger: logger}

	packageContent := &problems.PackageContent{Repo: problemDrafts, Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: cfg.ProblemWorkDir, MaxArchiveBytes: int64(problemPolicy.Limits.ObjectBytes), MaxEntries: problemPolicy.Limits.FileCount}
	draftUC.PackageContent = packageContent
	problemsUC := problems.NewProblemsUseCase(problemsRepo, pgstore.NewProblemTransactions(pool), orgsRepo, permissionsUC, draftUC, packagesRepo, packageContent)
	packagesUC := &problems.PackageUseCase{Repo: packagesRepo, Content: packageContent, Permissions: permissionsUC}
	contestDetails := contests.NewDetailsReader(contestsRepo, packageContent, permissionsUC, sandboxInstance, usersRepo)

	natsJS, err := pkg.NewNatsJetStream(cfg.NatsURL)
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

	judgeJobs := pgstore.NewJudgeJobs(pool)
	judgeTransactions := pgstore.NewJudgeTransactions(pool)
	judgeUC := judging.NewJudgeUseCase(judgeJobs, judgeTransactions, submissionsRepo,
		&judging.PackageLoader{Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: cfg.ProblemWorkDir, Packages: problemDrafts, Limits: problemPolicy.Limits},
		&judging.PackageJudge{Client: sandboxClient, Files: fileTransport, Environment: toolchain, Logger: logger}, judgeworker.NewEventPublisher(natsJS))

	judgeWorker, err := judgeworker.NewJudgeWorker(context.Background(), natsJS, judgeUC, cfg.JudgeWorkerCount)
	if err != nil {
		return fmt.Errorf("create judge worker: %w", err)
	}

	judgeRecovery := judging.NewJudgeRecovery(
		judgeJobs,
		judgeTransactions,
		judging.DefaultMaxJobAttempts,
		judging.DefaultRecoveryBatchSize,
		judging.DefaultRecoveryInterval,
	)

	submissionDispatcher := outbox.NewEventDispatcher()
	observerHub := wsobserver.NewHub(submissionsRingBufferSize, contestsRepo, contestsUC, pgstore.NewPolicyFactsRepo(pool))

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

	observer := wsobserver.NewObserver(observerHub, contestsObserverHub, newObserverMiddleware(usersRepo, authUC), authUC, permissionsUC, contestsRepo, pgstore.NewPolicyFactsRepo(pool))

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
			ProblemsReader: problems.NewReader(problemsRepo, permissionsUC, orgsRepo, problemDrafts),
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
		natsJS,
		cfg.Env != "local",
		access.NewCapabilities(pgstore.NewPolicyFactsRepo(pool)),
	).WithLanguages(sandboxCfg.Langs)

	secHandler := middleware.NewSecurityHandler()
	authzMw := middleware.AuthzMiddleware()
	apiServer, err := corev1.NewServer(
		coreServer,
		secHandler,
		corev1.WithMiddleware(authzMw),
		corev1.WithMaxMultipartMemory(64*1024),
		corev1.WithErrorHandler(middleware.ResponseErrorHandler(logger)),
	)
	if err != nil {
		return fmt.Errorf("create core api server: %w", err)
	}

	apiHandler := middleware.AuthMiddleware(authUC)(
		middleware.UsersMiddleware(usersRepo)(
			middleware.RateLimit(limiter, cfg.TrustedProxy)(middleware.RequestLimits(apiServer, func(operation string) int64 {
				switch operation {
				case "SaveProblemDraft":
					return int64(problemPolicy.RequestBytes)
				case "ImportProblem":
					return int64(problemPolicy.Limits.ObjectBytes + bytesize.MiB)
				case "UploadProblemDraft":
					return int64(2*problemPolicy.Limits.ObjectBytes + bytesize.MiB)
				default:
					return handlers.RequestBodyLimit(operation)
				}
			}, middleware.ResponseErrorHandler(logger))),
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
		ReadTimeout:       problemPolicy.UploadTTL,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	services := []appService{
		newService("problem build worker", buildWorker.Run, func(context.Context) error { return nil }),
		newService("problem object GC", problemGC.Run, func(context.Context) error { return nil }),
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
			"judge recovery worker",
			func(ctx context.Context) error {
				judgeRecovery.Start(ctx)
				return nil
			},
			func(context.Context) error {
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
