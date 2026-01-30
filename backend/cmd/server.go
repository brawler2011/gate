package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/config"
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	handlers "github.com/gate149/gate/backend/internal/transport/rest/core"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/internal/worker/outbox"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/ilyakaznacheev/cleanenv"
	ory "github.com/ory/client-go"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the public API server",
	Run: func(cmd *cobra.Command, args []string) {
		envFile, _ := cmd.Flags().GetString("env")
		runServer(envFile)
	},
}

func init() {
	serverCmd.Flags().String("env", "", "path to environment file")
	rootCmd.AddCommand(serverCmd)
}

func runServer(envFile string) {
	var cfg config.Config
	var err error
	if envFile != "" {
		err = cleanenv.ReadConfig(envFile, &cfg)
		if err != nil {
			panic(fmt.Sprintf("error reading config from %s: %s", envFile, err.Error()))
		}
	} else {
		err = cleanenv.ReadEnv(&cfg)
		if err != nil {
			panic(fmt.Sprintf("error reading config: %s", err.Error()))
		}
	}

	var logger *slog.Logger
	if cfg.Env == "prod" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	} else if cfg.Env == "dev" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	} else {
		panic(fmt.Sprintf(`error reading config: env expected "prod" or "dev", got "%s"`, cfg.Env))
	}

	logger.Info("connecting to postgres")
	pool, err := pkg.NewPostgresDB(cfg.PostgresDSN)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	logger.Info("successfully connected to postgres")

	// Initialize repositories
	usersRepo := pg.NewUsersRepo(pool)
	outboxRepo := pg.NewOutboxRepo(pool)
	txManager := pg.NewTransactor(pool)
	orgsRepo := pg.NewOrganizationsRepo(pool)
	teamsRepo := pg.NewTeamsRepo(pool)
	problemsRepo := pg.NewProblemsRepo(pool)
	contestsRepo := pg.NewContestsRepo(pool)
	logger.Info("successfully initialized repositories")

	// Initialize S3 client
	s3Client := pkg.NewS3Client(pkg.S3Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Region:    cfg.S3Region,
	})
	logger.Info("successfully initialized S3 client")

	// Ensure S3 buckets exist
	if err := s3Client.EnsureBucket(context.Background(), cfg.S3AvatarBucket); err != nil {
		logger.Warn("failed to ensure avatar bucket exists", slog.Any("error", err))
	}
	if err := s3Client.EnsureBucket(context.Background(), cfg.S3PackageBucket); err != nil {
		logger.Warn("failed to ensure package bucket exists", slog.Any("error", err))
	}

	// Initialize use cases
	usersUC := usecase.NewUsersUseCase(usersRepo, outboxRepo, txManager)
	avatarsUC := usecase.NewAvatarsUseCase(usersRepo, s3Client, cfg.S3AvatarBucket)
	problemImportUC := usecase.NewProblemImportUseCase(cfg.WorkshopReposDir)
	problemPublishUC := usecase.NewProblemPublishUseCase(problemsRepo, s3Client, cfg.S3PackageBucket, cfg.WorkshopReposDir)
	pandocClient := pkg.NewPandocClient(&http.Client{}, cfg.Pandoc)
	problemsUC := usecase.NewProblemsUseCase(problemsRepo, pandocClient)
	contestsUC := usecase.NewContestsUseCase(contestsRepo)

	// Initialize new permissions system with Organizations/Teams support
	permissionsUC := usecase.NewPermissionsUseCase(contestsUC, usersUC, problemsUC)
	logger.Info("successfully initialized permissions system with Organizations/Teams support")

	// Initialize Organizations and Teams use cases
	orgsUC := usecase.NewOrganizationsUseCase(orgsRepo, usersRepo, permissionsUC, txManager)
	teamsUC := usecase.NewTeamsUseCase(teamsRepo, orgsRepo, usersRepo, permissionsUC, txManager)
	logger.Info("successfully initialized Organizations and Teams use cases")

	// Initialize Judge0 client (for future use)
	_, err = pkg.NewJudge0Client(cfg.Judge0URL)
	if err != nil {
		logger.Warn("failed to create judge0 client", slog.Any("error", err))
	} else {
		logger.Info("successfully initialized judge0 client", slog.String("url", cfg.Judge0URL))
	}

	// Initialize NATS publisher (for future use)
	_, err = pkg.NewNatsConn(cfg.NatsUrl)
	if err != nil {
		logger.Warn("failed to create nats publisher", slog.Any("error", err))
	} else {
		logger.Info("successfully initialized nats publisher", slog.String("url", cfg.NatsUrl))
	}

	submissionsRepo := pg.NewSubmissionsRepo(pool)
	submissionsUC := usecase.NewSubmissionsUseCase(submissionsRepo, contestsUC, problemsUC, outboxRepo, txManager)

	// Initialize Workshop components
	vcsService := vcs.NewGoGitService(cfg.WorkshopReposDir)
	sandboxClient, err := sandbox.NewClient(sandbox.ClientConfig{
		Protocol: sandbox.ProtocolGRPC,
		BaseURL:  cfg.GoJudgeGRPCAddr,
	})
	if err != nil {
		logger.Warn("failed to create sandbox client, workshop features will be limited", slog.Any("error", err))
		sandboxClient = nil
	} else {
		logger.Info("successfully initialized sandbox client", slog.String("addr", cfg.GoJudgeGRPCAddr))
	}

	var workshopUC *usecase.WorkshopUseCase
	if sandboxClient != nil {
		sandboxOrch := sandbox.NewOrchestrator(sandboxClient)
		workshopUC = usecase.NewWorkshopUseCase(problemsRepo, vcsService, sandboxOrch, txManager)
		logger.Info("successfully initialized workshop use case")
	}

	// Initialize outbox worker (event dispatcher system)
	// TODO: Re-implement outbox handlers for submission events
	dispatcher := outbox.NewEventDispatcher()
	// dispatcher.Register(models.EventTypeSubmissionTest, testHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := outbox.NewOutboxWorker(dispatcher, outboxRepo)
	go worker.Start(ctx)

	server := http.NewServeMux()

	coreServer := handlers.NewCoreServer(
		contestsUC,
		permissionsUC,
		submissionsUC,
		usersUC,
		problemsUC,
		orgsUC,
		teamsUC,
	)
	avatarsHandler := handlers.NewAvatarsHandler(avatarsUC)
	problemImportHandler := handlers.NewProblemImportHandler(problemImportUC)
	problemPublishHandler := handlers.NewProblemPublishHandler(problemPublishUC)

	// Register workshop routes if available
	if workshopUC != nil {
		// Note: Workshop routes will be registered separately using Fiber
		// since the main server uses net/http
		logger.Info("workshop routes available (will need Fiber integration)")
	}

	oryPublicConfiguration := ory.NewConfiguration()
	oryPublicConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosURl,
	}}
	oryPublicClient := ory.NewAPIClient(oryPublicConfiguration)

	// Note: Kratos webhook is handled by separate kratos server (see cmd/kratos.go)

	// Register avatar routes
	server.HandleFunc("POST /api/v1/users/{id}/avatar", avatarsHandler.UploadAvatar)
	server.HandleFunc("DELETE /api/v1/users/{id}/avatar", avatarsHandler.DeleteAvatar)

	// Register problem import and publish routes
	server.HandleFunc("POST /api/v1/problems/import", problemImportHandler.ImportProblem)
	server.HandleFunc("POST /api/v1/problems/{id}/publish", problemPublishHandler.PublishProblem)
	server.HandleFunc("GET /api/v1/problems/{id}/package/{version}", problemPublishHandler.GetPublishedPackage)

	// TODO: Implement Organizations and Teams handlers
	// orgsHandler := handlers.NewOrganizationsHandler(orgsUC)
	// teamsHandler := handlers.NewTeamsHandler(teamsUC)

	logger.Info("API routes registered (Organizations and Teams routes pending implementation)")

	// Create strict handler
	strictHandler := corev1.NewStrictHandlerWithOptions(coreServer, nil, corev1.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
		ResponseErrorHandlerFunc: middleware.ResponseErrorHandler(logger),
	})

	// Register handlers with middlewares
	corev1.HandlerWithOptions(strictHandler, corev1.StdHTTPServerOptions{
		BaseRouter: server,
		Middlewares: []corev1.MiddlewareFunc{
			middleware.RequestLoggerMiddleware(logger),
			middleware.AuthMiddleware(oryPublicClient.FrontendAPI),
			middleware.UsersMiddleware(usersUC),
		},
	})

	httpServer := &http.Server{
		Handler: server,
		Addr:    cfg.Address,
	}

	// Start public server
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error("error starting server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	logger.Info("public server started", slog.String("address", cfg.Address))
	logger.Info("outbox worker started")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop

	logger.Info("shutting down server and worker...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down public server", slog.Any("error", err))
	}

	logger.Info("shutdown complete")
}
