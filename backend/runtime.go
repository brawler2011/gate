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
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	identity "github.com/brawler2011/gate/backend/internal/identity"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	"github.com/brawler2011/gate/backend/internal/messaging/booklet"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go/jetstream"
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

func runApp(configFile, envFile string, mode config.Mode) error {
	cfg, err := loadConfig(configFile, envFile, mode)
	if err != nil {
		return err
	}

	logger := slog.Default().With(slog.String("mode", string(mode)))
	slog.SetDefault(logger)

	logger.Info("connecting to postgres")
	pool, err := pkg.NewPostgresDB(cfg.Secrets.PostgresDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("successfully connected to postgres")

	store := storage.NewS3Storage(storage.S3Config{
		Endpoint:  cfg.Storage.S3Endpoint,
		AccessKey: cfg.Secrets.S3AccessKey,
		SecretKey: cfg.Secrets.S3SecretKey,
		Region:    defaultS3Region,
	})
	if err := store.EnsureBucket(context.Background(), defaultS3ObjectBucket); err != nil {
		return err
	}
	logger.Info("successfully initialized S3 storage client")

	if err := os.MkdirAll(cfg.Problems.WorkDir, 0700); err != nil {
		return fmt.Errorf("create problem work directory: %w", err)
	}

	natsJS, err := pkg.NewNatsJetStream(cfg.Secrets.NatsURL)
	if err != nil {
		return fmt.Errorf("create nats jetstream connection: %w", err)
	}
	defer natsJS.Conn().Close()
	if err := pkg.EnsureSubmissionsStream(context.Background(), natsJS); err != nil {
		return fmt.Errorf("ensure submissions stream: %w", err)
	}
	if err := pkg.EnsureContestsStream(context.Background(), natsJS); err != nil {
		return fmt.Errorf("ensure contests stream: %w", err)
	}
	logger.Info("SUBMISSIONS and CONTESTS streams ready")

	shared := sharedDeps{cfg: cfg, logger: logger, pool: pool, store: store, natsJS: natsJS, problemDrafts: pgstore.NewProblemDraftsRepo(pool, cfg.Problems.Draft, cfg.Problems.OperationLease)}

	var services []appService
	var judgeImage sandbox.Image
	if mode.Worker() {
		sandboxClient, err := sandbox.NewClient(cfg.GoJudge.GRPCAddr)
		if err != nil {
			return fmt.Errorf("initialize sandbox client: %w", err)
		}
		defer sandboxClient.Close()
		logger.Info("successfully initialized sandbox client", slog.String("addr", cfg.GoJudge.GRPCAddr))

		// The toolchain identity and languages come from the running judge image itself.
		imageCtx, cancelImage := context.WithTimeout(context.Background(), time.Minute)
		judgeImage, err = sandbox.ReadImage(imageCtx, sandboxClient)
		cancelImage()
		if err != nil {
			return err
		}
		workers, err := workerServices(shared, sandboxClient, judgeImage)
		if err != nil {
			return err
		}
		services = append(services, workers...)
		if !mode.API() {
			services = append(services, newHTTPService("health server", &http.Server{Addr: cfg.HTTP.Address, Handler: workerHealthHandler(), ReadHeaderTimeout: 10 * time.Second}))
		}
	}
	if mode.API() {
		if !mode.Worker() {
			// Copied from the judge image of the same release (deploy/charts/gate).
			judgeImage, err = sandbox.ReadImageDir(cfg.Judge.ManifestDir)
			if err != nil {
				return err
			}
		}
		api, closeAPI, err := apiServices(shared, judgeImage)
		if err != nil {
			return err
		}
		defer closeAPI()
		services = append(services, api...)
	}
	logger.Info("read judge image manifest", slog.String("toolchain_digest", judgeImage.Identity.Digest))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return runServices(ctx, logger, services)
}

// sharedDeps are the connections and stores that API and worker instances both use.
type sharedDeps struct {
	cfg           config.Config
	logger        *slog.Logger
	pool          *pgxpool.Pool
	store         *storage.S3Storage
	natsJS        jetstream.JetStream
	problemDrafts *pgstore.ProblemDraftsRepo
}

func toolchainOf(image sandbox.Image) problemsmodel.Toolchain {
	return problemsmodel.Toolchain{Digest: image.Identity.Digest, Architecture: image.Identity.Architecture, ABI: image.Identity.ABI, Languages: image.Config.Langs}
}

// workerServices judge submissions, build problem packages and compile booklets
// in the sandbox.
func workerServices(d sharedDeps, sandboxClient *sandbox.Client, judgeImage sandbox.Image) ([]appService, error) {
	problemPolicy := d.cfg.Problems
	toolchain := toolchainOf(judgeImage)
	fileTransport, err := sandbox.NewFileTransport(d.cfg.GoJudge.HTTPURL, &http.Client{Timeout: problemPolicy.UploadTTL})
	if err != nil {
		return nil, err
	}

	pipeline := &builder.Pipeline{Repo: d.problemDrafts, Store: d.store, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, Client: sandboxClient, Files: fileTransport, Environment: toolchain, UploadTTL: problemPolicy.UploadTTL, Logger: d.logger}
	buildWorker := &builder.Worker{Pipeline: pipeline, Parallelism: problemPolicy.Parallelism, Lease: problemPolicy.Lease, Poll: problemPolicy.Poll}
	problemGC := &builder.GC{Repo: d.problemDrafts, Store: d.store, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, ObjectRetention: problemPolicy.ObjectRetention, BuildRetention: problemPolicy.BuildRetention, PackageRetention: problemPolicy.PackageRetention, Interval: problemPolicy.GCInterval, Logger: d.logger}

	judgeJobs := pgstore.NewJudgeJobs(d.pool)
	judgeTransactions := pgstore.NewJudgeTransactions(d.pool)
	judgeUC := judging.NewJudgeUseCase(judgeJobs, judgeTransactions, pgstore.NewSubmissionsRepo(d.pool),
		&judging.PackageLoader{Store: d.store, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, Packages: d.problemDrafts, Limits: problemPolicy.Limits},
		&judging.PackageJudge{Client: sandboxClient, Files: fileTransport, Environment: toolchain, Logger: d.logger}, judgeworker.NewEventPublisher(d.natsJS))
	judgeWorker, err := judgeworker.NewJudgeWorker(context.Background(), d.natsJS, judgeUC, d.cfg.Judge.Workers)
	if err != nil {
		return nil, fmt.Errorf("create judge worker: %w", err)
	}
	judgeRecovery := judging.NewJudgeRecovery(
		judgeJobs,
		judgeTransactions,
		judging.DefaultMaxJobAttempts,
		judging.DefaultRecoveryBatchSize,
		judging.DefaultRecoveryInterval,
	)
	inboxPruner := inbox.NewPruner(pgstore.NewInbox(d.pool), d.cfg.Inbox.Retention, inbox.DefaultPruneInterval)
	bookletResponder := booklet.NewResponder(d.natsJS.Conn(), sandbox.NewSandbox(sandboxClient, judgeImage.Config))

	noStop := func(context.Context) error { return nil }
	return []appService{
		newService("problem build worker", buildWorker.Run, noStop),
		newService("problem object GC", problemGC.Run, noStop),
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
			noStop,
		),
		newService(
			"inbox receipts pruner",
			func(ctx context.Context) error {
				inboxPruner.Start(ctx)
				return nil
			},
			noStop,
		),
		newService("booklet responder", bookletResponder.Run, noStop),
	}, nil
}

