// Package config loads the backend configuration: non-secret settings from a
// versioned YAML file per environment, secrets and connection strings from the
// environment.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP     HTTP          `yaml:"http"`
	GoJudge  GoJudge       `yaml:"go_judge"`
	Storage  Storage       `yaml:"storage"`
	Email    Email         `yaml:"email"`
	Inbox    Inbox         `yaml:"inbox"`
	Judge    Judge         `yaml:"judge"`
	Problems ProblemPolicy `yaml:"problems"`

	Secrets Secrets `yaml:"-"`
	// Toolchain is the go-judge image identity pinned into every build.
	Toolchain Toolchain `yaml:"-"`
}

type HTTP struct {
	Address string `yaml:"address"`
	// TrustedProxy may supply forwarded client IPs; empty means direct clients only.
	TrustedProxy  string `yaml:"trusted_proxy"`
	SecureCookies bool   `yaml:"secure_cookies"`
}

type GoJudge struct {
	GRPCAddr string `yaml:"grpc_addr"`
	HTTPURL  string `yaml:"http_url"`
}

type Storage struct {
	S3Endpoint string `yaml:"s3_endpoint"`
}

const (
	EmailProviderResend = "resend"
	EmailProviderLog    = "log"
)

type Email struct {
	// Provider is "resend" to deliver mail or "log" to only log it.
	Provider   string `yaml:"provider"`
	From       string `yaml:"from"`
	AppBaseURL string `yaml:"app_base_url"`
}

type Inbox struct {
	Retention time.Duration `yaml:"retention"`
}

type Judge struct {
	Workers int `yaml:"workers"`
}

// Secrets are read only from the environment.
type Secrets struct {
	PostgresDSN  string `env:"POSTGRES_DSN"`
	RedisURL     string `env:"REDIS_URL"`
	NatsURL      string `env:"NATS_URL"`
	S3AccessKey  string `env:"S3_ACCESS_KEY"`
	S3SecretKey  string `env:"S3_SECRET_KEY"`
	ResendAPIKey string `env:"RESEND_API_KEY"`
}

type Toolchain struct {
	Digest       string `env:"GOJUDGE_TOOLCHAIN_DIGEST"`
	Architecture string `env:"GOJUDGE_ARCHITECTURE"`
	ABI          string `env:"GOJUDGE_ABI"`
}

// Load reads the YAML file at path, rejecting unknown fields, then the secrets
// from the environment, and validates the result.
func Load(path string) (Config, error) {
	var cfg Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode config %s: %w", path, err)
	}
	if err := cleanenv.ReadEnv(&cfg.Secrets); err != nil {
		return cfg, fmt.Errorf("read secrets: %w", err)
	}
	if err := cleanenv.ReadEnv(&cfg.Toolchain); err != nil {
		return cfg, fmt.Errorf("read toolchain: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// Validate reports every invalid setting, each prefixed with its YAML path or
// environment variable.
func (c Config) Validate() error {
	var v validator
	v.required("http.address", c.HTTP.Address)
	v.required("go_judge.grpc_addr", c.GoJudge.GRPCAddr)
	v.url("go_judge.http_url", c.GoJudge.HTTPURL)
	v.url("storage.s3_endpoint", c.Storage.S3Endpoint)
	switch c.Email.Provider {
	case EmailProviderResend:
		v.required("email.from", c.Email.From)
		v.required("RESEND_API_KEY", c.Secrets.ResendAPIKey)
	case EmailProviderLog:
	default:
		v.fail("email.provider", "must be %q or %q", EmailProviderResend, EmailProviderLog)
	}
	v.url("email.app_base_url", c.Email.AppBaseURL)
	v.check(c.Inbox.Retention > 0, "inbox.retention", "must be positive")
	v.check(c.Judge.Workers >= 1, "judge.workers", "must be at least 1")
	c.Problems.validate(&v)

	v.required("POSTGRES_DSN", c.Secrets.PostgresDSN)
	v.required("REDIS_URL", c.Secrets.RedisURL)
	v.required("NATS_URL", c.Secrets.NatsURL)
	v.required("S3_ACCESS_KEY", c.Secrets.S3AccessKey)
	v.required("S3_SECRET_KEY", c.Secrets.S3SecretKey)
	v.required("GOJUDGE_TOOLCHAIN_DIGEST", c.Toolchain.Digest)
	v.required("GOJUDGE_ARCHITECTURE", c.Toolchain.Architecture)
	v.required("GOJUDGE_ABI", c.Toolchain.ABI)
	return errors.Join(v.errs...)
}

type validator struct{ errs []error }

func (v *validator) fail(field, format string, args ...any) {
	v.errs = append(v.errs, fmt.Errorf("%s: %s", field, fmt.Sprintf(format, args...)))
}

func (v *validator) check(ok bool, field, message string) {
	if !ok {
		v.fail(field, "%s", message)
	}
}

func (v *validator) required(field, value string) {
	v.check(value != "", field, "is required")
}

func (v *validator) url(field, value string) {
	if value == "" {
		v.fail(field, "is required")
		return
	}
	u, err := url.Parse(value)
	v.check(err == nil && u.Scheme != "" && u.Host != "", field, "must be an absolute URL")
}
