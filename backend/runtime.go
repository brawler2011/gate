package main

import (
	"context"
	"encoding/json"
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
	identity "github.com/brawler2011/gate/backend/internal/identity"
	"github.com/brawler2011/gate/backend/internal/invitations"
	judging "github.com/brawler2011/gate/backend/internal/judging"
	notifications "github.com/brawler2011/gate/backend/internal/notifications"
	organizations "github.com/brawler2011/gate/backend/internal/organizations"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	submissions "github.com/brawler2011/gate/backend/internal/submissions"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	handlers "github.com/brawler2011/gate/backend/internal/transport/rest/core"
	wsobserver "github.com/brawler2011/gate/backend/internal/transport/ws/observer"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/diskcache"
	"github.com/brawler2011/gate/backend/pkg/email"
	"github.com/brawler2011/gate/backend/pkg/hostcheck"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

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

	typesetClient, err := sandbox.NewClient(cfg.Typeset.HTTPURL, &http.Client{})
	if err != nil {
		return fmt.Errorf("initialize typeset client: %w", err)
	}
	shared := sharedDeps{cfg: cfg, logger: logger, pool: pool, store: store, problemDrafts: pgstore.NewProblemDraftsRepo(pool, cfg.Problems.Draft), typeset: sandbox.NewTypesetter(typesetClient, cfg.Problems.Limits.Typeset)}

	var services []appService
	var languages *sandbox.Config
	if mode.Worker() {
		sandboxClient, err := sandbox.NewClient(cfg.GoJudge.HTTPURL, &http.Client{})
		if err != nil {
			return fmt.Errorf("initialize sandbox client: %w", err)
		}

		imageCtx, cancelImage := context.WithTimeout(context.Background(), time.Minute)
		judgeImage, err := sandbox.ReadImage(imageCtx, sandboxClient)
		cancelImage()
		if err != nil {
			return err
		}
		logger.Info("read judge image manifest", slog.String("judge_image_digest", judgeImage.Identity.Digest))
		languages = judgeImage.Config
		workers, ready, err := workerServices(shared, sandboxClient, judgeImage)
		if err != nil {
			return err
		}
		services = append(services, workers...)
		if !mode.API() {
			services = append(services, newHTTPService("worker server", &http.Server{Addr: cfg.HTTP.Address, Handler: workerHandler(ready), ReadHeaderTimeout: 10 * time.Second, ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError)}))
		}
	}
	if mode.API() {
		if !mode.Worker() {
			languages, err = sandbox.LoadConfig(filepath.Join(cfg.Judge.ManifestDir, "langs.toml"))
			if err != nil {
				return err
			}
		}
		api, err := apiServices(shared, languages)
		if err != nil {
			return err
		}
		services = append(services, api...)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return runServices(ctx, logger, services)
}

type sharedDeps struct {
	cfg           config.Config
	logger        *slog.Logger
	pool          *pgxpool.Pool
	store         *storage.S3Storage
	problemDrafts *pgstore.ProblemDraftsRepo
	typeset       *sandbox.Typesetter
}

