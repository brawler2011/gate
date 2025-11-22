package config

type Config struct {
	Env string `env:"ENV" env-default:"prod"`

	Address        string `env:"ADDRESS" required:"true"`
	PrivateAddress string `env:"PRIVATE_ADDRESS" env-default:":13011"`

	Pandoc      string `env:"PANDOC" required:"true"`
	PostgresDSN string `env:"POSTGRES_DSN" required:"true"`

	AdminUsername string `env:"ADMIN_USERNAME" env-default:"admin"`
	AdminPassword string `env:"ADMIN_PASSWORD" env-default:"admin"`

	Judge0URL string `env:"JUDGE0_URL" env-default:"http://localhost:2358"`

	NatsUrl string `env:"NATS_URL" env-default:"nats://localhost:4222"`

	KratosURl      string `env:"KRATOS_URL" env-default:"http://localhost:4433"`
	KratosAdminURL string `env:"KRATOS_ADMIN_URL" env-default:"http://localhost:4434"`
}
