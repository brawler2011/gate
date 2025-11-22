package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/config"
	"github.com/gate149/core/internal/contests"
	"github.com/gate149/core/internal/health"
	"github.com/gate149/core/internal/kratos"
	"github.com/gate149/core/internal/middleware"
	"github.com/gate149/core/internal/outbox"
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/internal/problems"
	"github.com/gate149/core/internal/submissions"
	"github.com/gate149/core/internal/users"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/ilyakaznacheev/cleanenv"
	ory "github.com/ory/client-go"
)

type MergedHandlers struct {
	*users.UsersHandlers
	*contests.ContestsHandlers
	*problems.ProblemsHandlers
	*submissions.SolutionsHandlers
	*health.HealthHandlers
}

func main() {
	var envFile string
	flag.StringVar(&envFile, "env", "", "path to environment file")
	flag.Parse()

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
	db, err := pkg.NewPostgresDB(cfg.PostgresDSN)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	logger.Info("successfully connected to postgres")

	usersRepo := users.NewRepository(db)
	usersUC := users.NewUseCase(usersRepo)

	pandocClient := pkg.NewPandocClient(&http.Client{}, cfg.Pandoc)

	problemsRepo := problems.NewRepository(db)

	problemsUC, err := problems.NewUseCase(problemsRepo, pandocClient)
	if err != nil {
		logger.Error("failed to create problems use case", slog.Any("error", err))
		return
	}

	contestsRepo := contests.NewRepository(db)
	contestsUC := contests.NewContestUseCase(contestsRepo)

	permissionsUC := permissions.NewUseCase(contestsRepo, usersRepo, problemsUC)
	logger.Info("successfully initialized permissions system")

	// Initialize Judge0 client
	judge0Client, err := pkg.NewJudge0Client(cfg.Judge0URL)
	if err != nil {
		logger.Error("failed to create judge0 client", slog.Any("error", err))
		return
	}
	logger.Info("successfully initialized judge0 client", slog.String("url", cfg.Judge0URL))

	// Initialize NATS publisher
	natsPublisher, err := pkg.NewNatsPublisher(cfg.NatsUrl)
	if err != nil {
		logger.Error("failed to create nats publisher", slog.Any("error", err))
		return
	}
	logger.Info("successfully initialized nats publisher", slog.String("url", cfg.NatsUrl))

	// Initialize outbox repository
	outboxRepo := outbox.NewRepository(db)

	solutionsRepo := submissions.NewRepository(db)
	solutionsUC := submissions.NewUseCase(solutionsRepo, contestsRepo, problemsRepo, outboxRepo)

	// Initialize and start outbox worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := outbox.NewWorker(logger, outboxRepo, solutionsRepo, problemsRepo, judge0Client, natsPublisher)
	go worker.Start(ctx)

	server := fiber.New()

	server.Use(middleware.RequestLoggerMiddleware(logger))

	merged := &MergedHandlers{
		UsersHandlers:     users.NewHandlers(usersUC, solutionsUC, permissionsUC),
		ContestsHandlers:  contests.NewHandlers(contestsUC, permissionsUC, solutionsUC),
		ProblemsHandlers:  problems.NewHandlers(problemsUC, permissionsUC),
		SolutionsHandlers: submissions.NewHandlers(solutionsUC, permissionsUC, usersUC),
		HealthHandlers:    health.NewHandlers(),
	}

	// Public API client for session validation (port 4433)
	oryPublicConfiguration := ory.NewConfiguration()
	oryPublicConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosURl,
	}}
	oryPublicClient := ory.NewAPIClient(oryPublicConfiguration)

	// Admin API client for identity management (port 4434)
	oryAdminConfiguration := ory.NewConfiguration()
	oryAdminConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosAdminURL,
	}}
	oryAdminClient := ory.NewAPIClient(oryAdminConfiguration)

	testerv1.RegisterHandlersWithOptions(server, merged, testerv1.FiberServerOptions{
		Middlewares: []testerv1.MiddlewareFunc{
			middleware.ErrorHandlerMiddleware(logger),
			middleware.AuthMiddleware(oryPublicClient.FrontendAPI),
			middleware.UsersMiddleware(usersUC),
		},
	})

	// Start private server for Kratos webhooks
	kratosHandler := kratos.NewKratosHandler(usersUC, oryAdminClient.IdentityAPI)
	privateServer := fiber.New(fiber.Config{
		BodyLimit: 1024 * 1024, // 1 MB for webhook requests
	})

	// Setup private server routes
	privateServer.Post("/webhook/kratos", kratosHandler.HandleKratosWebhook)

	go func() {
		err := privateServer.Listen(cfg.PrivateAddress)
		if err != nil {
			logger.Error("error starting private server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Start public server
	go func() {
		err := server.Listen(cfg.Address)
		if err != nil {
			logger.Error("error starting server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	logger.Info("public server started", slog.String("address", cfg.Address))
	logger.Info("private server started", slog.String("address", cfg.PrivateAddress))
	logger.Info("outbox worker started")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop

	logger.Info("shutting down servers and worker...")
	cancel() // Stop the outbox worker

	// Give some time for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("error shutting down public server", slog.Any("error", err))
	}

	if err := privateServer.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("error shutting down private server", slog.Any("error", err))
	}

	logger.Info("shutdown complete")
}
