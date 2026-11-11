package config

import "time"

type Config struct {
	Env string `env:"ENV" env-default:"prod"`

	Address      string `env:"ADDRESS" required:"true"`
	TrustedProxy string `env:"RATE_LIMIT_TRUSTED_PROXY"`

	PostgresDSN string `env:"POSTGRES_DSN" required:"true"`
	RedisURL    string `env:"REDIS_URL" required:"true"`
	NatsURL     string `env:"NATS_URL" required:"true"`

	// Workshop configuration
	GoJudgeGRPCAddr        string `env:"GOJUDGE_GRPC_ADDR" env-default:"localhost:5051"`
	GoJudgeHTTPURL         string `env:"GOJUDGE_HTTP_URL" required:"true"`
	GoJudgeToolchainDigest string `env:"GOJUDGE_TOOLCHAIN_DIGEST" required:"true"`
	GoJudgeArchitecture    string `env:"GOJUDGE_ARCHITECTURE" required:"true"`
	GoJudgeABI             string `env:"GOJUDGE_ABI" required:"true"`
	ProblemPolicyPath      string `env:"PROBLEM_POLICY_PATH" required:"true"`
	ProblemWorkDir         string `env:"PROBLEM_WORK_DIR" required:"true"`

	// Storage configuration
	StorageType      string `env:"STORAGE_TYPE" env-default:"local"`
	LocalStoragePath string `env:"LOCAL_STORAGE_PATH" env-default:"data"`

	// S3 configuration (SeaweedFS)
	S3Endpoint  string `env:"S3_ENDPOINT"`
	S3AccessKey string `env:"S3_ACCESS_KEY"`
	S3SecretKey string `env:"S3_SECRET_KEY"`

	// Message delivery configuration
	InboxRetentionDays int `env:"INBOX_RETENTION_DAYS" env-default:"7"`

	// Judging configuration
	JudgeWorkerCount int `env:"JUDGE_WORKER_COUNT" env-default:"4"`

	// Email configuration
	ResendAPIKey string `env:"RESEND_API_KEY"`
	EmailFrom    string `env:"EMAIL_FROM" env-default:"Gate <no-reply@gate.local>"`
	AppBaseURL   string `env:"APP_BASE_URL" env-default:"http://localhost:3000"`
}

func (c *Config) GetInboxRetention() time.Duration {
	days := c.InboxRetentionDays
	if days <= 0 {
		days = 7
	}
	return time.Duration(days) * 24 * time.Hour
}
