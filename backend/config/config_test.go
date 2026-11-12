package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setSecrets(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"POSTGRES_DSN": "postgres://db:5432/app", "REDIS_URL": "redis://redis:6379/0", "NATS_URL": "nats://nats:4222",
		"S3_ACCESS_KEY": "key", "S3_SECRET_KEY": "secret", "RESEND_API_KEY": "re_key",
	} {
		t.Setenv(k, v)
	}
}

func TestEnvironmentConfigsLoad(t *testing.T) {
	setSecrets(t)
	for _, env := range []string{"local", "prod", "k3d"} {
		t.Run(env, func(t *testing.T) {
			cfg, err := Load(filepath.Join("..", "..", "deploy", env, "config.yaml"))
			require.NoError(t, err)
			assert.Equal(t, "postgres://db:5432/app", cfg.Secrets.PostgresDSN)
			assert.Equal(t, 7*24*time.Hour, cfg.Inbox.Retention)
			assert.Equal(t, 30*time.Second, cfg.Problems.Lease)
		})
	}
}

func loadModified(t *testing.T, edit func(string) string) error {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "deploy", "prod", "config.yaml"))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(edit(string(data))), 0o600))
	_, err = Load(path)
	return err
}

func TestInvalidConfigNamesField(t *testing.T) {
	cases := []struct {
		name, from, to, field string
	}{
		{"missing address", "address: :8080", "address: ''", "http.address"},
		{"relative go-judge url", "http_url: http://go-judge:5050", "http_url: go-judge:5050", "go_judge.http_url"},
		{"unknown email provider", "provider: resend", "provider: smtp", "email.provider"},
		{"no judge workers", "workers: 4", "workers: 0", "judge.workers"},
		{"lease too short", "lease: 30s", "lease: 1s", "problems.lease"},
		{"compile diagnostic too large", "diagnostic: 64KiB", "diagnostic: 65KiB", "problems.limits.compile.diagnostic"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setSecrets(t)
			err := loadModified(t, func(s string) string { return strings.Replace(s, c.from, c.to, 1) })
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.field+":")
		})
	}
}

func TestMissingSecretNamesVariable(t *testing.T) {
	setSecrets(t)
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("POSTGRES_DSN", "")
	err := loadModified(t, func(s string) string { return s })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RESEND_API_KEY: is required")
	assert.Contains(t, err.Error(), "POSTGRES_DSN: is required")
}

func TestUnknownFieldIsRejected(t *testing.T) {
	setSecrets(t)
	err := loadModified(t, func(s string) string { return s + "unexpected: true\n" })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected")
}
