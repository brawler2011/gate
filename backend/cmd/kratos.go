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
	"github.com/gate149/gate/backend/internal/repository/pg"
	"github.com/gate149/gate/backend/internal/transport/rest/kratos"
	"github.com/gate149/gate/backend/internal/usecase"
	"github.com/gate149/gate/backend/pkg"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	ory "github.com/ory/client-go"
	"github.com/spf13/cobra"
)

var kratosCmd = &cobra.Command{
	Use:   "kratos",
	Short: "Start the private Kratos webhook server",
	Run: func(cmd *cobra.Command, args []string) {
		envFile, _ := cmd.Flags().GetString("env")
		runKratos(envFile)
	},
}

func init() {
	kratosCmd.Flags().String("env", "", "path to environment file")
	rootCmd.AddCommand(kratosCmd)
}

func runKratos(envFile string) {
	var cfg config.Config
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

	usersRepo := pg.NewUsersRepo(pool)
	outboxRepo := pg.NewOutboxRepo(pool)
	txManager := pg.NewTransactor(pool)

	usersUC := usecase.NewUsersUseCase(usersRepo, outboxRepo, txManager)

	// Admin API client for identity management (port 4434)
	oryAdminConfiguration := ory.NewConfiguration()
	oryAdminConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosAdminURL,
	}}
	oryAdminClient := ory.NewAPIClient(oryAdminConfiguration)

	// Start private server for Kratos webhooks
	kratosHandler := kratos.NewKratosHandler(usersUC, oryAdminClient.IdentityAPI)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook/kratos", kratosHandler.HandleKratosWebhook)

	privateServer := &http.Server{
		Addr:    cfg.PrivateAddress,
		Handler: mux,
	}

	go func() {
		err := privateServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error("error starting private server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	logger.Info("private server started", slog.String("address", cfg.PrivateAddress))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop

	logger.Info("shutting down private server...")

	// Give some time for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := privateServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down private server", slog.Any("error", err))
	}

	logger.Info("shutdown complete")
}
