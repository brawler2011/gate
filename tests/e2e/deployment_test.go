package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brawler2011/gate/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
)

// These are repository contract checks. Live business/DB/NATS tests belong to backend integration.
func TestDeploymentServicesAndRotation(t *testing.T) {
	for _, env := range []string{"local", "prod", "traefik"} {
		t.Run(env, func(t *testing.T) {
			cfg := helpers.ValidateComposeFile(t, helpers.GetDeployPath(env, "docker-compose.yml"))
			for _, retired := range []string{"otel-collector", "tempo", "loki", "victoriametrics", "grafana"} {
				require.NotContains(t, cfg.Services, retired)
				require.NotContains(t, cfg.Volumes, retired+"-data")
			}
			for name, svc := range cfg.Services {
				require.Equal(t, "json-file", svc.Logging.Driver, name)
				require.Equal(t, map[string]string{"max-size": "10m", "max-file": "3"}, svc.Logging.Options, name)
				for _, raw := range svc.Ports {
					port, err := helpers.ParsePortString(raw)
					require.NoError(t, err)
					require.NotContains(t, []int{4317, 4318, 9324, 9325, 9326, 9327}, port.ContainerPort, name)
				}
			}
			if env != "traefik" {
				for _, name := range []string{"postgres", "redis", "nats", "go-judge", "master", "volume", "filer", "s3"} {
					helpers.AssertServicePresent(t, cfg, name)
				}
				for _, name := range []string{"postgres-data", "redis-data", "nats-data", "seaweedfs-master", "seaweedfs-volume", "seaweedfs-filer"} {
					helpers.AssertNamedVolumePresent(t, cfg, name)
				}
			}
			if env == "local" {
				require.NotContains(t, cfg.Services, "traefik")
				helpers.AssertPortAllocation(t, cfg, "filer", 8888, 8888)
				helpers.AssertPortAllocation(t, cfg, "s3", 8333, 8333)
			}
		})
	}
}

func TestTraefikHasNoObservabilityRoutes(t *testing.T) {
	for _, path := range []string{helpers.GetDeployPath("traefik", "dynamic", "middlewares.yml"), helpers.GetDeployPath("prod", "docker-compose.yml")} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(data), "/otlp")
		require.NotContains(t, string(data), "grafana.")
	}
}

func TestOutboxLockingAndHeadersContract(t *testing.T) {
	for _, path := range []string{helpers.GetBackendPath("internal", "repository", "pg", "outbox.sql"), helpers.GetBackendPath("internal", "repository", "pg", "sqlc", "outbox.sql.go")} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(data), "FOR UPDATE SKIP LOCKED")
		require.Contains(t, string(data), "ORDER BY created_at ASC")
		require.Contains(t, string(data), "headers")
	}
}

func TestNoGateInstrumentation(t *testing.T) {
	for _, dir := range []string{helpers.GetBackendPath(), helpers.GetFrontendPath("lib"), helpers.GetFrontendPath("app")} {
		require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".go" && ext != ".ts" && ext != ".tsx" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			require.NotContains(t, string(data), "go.opentelemetry.io/", path)
			require.NotContains(t, string(data), "@opentelemetry/", path)
			require.NotContains(t, string(data), "@vercel/otel", path)
			return nil
		}))
	}
}