// The returned ready reports whether the worker takes judging and build jobs.
func workerServices(d sharedDeps, sandboxClient *sandbox.Client, judgeImage sandbox.Image) ([]appService, func() (bool, []string), error) {
	problemPolicy := d.cfg.Problems
	fileTransport, err := sandbox.NewFileTransport(d.cfg.GoJudge.HTTPURL, &http.Client{Timeout: problemPolicy.UploadTTL})
	if err != nil {
		return nil, nil, err
	}

	cache, err := diskcache.Open(filepath.Join(problemPolicy.WorkDir, "cache"), int64(problemPolicy.CacheBytes))
	if err != nil {
		return nil, nil, err
	}
	parallelism, err := d.cfg.Parallelism()
	if err != nil {
		return nil, nil, err
	}
	d.logger.Info("go-judge parallelism", slog.Int("parallelism", parallelism), slog.String("judge_mode", string(d.cfg.Judge.Mode)))
	status := func() (bool, []string) { return true, nil }
	var gate interface {
		Ready() bool
		Admit(ctx context.Context) bool
	} = hostcheck.Open{}
	calib := calibration.Uncalibrated
	var hostMonitor *hostcheck.Monitor
	if d.cfg.Judge.Mode == config.JudgeStrict {
		cores, err := d.cfg.Judge.Cores()
		if err != nil {
			return nil, nil, err
		}
		bench := d.cfg.Judge.Benchmark
		benchmark := func(ctx context.Context) (calibration.Calibration, []string) {
			m, err := calibration.Measure(ctx, sandbox.NewSandbox(sandboxClient, judgeImage.Config), fileTransport, problemPolicy.Limits.Compile, cores, bench.Repeats)
			if err != nil {
				return calibration.Calibration{}, []string{fmt.Sprintf("benchmark: %v", err)}
			}
			k, unfit := bench.Evaluate(m)
			return calibration.Calibration{K: k, Benchmark: calibration.ID(judgeImage.Identity.Digest)}, unfit
		}
		host := hostcheck.Host{Sys: "/sys", Proc: "/proc", Interrupts: func(ctx context.Context) ([]byte, error) { return sandboxClient.ReadFile(ctx, "/proc/interrupts") }}
		hostMonitor = &hostcheck.Monitor{Host: host, Cores: cores, Probes: sandboxClient.Probes, Benchmark: benchmark, Logger: d.logger}
		status, gate, calib = hostMonitor.Status, hostMonitor, hostMonitor.Calibration
	}
	programs := sandbox.NewProgramCache(cache, sandboxClient, fileTransport, judgeImage)
	pipeline := &builder.Pipeline{Repo: d.problemDrafts, Store: d.store, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, Client: sandboxClient, Typeset: d.typeset, Files: fileTransport, Languages: judgeImage.Config, Programs: programs, Parallelism: parallelism, Calibration: calib, Logger: d.logger}
	buildWorker := &builder.Worker{Pipeline: pipeline, Parallelism: problemPolicy.Parallelism, Lease: problemPolicy.Lease, Poll: problemPolicy.Poll, Gate: gate}
	problemGC := &builder.GC{Repo: d.problemDrafts, Store: d.store, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, ObjectRetention: problemPolicy.ObjectRetention, BuildRetention: problemPolicy.BuildRetention, PackageRetention: problemPolicy.PackageRetention, Interval: problemPolicy.GCInterval, Logger: d.logger}

	judgeJobs := pgstore.NewJudgeJobs(d.pool)
	judgeTransactions := pgstore.NewJudgeTransactions(d.pool)
	packageJudge := &judging.PackageJudge{Client: sandboxClient, Files: fileTransport, Languages: judgeImage.Config, Programs: programs, Limits: problemPolicy.Limits, WorkDir: sandbox.SharedDir{Local: problemPolicy.WorkDir, Sandbox: d.cfg.GoJudge.WorkDir}, RerunTLE: d.cfg.Judge.Mode == config.JudgeDirty, Calibration: calib, Logger: d.logger}
	judgeUC := judging.NewJudgeUseCase(judgeJobs, judgeTransactions, pgstore.NewSubmissionsRepo(d.pool),
		&judging.PackageLoader{Store: d.store, Bucket: defaultS3ObjectBucket, Cache: cache, Packages: d.problemDrafts, Limits: problemPolicy.Limits},
		packageJudge, pgstore.NewNotifier(d.pool))
	wake := make(chan struct{}, d.cfg.Judge.Workers)
	judgeWorker := &judging.Worker{Jobs: judgeJobs, Judge: judgeUC, Workers: d.cfg.Judge.Workers, Gate: gate, Wake: wake, Logger: d.logger.With("component", "judge_worker")}
	wakeWorkers := func(n int) {
		for range n {
			select {
			case wake <- struct{}{}:
			default:
			}
		}
	}
	judgeRecovery := judging.NewJudgeRecovery(
		judgeJobs,
		judgeTransactions,
		judging.DefaultMaxJobAttempts,
		judging.DefaultRecoveryBatchSize,
		judging.DefaultRecoveryInterval,
	)

	noStop := func(context.Context) error { return nil }
	services := []appService{
		newService("problem build worker", buildWorker.Run, noStop),
		newService("problem object GC", problemGC.Run, noStop),
		newService("judge worker", judgeWorker.Run, noStop),
		newService("judge job listener", func(ctx context.Context) error {
			return pgstore.Listen(ctx, d.pool, pgstore.JudgeJobsChannel,
				func() { wakeWorkers(d.cfg.Judge.Workers) }, func(*pgconn.Notification) { wakeWorkers(1) })
		}, noStop),
		newService(
			"judge recovery worker",
			func(ctx context.Context) error {
				judgeRecovery.Start(ctx)
				return nil
			},
			noStop,
		),
	}
	if hostMonitor != nil {
		services = append(services, newService("judging host monitor", hostMonitor.Run, noStop))
	}
	return services, status, nil
}

func workerHandler(ready func() (bool, []string)) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","message":"Worker is running"}`))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		ok, reasons := ready()
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ready": ok, "reasons": reasons})
	})
	return mux
}