// workerHealthHandler answers the probes of a worker-only instance, which serves
// no API. It is ready once its workers have started.
func workerHealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","message":"Worker is running"}`))
	})
	return mux
}

// apiServices serve HTTP and WebSocket and publish work to the workers. The
// returned function closes what they opened.
func apiServices(d sharedDeps, judgeImage sandbox.Image) (services []appService, closeAPI func(), err error) {
	cfg, logger, pool, store, natsJS := d.cfg, d.logger, d.pool, d.store, d.natsJS

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

	for bucket, name := range map[string]string{
		defaultS3AvatarBucket: "avatar",
		defaultS3BlogBucket:   "blog",
	} {
		if err := store.EnsureBucket(context.Background(), bucket); err != nil {
			return nil, nil, fmt.Errorf("ensure %s bucket %q: %w", name, bucket, err)
		}
	}

	redisClient, err := pkg.NewRedisClient(cfg.Secrets.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("create rate limiter redis client: %w", err)
	}
	closeAPI = func() { _ = redisClient.Close() }
	defer func() {
		if err != nil {
			closeAPI()
		}
	}()
	limiter := ratelimit.New(redisClient)
	authRepo := pgstore.NewAuthRepo(pool)
	emailService := email.NewEmailService(cfg.Email.Provider == config.EmailProviderResend, cfg.Secrets.ResendAPIKey, cfg.Email.From, cfg.Email.AppBaseURL)

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
	announcementsUC := contests.NewAnnouncementsUseCase(pgstore.NewContestEventsTransactions(pool), announcementsRepo, permissionsUC)
	clarificationsUC := contests.NewClarificationsUseCase(pgstore.NewContestEventsTransactions(pool), clarificationsRepo, permissionsUC)
	logger.Info("successfully initialized use cases")

	problemPolicy := cfg.Problems
	toolchain := toolchainOf(judgeImage)
	sandboxCfg := judgeImage.Config
	problemDrafts := d.problemDrafts
	objectStore := store
	buildOptions := problemsmodel.BuildOptions{Toolchain: toolchain, Limits: problemPolicy.Limits, MaxAttempts: problemPolicy.Attempts, Duration: problemPolicy.Duration, Lease: problemPolicy.Lease}
	draftUC := &problems.DraftUseCase{DraftQuotas: problemPolicy.Draft, Organizations: orgsRepo, Templates: packagesRepo, Repo: problemDrafts, Problems: problemsRepo, Permissions: permissionsUC, Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, UploadTTL: problemPolicy.UploadTTL, ObjectBytes: int64(problemPolicy.Limits.ObjectBytes), BuildOptions: buildOptions}
	packageContent := &problems.PackageContent{Repo: problemDrafts, Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, MaxArchiveBytes: int64(problemPolicy.Limits.ObjectBytes), MaxEntries: problemPolicy.Limits.FileCount}
	draftUC.PackageContent = packageContent
	problemsUC := problems.NewProblemsUseCase(problemsRepo, pgstore.NewProblemTransactions(pool), orgsRepo, permissionsUC, problemDrafts)
	packagesUC := &problems.PackageUseCase{Repo: packagesRepo, Content: packageContent, Permissions: permissionsUC}
	contestDetails := contests.NewDetailsReader(contestsRepo, packageContent, permissionsUC, booklet.NewClient(natsJS.Conn()), usersRepo)

	outboxDispatcher := outbox.NewEventDispatcher()
	outboxDispatcher.Register(eventdata.OutboxEventSubmissionCreated, pubsub.NewSubmissionCreatedPublisher(natsJS))
	outboxDispatcher.Register(eventdata.OutboxEventSubmissionCompleted, pubsub.NewSubmissionCreatedPublisher(natsJS))
	contestEventsPublisher := pubsub.NewContestEventsPublisher(natsJS)
	outboxDispatcher.Register(eventdata.OutboxEventContestAnnouncementCreated, contestEventsPublisher)
	outboxDispatcher.Register(eventdata.OutboxEventContestAnnouncementDeleted, contestEventsPublisher)
	outboxDispatcher.Register(eventdata.OutboxEventContestClarificationCreated, contestEventsPublisher)
	outboxDispatcher.Register(eventdata.OutboxEventContestClarificationAnswered, contestEventsPublisher)
	outboxWorker := outbox.NewOutboxWorker(outboxDispatcher, outboxRepo)
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
		return nil, nil, fmt.Errorf("create submissions subscriber: %w", err)
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
		return nil, nil, fmt.Errorf("create contests subscriber: %w", err)
	}

	observer := wsobserver.NewObserver(observerHub, contestsObserverHub, newObserverMiddleware(usersRepo, authUC), authUC, permissionsUC, contestsRepo, pgstore.NewPolicyFactsRepo(pool))

	publicMux := http.NewServeMux()
	publicMux.Handle("/ws/", observer.Handler())

	coreServer := handlers.NewCoreServer(handlers.Services{
		Auth:               authUC,
		Users:              usersUC,
		UserReads:          usersRepo,
		Avatars:            avatarsUC,
		Organizations:      orgsUC,
		Teams:              teamsUC,
		Contests:           contestsUC,
		ContestPermissions: permissionsUC,
		ContestDetails:     contestDetails,
		Drafts:             draftsUC,
		Announcements:      announcementsUC,
		Clarifications:     clarificationsUC,
		Problems:           problemsUC,
		ProblemDrafts:      draftUC,
		Packages:           packagesUC,
		Submissions:        submissionsUC,
		Notifications:      notificationsUC,
		NotificationReads:  notificationsRepo,
		Blogs:              blogsUC,
		BlogReads:          blogsRepo,
		NATS:               natsJS,
		Capabilities:       access.NewCapabilities(pgstore.NewPolicyFactsRepo(pool)),
	}, cfg.HTTP.SecureCookies).WithLanguages(sandboxCfg.Langs)

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
		return nil, nil, fmt.Errorf("create core api server: %w", err)
	}

	apiHandler := middleware.AuthMiddleware(authUC)(
		middleware.UsersMiddleware(usersRepo)(
			middleware.RateLimit(limiter, cfg.HTTP.TrustedProxy)(middleware.RequestLimits(apiServer, func(operation string) int64 {
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
		Addr: cfg.HTTP.Address,
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

	services = []appService{
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
		newHTTPService("public server", publicServer),
	}
	return services, closeAPI, nil
}

// loadConfig reads the YAML config for mode; envFile, when set, supplies secrets for local runs.
func loadConfig(configFile, envFile string, mode config.Mode) (config.Config, error) {
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return config.Config{}, fmt.Errorf("load env file %s: %w", envFile, err)
		}
	}
	return config.Load(configFile, mode)
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
