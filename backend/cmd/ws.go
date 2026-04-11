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

	"github.com/gate149/gate/backend/config"
	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	ws "github.com/gate149/gate/backend/internal/transport/ws/observer"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/internal/worker/outbox"
	"github.com/gate149/gate/backend/internal/worker/pubsub"
	"github.com/gate149/gate/backend/pkg"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	"github.com/ory/client-go"
	ory "github.com/ory/client-go"
	"github.com/spf13/cobra"
)

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "Start the WebSocket server for submission list updates",
	Long: `Start a dedicated WebSocket server that handles real-time 
submission list updates and testing progress.

The server connects to NATS and broadcasts submission events to connected clients
based on their filter parameters.

Endpoint: GET /ws/submissions?sortOrder=desc&contestId=uuid&userId=uuid&problemId=uuid&state=int&language=int

Parameters:
  sortOrder (required): Must be "desc" for real-time updates (page=1, sortOrder=desc)
  contestId (optional): Filter by contest Id
  userId (optional): Filter by user Id  
  problemId (optional): Filter by problem Id
  state (optional): Filter by submission state
  language (optional): Filter by programming language

Events are only sent for submissions that match the filter criteria.
For private contests, only clients with a matching contestId filter will receive events.

Environment variables:
  WS_ADDRESS - WebSocket server address (default: :8081)
  NATS_URL   - NATS server URL (default: nats://localhost:4222)
  ENV        - Environment: "dev" or "prod" (default: prod)`,
	Run: func(cmd *cobra.Command, args []string) {
		envFile, _ := cmd.Flags().GetString("env")
		runWsServer(envFile)
	},
}

func init() {
	wsCmd.Flags().String("env", "", "path to environment file")
	rootCmd.AddCommand(wsCmd)
}

func runWsServer(envFile string) {
	var cfg config.WsConfig
	var err error

	if envFile != "" {
		err = godotenv.Load(envFile)
		if err != nil {
			panic(fmt.Sprintf("error loading env file %s: %s", envFile, err.Error()))
		}
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

	// Initialize logger
	var log *slog.Logger
	if cfg.Env == "prod" {
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	} else {
		log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}
	slog.SetDefault(log)

	slog.Info("starting websocket server",
		slog.String("address", cfg.WsAddress),
		slog.String("nats_url", cfg.GetNatsURL()),
		slog.String("env", cfg.Env))

	// Initialize WebSocket observer with ring buffer size
	const ringBufferSize = 10000

	pool, err := pkg.NewPostgresDB(cfg.GetPostgresDSN())
	usersRepo := pg.NewUsersRepo(pool)
	outboxRepo := pg.NewOutboxRepo(pool)
	txManager := pg.NewTransactor(pool)

	usersUC := usecase.NewUsersUseCase(usersRepo, outboxRepo, txManager)

	oryPublicConfiguration := ory.NewConfiguration()
	oryPublicConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosURl,
	}}
	oryPublicClient := ory.NewAPIClient(oryPublicConfiguration)

	observerMiddleware := newMiddleware(usersUC, oryPublicClient)
	observerHub := ws.NewHub(ringBufferSize)
	observer := ws.NewObserver(&cfg, observerHub, observerMiddleware)
	slog.Info("websocket observer initialized")

	js, err := pkg.NewNatsJetStream(cfg.GetNatsURL())
	if err != nil {
		panic(fmt.Sprintf("failed to connect to nats jetstream: %s", err.Error()))
	}

	if err := pkg.EnsureSubmissionsStream(context.Background(), js); err != nil {
		panic(fmt.Sprintf("failed to ensure SUBMISSIONS stream: %s", err.Error()))
	}
	slog.Info("nats jetstream initialized", slog.String("url", cfg.GetNatsURL()))

	dispatcher := outbox.NewEventDispatcher()
	dispatcher.Register(models.SubmissionEventCreated, observerHub)
	dispatcher.Register(models.SubmissionEventQueued, observerHub)
	dispatcher.Register(models.SubmissionEventCompilingStarted, observerHub)
	dispatcher.Register(models.SubmissionEventTestingStarted, observerHub)
	dispatcher.Register(models.SubmissionEventTestStarted, observerHub)
	dispatcher.Register(models.SubmissionEventCompleted, observerHub)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub, err := pubsub.NewSubmissionsSub(rootCtx, js, dispatcher)
	if err != nil {
		panic(fmt.Sprintf("failed to create submissions subscriber: %s", err.Error()))
	}

	go func() {
		slog.Info("nats submissions subscriber started")
		if err := sub.Start(rootCtx); err != nil && err != context.Canceled {
			slog.Error("nats submissions subscriber stopped with error", slog.Any("error", err))
		}
	}()

	// Start observer server in background
	go func() {
		slog.Info("websocket server listening", slog.String("address", cfg.WsAddress))
		err := observer.Start()
		if err != nil {
			panic(fmt.Errorf("failed to start http server %w", err))
		}
	}()

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	cancel()

	slog.Info("shutting down websocket server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := observer.Close(shutdownCtx); err != nil {
		slog.Error("error shutting down server", slog.Any("error", err))
	}

	slog.Info("websocket server stopped")
}

func newMiddleware(usersUC interfaces.UsersUC, oryPublicClient *client.APIClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return middleware.AuthMiddleware(oryPublicClient.FrontendAPI)(
			middleware.UsersMiddleware(usersUC)(next),
		)
	}
}
