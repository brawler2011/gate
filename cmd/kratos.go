package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gate149/core/config"
	"github.com/gate149/core/internal/repository/pg"
	"github.com/gate149/core/internal/transport/rest/kratos"
	"github.com/gate149/core/internal/usecase"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/ilyakaznacheev/cleanenv"
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

	// Admin API client for identity management (port 4434)
	oryAdminConfiguration := ory.NewConfiguration()
	oryAdminConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosAdminURL,
	}}
	oryAdminClient := ory.NewAPIClient(oryAdminConfiguration)

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

	logger.Info("private server started", slog.String("address", cfg.PrivateAddress))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop

	logger.Info("shutting down private server...")

	// Give some time for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := privateServer.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("error shutting down private server", slog.Any("error", err))
	}

	logger.Info("shutdown complete")
}
