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
	"github.com/gate149/core/config"
	"github.com/gate149/core/internal/repository/pg"
	"github.com/gate149/core/internal/transport/middleware"
	handlers "github.com/gate149/core/internal/transport/rest/core"
	"github.com/gate149/core/internal/transport/rest/kratos"
	"github.com/gate149/core/internal/usecase"
	"github.com/gate149/core/internal/worker/outbox"

	"github.com/gate149/core/pkg"
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

	usersRepo := pg.NewUsersRepo(pool)
	outboxRepo := pg.NewOutboxRepo(pool)
	imagesRepo := pg.NewImagesRepo(pool)
	txManager := pkg.NewTxManager(pool)

	usersUC := usecase.NewUsersUseCase(usersRepo, outboxRepo, imagesRepo, txManager)

	pandocClient := pkg.NewPandocClient(&http.Client{}, cfg.Pandoc)

	problemsRepo := pg.NewProblemsRepo(pool)

	problemsUC := usecase.NewProblemsUseCase(problemsRepo, pandocClient)

	contestsRepo := pg.NewContestsRepo(pool)
	contestsUC := usecase.NewContestsUseCase(contestsRepo)

	permissionsUC := usecase.NewPermissionsUseCase(contestsUC, usersUC, problemsUC)
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

	submissionsRepo := pg.NewSubmissionsRepo(pool)
	submissionsUC := usecase.NewSubmissionsUseCase(submissionsRepo, contestsUC, problemsUC, outboxRepo, natsPublisher)

	// Initialize and start outbox worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := outbox.NewWorker(logger, outboxRepo, submissionsRepo, problemsRepo, judge0Client, natsPublisher)
	go worker.Start(ctx)

	server := http.NewServeMux()

	coreServer := handlers.NewCoreServer(
		contestsUC,
		permissionsUC,
		submissionsUC,
		usersUC,
		problemsUC,
	)

	oryPublicConfiguration := ory.NewConfiguration()
	oryPublicConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosURl,
	}}
	oryPublicClient := ory.NewAPIClient(oryPublicConfiguration)

	kratosHandler := kratos.NewKratosHandler(usersUC, oryPublicClient.IdentityAPI)
	server.HandleFunc("POST /kratos/webhook", kratosHandler.HandleKratosWebhook)

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
