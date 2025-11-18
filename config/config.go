package config

type Config struct {
	Env string `env:"ENV" env-default:"prod"`

	Address        string `env:"ADDRESS" required:"true"`
	PrivateAddress string `env:"PRIVATE_ADDRESS" env-default:":13011"`

	Pandoc      string `env:"PANDOC" required:"true"`
	PostgresDSN string `env:"POSTGRES_DSN" required:"true"`

	AdminUsername string `env:"ADMIN_USERNAME" env-default:"admin"`
	AdminPassword string `env:"ADMIN_PASSWORD" env-default:"admin"`

	S3Endpoint  string `env:"S3_ENDPOINT" required:"true"`
	S3AccessKey string `env:"S3_ACCESS_KEY" required:"true"`
	S3SecretKey string `env:"S3_SECRET_KEY" required:"true"`
	S3Bucket    string `env:"S3_BUCKET" env-default:"gate149"`

	CacheDir string `env:"CACHE_DIR" env-default:"/tmp"`

	Judge0URL string `env:"JUDGE0_URL" env-default:"http://localhost:2358"`

	NatsUrl string `env:"NATS_URL" env-default:"nats://localhost:4222"`

	KratosURl string `env:"KRATOS_URL" env-default:"http://localhost:4433"`

	RedisAddr     string `env:"REDIS_ADDR" env-default:"localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD" required:"true"`

	TypesenseURL    string `env:"TYPESENSE_URL" env-default:"http://steins.ru:8108"`
	TypesenseAPIKey string `env:"TYPESENSE_API_KEY" required:"true"`
}
