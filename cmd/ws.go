package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gate149/core/config"
	ws "github.com/gate149/core/internal/transport/ws/observer"
	"github.com/ilyakaznacheev/cleanenv"
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
  contestId (optional): Filter by contest ID
  userId (optional): Filter by user ID  
  problemId (optional): Filter by problem ID
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
		slog.String("nats_url", cfg.NatsUrl),
		slog.String("env", cfg.Env))

	// Initialize WebSocket hub
	hub, err := ws.NewHub(log, cfg.NatsUrl)
	if err != nil {
		log.Error("failed to create websocket hub", slog.Any("error", err))
		os.Exit(1)
	}
	defer hub.Close()

	// Start hub in background
	go hub.Run()
	log.Info("websocket hub started")

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "websocket",
		})
	})

	// Register WebSocket routes
	handler := ws.NewHandler(log, hub)
	mux.HandleFunc("/ws/submissions", handler.HandleSubmissions)

	// Configure CORS based on environment
	allowOrigins := map[string]bool{
		"https://gate149.ru":     true,
		"https://dev.gate149.ru": true,
	}
	if cfg.Env == "dev" {
		allowOrigins["http://localhost:3000"] = true
		allowOrigins["http://localhost:3001"] = true
	}

	// CORS Middleware
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:    cfg.WsAddress,
		Handler: corsHandler,
	}

	// Start server in background
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("websocket server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	log.Info("websocket server started",
		slog.String("address", cfg.WsAddress),
		slog.String("endpoint", "/ws/submissions"))

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	log.Info("shutting down websocket server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("error shutting down server", slog.Any("error", err))
	}

	log.Info("websocket server stopped")
}
