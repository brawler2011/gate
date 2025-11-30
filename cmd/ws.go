package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gate149/core/config"
	"github.com/gate149/core/internal/ws"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/spf13/cobra"
)

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "Start the WebSocket server for submission testing progress",
	Long: `Start a dedicated WebSocket server that handles real-time 
submission testing progress updates.

The server connects to NATS and broadcasts testing events to connected clients.

Endpoint: GET /ws/submissions?ids=uuid1,uuid2,...

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

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add middlewares
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Configure CORS based on environment
	allowOrigins := "https://gate149.ru,https://dev.gate149.ru"
	if cfg.Env == "dev" {
		// In dev mode, also allow localhost
		allowOrigins = "https://gate149.ru,https://dev.gate149.ru,http://localhost:3000,http://localhost:3001"
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowOrigins,
		AllowMethods:     "GET,OPTIONS",
		AllowHeaders:     "*",
		AllowCredentials: true,
	}))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "websocket",
		})
	})

	// Register WebSocket routes
	handler := ws.NewHandler(log, hub)
	handler.RegisterRoutes(app)

	// Start server in background
	go func() {
		if err := app.Listen(cfg.WsAddress); err != nil {
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

	if err := app.Shutdown(); err != nil {
		log.Error("error shutting down server", slog.Any("error", err))
	}

	log.Info("websocket server stopped")
}
