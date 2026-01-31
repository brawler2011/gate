package config

type Config struct {
	Env string `env:"ENV" env-default:"prod"`

	Address        string `env:"ADDRESS" required:"true"`
	PrivateAddress string `env:"PRIVATE_ADDRESS" env-default:":13011"`
	WsAddress      string `env:"WS_ADDRESS" env-default:":8081"`

	Pandoc      string `env:"PANDOC" required:"true"`
	PostgresDSN string `env:"POSTGRES_DSN" required:"true"`
	RedisURL    string `env:"REDIS_URL" env-default:"redis://localhost:6379"`

	AdminUsername string `env:"ADMIN_USERNAME" env-default:"admin"`
	AdminPassword string `env:"ADMIN_PASSWORD" env-default:"admin"`

	Judge0URL string `env:"JUDGE0_URL" env-default:"http://localhost:2358"`

	NatsUrl string `env:"NATS_URL" env-default:"nats://localhost:4222"`

	KratosURl      string `env:"KRATOS_URL" env-default:"http://localhost:4433"`
	KratosAdminURL string `env:"KRATOS_ADMIN_URL" env-default:"http://localhost:4434"`

	// Workshop configuration
	WorkshopReposDir string `env:"WORKSHOP_REPOS_DIR" env-default:"./workshop-repos"`
	GoJudgeGRPCAddr  string `env:"GOJUDGE_GRPC_ADDR" env-default:"localhost:5051"`

	// S3 configuration (SeaweedFS)
	S3Endpoint      string `env:"S3_ENDPOINT" required:"true"`
	S3AccessKey     string `env:"S3_ACCESS_KEY" required:"true"`
	S3SecretKey     string `env:"S3_SECRET_KEY" required:"true"`
	S3Region        string `env:"S3_REGION" env-default:"us-east-1"`
	S3AvatarBucket  string `env:"S3_AVATAR_BUCKET" env-default:"avatars"`
	S3PackageBucket string `env:"S3_PACKAGE_BUCKET" env-default:"problem-packages"`
	S3BlogBucket    string `env:"S3_BLOG_BUCKET" env-default:"blog-images"`

	// Judging configuration
	JudgeWorkerCount int    `env:"JUDGE_WORKER_COUNT" env-default:"4"`
	JudgeTempDir     string `env:"JUDGE_TEMP_DIR" env-default:"/tmp/judge"`
	JudgeTimeout     int    `env:"JUDGE_TIMEOUT" env-default:"300000"` // milliseconds
	JudgeMaxRetries  int    `env:"JUDGE_MAX_RETRIES" env-default:"3"`
}

type WsConfig struct {
	Env       string `env:"ENV" env-default:"prod"`
	WsAddress string `env:"WS_ADDRESS" env-default:":8081"`
	NatsUrl   string `env:"NATS_URL" env-default:"nats://localhost:4222"`
}