func apiServices(d sharedDeps, languages *sandbox.Config) ([]appService, error) {
	cfg, logger, pool, store := d.cfg, d.logger, d.pool, d.store

	usersRepo := pgstore.NewUsersRepo(pool)
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
			return nil, fmt.Errorf("ensure %s bucket %q: %w", name, bucket, err)
		}
	}

	limiter := ratelimit.New(pgstore.NewRateLimits(pool))
	authRepo := pgstore.NewAuthRepo(pool)
	emailService := email.NewEmailService(cfg.Email.Provider == config.EmailProviderResend, cfg.Secrets.ResendAPIKey, cfg.Email.From, cfg.Email.AppBaseURL)

	usersUC := identity.NewUsersUseCase(usersRepo, pgstore.NewIdentityTransactions(pool), emailService)
	authUC := identity.NewAuthUseCase(usersRepo, authRepo, pgstore.NewIdentityTransactions(pool), emailService, limiter)
	notificationsUC := notifications.NewNotificationsUseCase(notificationsRepo, usersRepo, emailService)

	avatarsUC := identity.NewAvatarsUseCase(usersRepo, pgstore.NewIdentityTransactions(pool), store, defaultS3AvatarBucket)
	accessLoader := pgstore.NewAccess(pool)
	contestsUC := contests.NewContestsUseCase(contestsRepo, orgsRepo, pgstore.NewContestCommandTransactions(pool), accessLoader)
	contestsUC.SetNotificationsUC(notificationsUC)

	blogsUC := blogs.NewBlogsUseCase(pgstore.NewBlogTransactions(pool), store, defaultS3BlogBucket)
	orgsUC := organizations.NewOrganizationsUseCase(orgsRepo, usersRepo, accessLoader, pgstore.NewOrganizationTransactions(pool), notificationsUC)
	teamsUC := organizations.NewTeamsUseCase(teamsRepo, accessLoader, pgstore.NewTeamTransactions(pool))
	invitationsUC := invitations.NewUseCase(pgstore.NewInvitationsRepo(pool), accessLoader, pgstore.NewInvitationTransactions(pool), notificationsUC)
	submissionsUC := submissions.NewSubmissionsUseCase(submissionsRepo, pgstore.NewSubmissionTransactions(pool), accessLoader)
	draftsUC := contests.NewDraftsUseCase(pgstore.NewDraftTransactions(pool), pgstore.NewDraftsRepo(pool), accessLoader)
	announcementsUC := contests.NewAnnouncementsUseCase(pgstore.NewContestEventsTransactions(pool), announcementsRepo, accessLoader)
	clarificationsUC := contests.NewClarificationsUseCase(pgstore.NewContestEventsTransactions(pool), clarificationsRepo, accessLoader)
	logger.Info("successfully initialized use cases")

	problemPolicy := cfg.Problems
	problemDrafts := d.problemDrafts
	objectStore := store
	buildOptions := problemsmodel.BuildOptions{Languages: languages.Langs, Limits: problemPolicy.Limits, MaxAttempts: problemPolicy.Attempts, Duration: problemPolicy.Duration, Lease: problemPolicy.Lease}
	draftUC := &problems.DraftUseCase{DraftQuotas: problemPolicy.Draft, Organizations: orgsRepo, Templates: packagesRepo, Repo: problemDrafts, Problems: problemsRepo, Access: accessLoader, Store: objectStore, Bucket: defaultS3ObjectBucket, TempDir: problemPolicy.WorkDir, UploadTTL: problemPolicy.UploadTTL, BuildOptions: buildOptions, Languages: languages, Typeset: d.typeset}
	packageContent := &problems.PackageContent{Repo: problemDrafts, Store: objectStore, Bucket: defaultS3ObjectBucket}
	draftUC.PackageContent = packageContent
	problemsUC := problems.NewProblemsUseCase(problemsRepo, pgstore.NewProblemTransactions(pool), orgsRepo, accessLoader, packagesRepo)
	packagesUC := &problems.PackageUseCase{Repo: packagesRepo, Content: packageContent, Access: accessLoader}
	contestDetails := contests.NewDetailsReader(contestsRepo, packageContent, accessLoader, d.typeset)

	observerHub := wsobserver.NewHub(contestsRepo, contestsUC)
	contestsObserverHub := wsobserver.NewContestsHub()
	feed := &wsobserver.Feed{Hub: observerHub, Contests: contestsObserverHub, Records: struct {
		*pgstore.SubmissionsRepo
		*pgstore.AnnouncementsRepo
		*pgstore.ClarificationsRepo
	}{submissionsRepo, announcementsRepo, clarificationsRepo}}
	observer := wsobserver.NewObserver(observerHub, contestsObserverHub, newObserverMiddleware(usersRepo, authUC, logger), authUC, accessLoader, contestsRepo)

	publicMux := http.NewServeMux()
	publicMux.Handle("/ws/", observer.Handler())

	coreServer := handlers.NewCoreServer(handlers.Services{
		Auth:              authUC,
		Users:             usersUC,
		UserReads:         usersRepo,
		Avatars:           avatarsUC,
		Organizations:     orgsUC,
		Teams:             teamsUC,
		Invitations:       invitationsUC,
		Contests:          contestsUC,
		Access:            accessLoader,
		ContestDetails:    contestDetails,
		Drafts:            draftsUC,
		Announcements:     announcementsUC,
		Clarifications:    clarificationsUC,
		Problems:          problemsUC,
		ProblemDrafts:     draftUC,
		Packages:          packagesUC,
		Submissions:       submissionsUC,
		Notifications:     notificationsUC,
		NotificationReads: notificationsRepo,
		Blogs:             blogsUC,
		BlogReads:         blogsRepo,
		Permissions:       access.NewPermissions(accessLoader),
	}, cfg.HTTP.SecureCookies).WithLanguages(languages)

	secHandler := middleware.NewSecurityHandler()
	authzMw := middleware.AuthzMiddleware()
	apiServer, err := corev1.NewServer(
		coreServer,
		secHandler,
		corev1.WithMiddleware(authzMw),
		corev1.WithMaxMultipartMemory(64*1024),
		corev1.WithErrorHandler(middleware.ResponseErrorHandler(logger)),
		corev1.WithNotFound(middleware.NotFound),
	)
	if err != nil {
		return nil, fmt.Errorf("create core api server: %w", err)
	}

	apiHandler := middleware.AuthMiddleware(authUC, middleware.ResponseErrorHandler(logger))(
		middleware.UsersMiddleware(usersRepo, middleware.ResponseErrorHandler(logger))(
			middleware.RateLimit(limiter, cfg.HTTP.TrustedProxy, middleware.ResponseErrorHandler(logger))(middleware.RequestLimits(apiServer, func(operation string) int64 {
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
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	services := []appService{
		newService("event listener", func(ctx context.Context) error {
			return pgstore.Listen(ctx, pool, pgstore.EventsChannel, feed.Resync,
				func(n *pgconn.Notification) { feed.Handle(ctx, n.Payload) })
		}, func(context.Context) error { return nil }),
		newService(
			"session cleanup worker",
			func(ctx context.Context) error {
				ticker := time.NewTicker(10 * time.Minute)
				defer ticker.Stop()

				if err := authUC.CleanupExpiredSessions(ctx); err != nil && ctx.Err() == nil {
					slog.ErrorContext(ctx, "failed to cleanup expired sessions", "error", err)
				}

				for {
					select {
					case <-ctx.Done():
						return nil
					case <-ticker.C:
						if err := authUC.CleanupExpiredSessions(ctx); err != nil && ctx.Err() == nil {
							slog.ErrorContext(ctx, "failed to cleanup expired sessions", "error", err)
						}
					}
				}
			},
			func(context.Context) error {
				return nil
			},
		),
		newService("system testing", func(ctx context.Context) error {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				if n, err := judging.SystemTest(ctx, pgstore.NewJudgeTransactions(pool)); err != nil && ctx.Err() == nil {
					slog.ErrorContext(ctx, "system testing failed", "error", err)
				} else if n > 0 {
					slog.InfoContext(ctx, "system testing started", "submissions", n)
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		}, func(context.Context) error { return nil }),
		newHTTPService("public server", publicServer),
	}
	return services, nil
}

// writeJudgeArgs lets the go-judge container start with the flags the worker's config implies.
func writeJudgeArgs(configFile, envFile, path string) error {
	cfg, err := loadConfig(configFile, envFile, config.ModeWorker)
	if err != nil {
		return err
	}
	args, err := cfg.GoJudgeArgs()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(strings.Join(args, " ")), 0o600); err != nil {
		return fmt.Errorf("write go-judge args: %w", err)
	}
	return nil
}

func loadConfig(configFile, envFile string, mode config.Mode) (config.Config, error) {
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return config.Config{}, fmt.Errorf("load env file %s: %w", envFile, err)
		}
	}
	return config.Load(configFile, mode)
}

func newObserverMiddleware(users middleware.UsersUC, authUC middleware.AuthUC, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return middleware.AuthMiddleware(authUC, middleware.ResponseErrorHandler(logger))(
			middleware.UsersMiddleware(users, middleware.ResponseErrorHandler(logger))(next),
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
