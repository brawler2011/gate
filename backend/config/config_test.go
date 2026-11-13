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
	// Local runs everything in one process; the clusters run API and worker pods
	// from the same config.
	envModes := map[string][]Mode{"local": {ModeAll}, "prod": {ModeAPI, ModeWorker}, "k3d": {ModeAPI, ModeWorker}}
	for env, modes := range envModes {
		for _, mode := range modes {
			t.Run(env+"/"+string(mode), func(t *testing.T) {
				cfg, err := Load(filepath.Join("..", "..", "deploy", env, "config.yaml"), mode)
				require.NoError(t, err)
				assert.Equal(t, "postgres://db:5432/app", cfg.Secrets.PostgresDSN)
				assert.Equal(t, 7*24*time.Hour, cfg.Inbox.Retention)
				assert.Equal(t, 30*time.Second, cfg.Problems.Lease)
			})
		}
	}
}

func loadModified(t *testing.T, edit func(string) string) error {
	t.Helper()
	return loadModifiedAs(t, ModeAll, edit)
}

func loadModifiedAs(t *testing.T, mode Mode, edit func(string) string) error {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "deploy", "prod", "config.yaml"))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(edit(string(data))), 0o600))
	_, err = Load(path, mode)
	return err
}

func TestInvalidConfigNamesField(t *testing.T) {
	cases := []struct {
		name, from, to, field string
	}{
		{"missing address", "address: :8080", "address: ''", "http.address"},
		{"relative go-judge url", "http_url: http://127.0.0.1:5050", "http_url: 127.0.0.1:5050", "go_judge.http_url"},
		{"unknown email provider", "provider: resend", "provider: smtp", "email.provider"},
		{"no judge workers", "workers: 1", "workers: 0", "judge.workers"},
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

func TestModeChecksOnlyItsSettings(t *testing.T) {
	setSecrets(t)
	noSandbox := func(s string) string { return strings.Replace(s, "grpc_addr: 127.0.0.1:5051", "grpc_addr: ''", 1) }
	require.NoError(t, loadModifiedAs(t, ModeAPI, noSandbox))
	err := loadModifiedAs(t, ModeWorker, noSandbox)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go_judge.grpc_addr:")

	noManifest := func(s string) string { return strings.Replace(s, "manifest_dir: /etc/judge", "manifest_dir: ''", 1) }
	require.NoError(t, loadModifiedAs(t, ModeWorker, noManifest))
	err = loadModifiedAs(t, ModeAPI, noManifest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "judge.manifest_dir:")
}

func TestParseMode(t *testing.T) {
	for _, m := range []Mode{ModeAll, ModeAPI, ModeWorker} {
		got, err := ParseMode(string(m))
		require.NoError(t, err)
		assert.Equal(t, m, got)
	}
	_, err := ParseMode("judge")
	require.Error(t, err)
}
