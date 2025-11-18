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
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/internal/problems"
	"github.com/gate149/core/internal/queue"
	"github.com/gate149/core/internal/submissions"
	"github.com/gate149/core/internal/users"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/ilyakaznacheev/cleanenv"
	ory "github.com/ory/client-go"

	"github.com/redis/go-redis/v9"
)

// MergedHandlers combines all handler structs
type MergedHandlers struct {
	*users.UsersHandlers
	*contests.ContestsHandlers
	*problems.ProblemsHandlers
	*submissions.SolutionsHandlers
	*health.HealthHandlers
	*permissions.PermissionsHandler
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

	// Create slog logger
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

	logger.Info("connecting to s3")
	s3Client, err := pkg.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey)
	if err != nil {
		logger.Error("error connecting to s3", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("successfully connected to s3")

	logger.Info("connecting to redis")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		logger.Error("Failed to connect to Redis", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("successfully connected to redis")

	usersRepo := users.NewRepository(db)
	usersUC := users.NewUseCase(usersRepo)

	np, err := pkg.NewNatsPublisher(cfg.NatsUrl)
	if err != nil {
		logger.Error("error connecting to nats", slog.Any("error", err))
		os.Exit(1)
	}

	pandocClient := pkg.NewPandocClient(&http.Client{}, cfg.Pandoc)

	problemsRepo := problems.NewRepository(db)
	s3Repo := problems.NewS3Repository(s3Client, "tester-problems-archives")

	problemsUC, err := problems.NewUseCase(problemsRepo, pandocClient, s3Repo, cfg.CacheDir)
	if err != nil {
		logger.Error("failed to create problems use case", slog.Any("error", err))
		os.Exit(1)
	}

	contestsRepo := contests.NewRepository(db)
	contestsUC := contests.NewContestUseCase(contestsRepo)

	permissionsUC := permissions.NewUseCase(contestsRepo, usersRepo)
	logger.Info("successfully initialized permissions system")

	solutionsRepo := submissions.NewRepository(db)
	judgeQueue := "submissions:judge"
	solutionsUC := submissions.NewUseCase(solutionsRepo, problemsUC, np, redisClient, judgeQueue)

	// Initialize test results repository
	//testResultsRepo := testresults.NewRepository(db)

	if err := os.MkdirAll(cfg.CacheDir, 0700); err != nil {
		panic(fmt.Errorf("failed to create cache dir: %v", err))
	}

	server := fiber.New(fiber.Config{
		BodyLimit: 512 * 1024 * 1024, // 512 MB for problem archives and submissions
	})

	// Add CORS middleware
	server.Use(func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-Session-ID")

		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.Next()
	})

	// Add request logging middleware with timing and context
	server.Use(middleware.RequestLoggerMiddleware(logger))

	merged := &MergedHandlers{
		UsersHandlers:     users.NewHandlers(usersUC),
		ContestsHandlers:  contests.NewHandlers(contestsUC, permissionsUC),
		ProblemsHandlers:  problems.NewHandlers(problemsUC, permissionsUC),
		SolutionsHandlers: submissions.NewHandlers(solutionsUC, contestsUC, permissionsUC, usersUC),
		HealthHandlers:    health.NewHandlers(),
	}

	oryConfiguration := ory.NewConfiguration()
	oryConfiguration.Servers = []ory.ServerConfiguration{{
		URL: cfg.KratosURl,
	}}

	oryClient := ory.NewAPIClient(oryConfiguration)

	testerv1.RegisterHandlersWithOptions(server, merged, testerv1.FiberServerOptions{
		Middlewares: []testerv1.MiddlewareFunc{
			middleware.ErrorHandlerMiddleware(logger),
			middleware.AuthMiddleware(oryClient.FrontendAPI),
			middleware.UsersMiddleware(usersUC),
		},
	})

	// Start queue consumer for user creation
	consumer := queue.NewConsumer(redisClient, usersUC)
	go func() {
		consumerCtx := context.Background()
		consumer.StartConsuming(consumerCtx, "user:created")
	}()

	// Initialize Judge0 components
	//logger.Info("initializing judge0 client", slog.String("url", cfg.Judge0URL))
	//judge0Client, err := pkg.NewJudge0Client(cfg.Judge0URL)
	//if err != nil {
	//	logger.Error("failed to create judge0 client", slog.Any("error", err))
	//	os.Exit(1)
	//}
	//logger.Info("successfully initialized judge0 client")

	// Initialize test files manager
	//testFilesManager := testing.NewTestFilesManager(s3Client, cfg.S3Bucket, cfg.CacheDir)

	// Initialize event publisher
	//eventPublisher := testing.NewEventPublisher(np)

	// Initialize judge
	//judge := testing.NewJudge(judge0Client, testFilesManager, eventPublisher, testResultsRepo, logger)

	// Initialize and start judge worker
	//judgeWorker := testing.NewWorker(
	//	redisClient,
	//	judge,
	//	solutionsRepo,
	//	problemsUC,
	//	judgeQueue,
	//	logger,
	//)

	// Start judge worker in background
	//go func() {
	//	workerCtx := context.Background()
	//	judgeWorker.Start(workerCtx)
	//}()
	//logger.Info("started judge worker", slog.String("queue", judgeQueue))

	// Start private server for Kratos webhooks
	kratosHandler := kratos.NewKratosHandler(usersUC)
	privateServer := fiber.New(fiber.Config{
		BodyLimit: 1024 * 1024, // 1 MB for webhook requests
	})

	// Setup private server routes
	privateServer.Post("/webhook/kratos", kratosHandler.HandleKratosWebhook)
	privateServer.Get("/health", kratosHandler.HealthCheck)

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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop
}
