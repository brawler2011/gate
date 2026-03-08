package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gate149/gate/backend/config"
	ws "github.com/gate149/gate/backend/internal/transport/ws/observer"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
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

	log.Info("starting websocket server",
		slog.String("address", cfg.WsAddress),
		slog.String("nats_url", cfg.GetNatsURL()),
		slog.String("env", cfg.Env))

	// Initialize WebSocket observer with ring buffer size
	const ringBufferSize = 10000
	observer := ws.NewObserver(cfg.WsAddress, ringBufferSize)
	log.Info("websocket observer initialized")

	// Start observer server in background
	go func() {
		log.Info("websocket server listening", slog.String("address", cfg.WsAddress))
		observer.Start()
	}()

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	log.Info("shutting down websocket server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := observer.Close(shutdownCtx); err != nil {
		log.Error("error shutting down server", slog.Any("error", err))
	}

	log.Info("websocket server stopped")
}
