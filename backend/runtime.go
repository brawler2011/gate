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
	"syscall"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/config"
	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	handlers "github.com/gate149/gate/backend/internal/transport/rest/core"
	wsobserver "github.com/gate149/gate/backend/internal/transport/ws/observer"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/internal/worker/judge"
	"github.com/gate149/gate/backend/internal/worker/outbox"
	"github.com/gate149/gate/backend/internal/worker/pubsub"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/gate149/gate/backend/pkg/storage"
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
				return err
			}
			return nil
		},
		func(ctx context.Context) error {
			if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
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

	logger, err := newLogger(cfg.Env)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	logger.Info("connecting to postgres")
	pool, err := pkg.NewPostgresDB(cfg.GetPostgresDSN())
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("successfully connected to postgres")

	usersRepo := pg.NewUsersRepo(pool)
	outboxRepo := pg.NewOutboxRepo(pool)
	txManager := pg.NewTransactor(pool)
	orgsRepo := pg.NewOrganizationsRepo(pool)
	teamsRepo := pg.NewTeamsRepo(pool)
	problemsRepo := pg.NewProblemsRepo(pool)
	contestsRepo := pg.NewContestsRepo(pool)
	blogsRepo := pg.NewBlogsRepo(pool)
	submissionsRepo := pg.NewSubmissionsRepo(pool)
	packagesRepo := pg.NewPackagesRepo(pool)
	logger.Info("successfully initialized repositories")

	var store storage.Storage
	if cfg.StorageType == "s3" {
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

	workspaceStorage := usecase.NewWorkspaceStorage(store, defaultS3WorkshopBucket)

	authRepo := pg.NewAuthRepo(pool)

	usersUC := usecase.NewUsersUseCase(usersRepo, outboxRepo, txManager)
	authUC := usecase.NewAuthUseCase(usersRepo, authRepo, txManager)
	avatarsUC := usecase.NewAvatarsUseCase(usersRepo, store, defaultS3AvatarBucket)
	problemImportUC := usecase.NewProblemImportUseCase(problemsRepo, workspaceStorage)
	problemsUC := usecase.NewProblemsUseCase(problemsRepo)
	contestsUC := usecase.NewContestsUseCase(contestsRepo)
	blogsUC := usecase.NewBlogsUseCase(blogsRepo, store, defaultS3BlogBucket)
	permissionsUC := usecase.NewPermissionsUseCase(contestsRepo, usersUC, problemsRepo, teamsRepo, orgsRepo)
	orgsUC := usecase.NewOrganizationsUseCase(orgsRepo, usersRepo, permissionsUC, txManager)
	teamsUC := usecase.NewTeamsUseCase(teamsRepo, orgsRepo, usersRepo, permissionsUC, txManager)
	submissionsUC := usecase.NewSubmissionsUseCase(submissionsRepo, contestsUC, problemsUC, outboxRepo, txManager)
	problemPublishUC := usecase.NewProblemPublishUseCase(problemsRepo, packagesRepo, workspaceStorage, store, defaultS3PackageBucket)
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
	workshopUC := usecase.NewWorkshopUseCase(problemsRepo, workspaceStorage, sandboxInstance, txManager)

	natsJS, err := pkg.NewNatsJetStream(cfg.GetNatsURL())
	if err != nil {
		return fmt.Errorf("create nats jetstream connection: %w", err)
	}
	logger.Info("successfully initialized nats jetstream", slog.String("url", cfg.GetNatsURL()))

	if err := pkg.EnsureSubmissionsStream(context.Background(), natsJS); err != nil {
		return fmt.Errorf("ensure submissions stream: %w", err)
	}
	logger.Info("SUBMISSIONS stream ready")

	outboxDispatcher := outbox.NewEventDispatcher()
	outboxDispatcher.Register(models.OutboxEventSubmissionCreated, pubsub.NewSubmissionCreatedPublisher(natsJS))
	outboxWorker := outbox.NewOutboxWorker(outboxDispatcher, outboxRepo)

	judgeUC := usecase.NewJudgeUseCase(
		submissionsRepo,
		packagesRepo,
		store,
		defaultS3PackageBucket,
		judgeTempDir,
		sandboxInstance,
		judge.NewEventPublisher(natsJS),
	)

	judgeWorker, err := judge.NewJudgeWorker(context.Background(), natsJS, judgeUC, cfg.JudgeWorkerCount)
	if err != nil {
		return fmt.Errorf("create judge worker: %w", err)
	}

	submissionDispatcher := outbox.NewEventDispatcher()
	observerHub := wsobserver.NewHub(submissionsRingBufferSize)

	for _, event := range []models.SubmissionEventType{
		models.SubmissionEventCreated,
		models.SubmissionEventQueued,
		models.SubmissionEventCompilingStarted,
		models.SubmissionEventTestingStarted,
		models.SubmissionEventTestStarted,
		models.SubmissionEventCompleted,
	} {
		submissionDispatcher.Register(event, observerHub)
	}

	submissionsSub, err := pubsub.NewSubmissionsSub(context.Background(), natsJS, submissionDispatcher)
	if err != nil {
		return fmt.Errorf("create submissions subscriber: %w", err)
	}

	observer := wsobserver.NewObserver(&cfg, observerHub, newObserverMiddleware(usersUC, authUC))

	publicMux := http.NewServeMux()
	publicMux.Handle("/ws/", observer.Handler())
	if cfg.StorageType == "local" {
		publicMux.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(cfg.LocalStoragePath))))
		logger.Info("mounted local files storage server", slog.String("prefix", "/files/"), slog.String("dir", cfg.LocalStoragePath))
	}

	coreServer := handlers.NewCoreServer(
		authUC,
		contestsUC,
		permissionsUC,
		submissionsUC,
		usersUC,
		problemsUC,
		orgsUC,
		teamsUC,
		workshopUC,
		blogsUC,
		avatarsUC,
		problemImportUC,
		problemPublishUC,
		natsJS,
	)

	strictHandler := corev1.NewStrictHandlerWithOptions(coreServer, []corev1.StrictMiddlewareFunc{
		middleware.AuthzStrictMiddleware(permissionsUC, submissionsUC),
	}, corev1.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
		ResponseErrorHandlerFunc: middleware.ResponseErrorHandler(logger),
	})

	corev1.HandlerWithOptions(strictHandler, corev1.StdHTTPServerOptions{
		BaseRouter: publicMux,
		Middlewares: []corev1.MiddlewareFunc{
			middleware.UsersMiddleware(usersUC),
			middleware.AuthMiddleware(authUC),
			middleware.RequestLoggerMiddleware(logger),
		},
	})

	publicServer := &http.Server{
		Addr:    cfg.Address,
		Handler: publicMux,
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

func newLogger(env string) (*slog.Logger, error) {
	switch env {
	case "prod":
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})), nil
	case "dev", "local":
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})), nil
	default:
		return nil, fmt.Errorf("invalid ENV %q: expected prod, dev, or local", env)
	}
}

func newObserverMiddleware(usersUC interfaces.UsersUC, authUC interfaces.AuthUC) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return middleware.AuthMiddleware(authUC)(
			middleware.UsersMiddleware(usersUC)(next),
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
