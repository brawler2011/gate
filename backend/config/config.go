package config

import "fmt"

type Config struct {
	Env string `env:"ENV" env-default:"prod"`

	Address        string `env:"ADDRESS" required:"true"`
	AllowedOrigins string `env:"ALLOWED_ORIGINS" env-default:"http://localhost,http://127.0.0.1"`

	PostgresDSN      string `env:"POSTGRES_DSN"`
	PostgresUser     string `env:"POSTGRES_USER" env-default:"postgres"`
	PostgresPassword string `env:"POSTGRES_PASSWORD" env-default:"postgres"`
	PostgresHost     string `env:"POSTGRES_HOST" env-default:"localhost"`
	PostgresPort     string `env:"POSTGRES_PORT" env-default:"5432"`
	PostgresDatabase string `env:"POSTGRES_DB" env-default:"gate"`
	PostgresSSLMode  string `env:"POSTGRES_SSLMODE" env-default:"disable"`

	RedisURL      string `env:"REDIS_URL"`
	RedisHost     string `env:"REDIS_HOST" env-default:"localhost"`
	RedisPort     string `env:"REDIS_PORT" env-default:"6379"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisDB       int    `env:"REDIS_DB_INDEX" env-default:"0"`

	AdminUsername string `env:"ADMIN_USERNAME" env-default:"admin"`
	AdminPassword string `env:"ADMIN_PASSWORD" env-default:"admin"`

	NatsUrl  string `env:"NATS_URL"`
	NatsHost string `env:"NATS_HOST" env-default:"localhost"`
	NatsPort string `env:"NATS_PORT" env-default:"4222"`
	// Workshop configuration
	GoJudgeGRPCAddr string `env:"GOJUDGE_GRPC_ADDR" env-default:"localhost:5051"`

	// S3 configuration (SeaweedFS)
	S3Endpoint  string `env:"S3_ENDPOINT" required:"true"`
	S3AccessKey string `env:"S3_ACCESS_KEY" required:"true"`
	S3SecretKey string `env:"S3_SECRET_KEY" required:"true"`

	// Judging configuration
	JudgeWorkerCount int    `env:"JUDGE_WORKER_COUNT" env-default:"4"`
	JudgeTempDir     string `env:"JUDGE_TEMP_DIR"`                     // defaults to os.TempDir()/judge at runtime
	JudgeTimeout     int    `env:"JUDGE_TIMEOUT" env-default:"300000"` // milliseconds
	JudgeMaxRetries  int    `env:"JUDGE_MAX_RETRIES" env-default:"3"`
}

func (c *Config) GetPostgresDSN() string {
	if c.PostgresDSN != "" {
		return c.PostgresDSN
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.PostgresUser, c.PostgresPassword, c.PostgresHost, c.PostgresPort, c.PostgresDatabase, c.PostgresSSLMode)
}

func (c *Config) GetRedisURL() string {
	if c.RedisURL != "" {
		return c.RedisURL
	}
	// Redis URL format: redis://:<password>@<host>:<port>/<db_index>
	if c.RedisPassword != "" {
		return fmt.Sprintf("redis://:%s@%s:%s/%d", c.RedisPassword, c.RedisHost, c.RedisPort, c.RedisDB)
	}
	return fmt.Sprintf("redis://%s:%s/%d", c.RedisHost, c.RedisPort, c.RedisDB)
}

func (c *Config) GetNatsURL() string {
	if c.NatsUrl != "" {
		return c.NatsUrl
	}
	return fmt.Sprintf("nats://%s:%s", c.NatsHost, c.NatsPort)
}
