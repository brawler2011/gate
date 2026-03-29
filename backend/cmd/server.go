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
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	handlers "github.com/gate149/gate/backend/internal/transport/rest/core"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/internal/worker/outbox"
	"github.com/gate149/gate/backend/internal/worker/pubsub"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
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
		// Load .env file first (works with any extension)
		err = godotenv.Load(envFile)
		if err != nil {
			panic(fmt.Sprintf("error loading env file %s: %s", envFile, err.Error()))
		}
		// Then read config from environment variables
		err = cleanenv.ReadEnv(&cfg)
		if err != nil {
			panic(fmt.Sprintf("error reading config from environment: %s", err.Error()))
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
	} else if cfg.Env == "dev" || cfg.Env == "local" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	} else {
		panic(fmt.Sprintf(`error reading config: env expected "prod", "dev", or "local", got "%s"`, cfg.Env))
	}

	logger.Info("connecting to postgres")
	pool, err := pkg.NewPostgresDB(cfg.GetPostgresDSN())
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
	blogsRepo := pg.NewBlogsRepo(pool)
	logger.Info("successfully initialized repositories")

	// Initialize S3 client
	s3Client := pkg.NewS3Client(pkg.S3Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Region:    defaultS3Region,
	})
	logger.Info("successfully initialized S3 client")

	// Ensure S3 buckets exist
	if err := s3Client.EnsureBucket(context.Background(), defaultS3AvatarBucket); err != nil {
		logger.Warn("failed to ensure avatar bucket exists", slog.Any("error", err))
	}
	if err := s3Client.EnsureBucket(context.Background(), defaultS3PackageBucket); err != nil {
		logger.Warn("failed to ensure package bucket exists", slog.Any("error", err))
	}
	if err := s3Client.EnsureBucket(context.Background(), defaultS3WorkshopBucket); err != nil {
		logger.Warn("failed to ensure workshop bucket exists", slog.Any("error", err))
	}
	if err := s3Client.EnsureBucket(context.Background(), defaultS3BlogBucket); err != nil {
		logger.Warn("failed to ensure blog bucket exists", slog.Any("error", err))
	}
	vcsService := vcs.NewS3Service(s3Client, defaultS3WorkshopBucket)

	// Initialize use cases
	usersUC := usecase.NewUsersUseCase(usersRepo, outboxRepo, txManager)
	avatarsUC := usecase.NewAvatarsUseCase(usersRepo, s3Client, defaultS3AvatarBucket)
	problemImportUC := usecase.NewProblemImportUseCase(problemsRepo, vcsService)
	problemsUC := usecase.NewProblemsUseCase(problemsRepo)
	contestsUC := usecase.NewContestsUseCase(contestsRepo)
	blogsUC := usecase.NewBlogsUseCase(blogsRepo, s3Client, defaultS3BlogBucket)

	// Initialize new permissions system with Organizations/Teams support
	permissionsUC := usecase.NewPermissionsUseCase(contestsUC, usersUC, problemsUC)
	logger.Info("successfully initialized permissions system with Organizations/Teams support")

	// Initialize Organizations and Teams use cases
	orgsUC := usecase.NewOrganizationsUseCase(orgsRepo, usersRepo, permissionsUC, txManager)
	teamsUC := usecase.NewTeamsUseCase(teamsRepo, orgsRepo, usersRepo, permissionsUC, txManager)
	logger.Info("successfully initialized Organizations and Teams use cases")

	// Initialize NATS JetStream connection
	natsJS, err := pkg.NewNatsJetStream(cfg.GetNatsURL())
	if err != nil {
		logger.Warn("failed to create nats jetstream connection", slog.Any("error", err))
		natsJS = nil
	} else {
		logger.Info("successfully initialized nats jetstream", slog.String("url", cfg.GetNatsURL()))
		if err := pkg.EnsureSubmissionsStream(context.Background(), natsJS); err != nil {
			logger.Warn("failed to ensure SUBMISSIONS stream", slog.Any("error", err))
			natsJS = nil
		} else {
			logger.Info("SUBMISSIONS stream ready")
		}
	}

	submissionsRepo := pg.NewSubmissionsRepo(pool)
	submissionsUC := usecase.NewSubmissionsUseCase(submissionsRepo, contestsUC, problemsUC, outboxRepo, txManager)

	// Initialize problem publish use case (depends on vcsService)
	packagesRepo := pg.NewPackagesRepo(pool)
	problemPublishUC := usecase.NewProblemPublishUseCase(problemsRepo, packagesRepo, vcsService, s3Client, defaultS3PackageBucket)
	sandboxClient, err := sandbox.NewClient(sandbox.ClientConfig{
		Addr: cfg.GoJudgeGRPCAddr,
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
	dispatcher := outbox.NewEventDispatcher()
	if natsJS != nil {
		dispatcher.Register(models.OutboxEventSubmissionCreated, pubsub.NewSubmissionCreatedPublisher(natsJS))
		logger.Info("registered submission.created outbox handler")
	} else {
		logger.Warn("nats jetstream unavailable, submission events will not be forwarded to judge worker")
	}

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
		workshopUC,
		blogsUC,
		avatarsUC,
		problemImportUC,
		problemPublishUC,
	)

	if workshopUC != nil {
		logger.Info("workshop routes registered with net/http")
	} else {
		logger.Warn("workshop use case not initialized, workshop endpoints will not be available")
	}

	oryPublicConfiguration := ory.NewConfiguration()
	oryPublicConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosURl,
	}}
	oryPublicClient := ory.NewAPIClient(oryPublicConfiguration)

	// Note: Kratos webhook is handled by separate kratos server (see cmd/kratos.go)

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
			middleware.UsersMiddleware(usersUC),
			middleware.AuthMiddleware(oryPublicClient.FrontendAPI),
			middleware.RequestLoggerMiddleware(logger),
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
