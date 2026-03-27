package cmd

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gate149/gate/backend/config"
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/internal/worker/judge"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var judgeCmd = &cobra.Command{
	Use:   "judge",
	Short: "Start the judging worker",
	Long:  "Start the judging worker that processes submission.created events from NATS",
	Run:   runJudge,
}

func init() {
	judgeCmd.Flags().String("env", ".env", "Path to .env file")
	rootCmd.AddCommand(judgeCmd)
}

func runJudge(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	envPath, _ := cmd.Flags().GetString("env")
	var cfg config.Config
	if err := godotenv.Load(envPath); err != nil {
		slog.Error("failed to load env file", slog.Any("error", err))
		return
	}
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("failed to read config from environment", slog.Any("error", err))
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if cfg.JudgeTempDir == "" {
		cfg.JudgeTempDir = filepath.Join(os.TempDir(), "judge")
	}
	if err := os.MkdirAll(cfg.JudgeTempDir, 0o755); err != nil {
		slog.Error("failed to create judge temp directory", slog.Any("error", err))
		return
	}

	logger.Info("starting judge worker",
		slog.String("env", cfg.Env),
		slog.Int("worker_count", cfg.JudgeWorkerCount),
		slog.String("temp_dir", cfg.JudgeTempDir),
	)

	// Initialize PostgreSQL connection
	pool, err := pkg.NewPostgresDB(cfg.GetPostgresDSN())
	if err != nil {
		logger.Error("failed to connect to postgres", slog.Any("error", err))
		return
	}
	defer pool.Close()
	logger.Info("successfully connected to postgres")

	// Initialize repositories
	submissionsRepo := pg.NewSubmissionsRepo(pool)
	packagesRepo := pg.NewPackagesRepo(pool)

	// Initialize S3 client
	s3Client := pkg.NewS3Client(pkg.S3Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Region:    defaultS3Region,
	})
	logger.Info("successfully initialized S3 client", slog.String("endpoint", cfg.S3Endpoint))

	// Initialize sandbox client
	sandboxClient, err := sandbox.NewClient(sandbox.ClientConfig{
		Protocol: sandbox.ProtocolGRPC,
		BaseURL:  cfg.GoJudgeGRPCAddr,
	})
	if err != nil {
		logger.Error("failed to create sandbox client", slog.Any("error", err))
		return
	}
	logger.Info("successfully initialized sandbox client", slog.String("addr", cfg.GoJudgeGRPCAddr))

	// Initialize NATS JetStream
	js, err := pkg.NewNatsJetStream(cfg.GetNatsURL())
	if err != nil {
		logger.Error("failed to connect to NATS JetStream", slog.Any("error", err))
		return
	}
	logger.Info("successfully connected to NATS JetStream", slog.String("url", cfg.GetNatsURL()))

	if err := pkg.EnsureSubmissionsStream(ctx, js); err != nil {
		logger.Error("failed to ensure SUBMISSIONS stream", slog.Any("error", err))
		return
	}
	logger.Info("SUBMISSIONS stream ready")

	// Create event publisher
	eventPublisher := judge.NewEventPublisher(js)

	// Create judge use case
	judgeUC := usecase.NewJudgeUseCase(
		submissionsRepo,
		packagesRepo,
		s3Client,
		defaultS3PackageBucket,
		cfg.JudgeTempDir,
		sandboxClient,
		eventPublisher,
	)

	// Create judge worker
	worker, err := judge.NewJudgeWorker(ctx, js, judgeUC, cfg.JudgeWorkerCount)
	if err != nil {
		logger.Error("failed to create judge worker", slog.Any("error", err))
		return
	}

	// Start HTTP health check endpoint
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	healthServer := &http.Server{
		Addr:    ":8082",
		Handler: healthMux,
	}

	go func() {
		logger.Info("starting health check server", slog.String("addr", ":8082"))
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health check server error", slog.Any("error", err))
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start worker in goroutine
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- worker.Start(ctx)
	}()

	// Wait for shutdown signal or worker error
	select {
	case <-sigChan:
		logger.Info("shutdown signal received")
		cancel()     // cancel context to stop worker
		<-workerDone // wait for worker to finish
	case err := <-workerDone:
		if err != nil && err != context.Canceled {
			logger.Error("worker error", slog.Any("error", err))
		}
	}

	// Shutdown health check server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown health server", slog.Any("error", err))
	}

	logger.Info("judge worker stopped")
}
