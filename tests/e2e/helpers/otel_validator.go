package helpers

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// OTelConfig represents the full structure of the OpenTelemetry Collector YAML configuration.
type OTelConfig struct {
	Receivers  map[string]interface{} `yaml:"receivers"`
	Processors map[string]interface{} `yaml:"processors"`
	Exporters  map[string]interface{} `yaml:"exporters"`
	Service    OTelService            `yaml:"service"`
}

type OTelService struct {
	Telemetry OTelTelemetry            `yaml:"telemetry"`
	Pipelines map[string]OTelPipeline  `yaml:"pipelines"`
}

type OTelTelemetry struct {
	Logs    map[string]interface{} `yaml:"logs"`
	Metrics OTelMetricsTelemetry   `yaml:"metrics"`
}

type OTelMetricsTelemetry struct {
	Address string `yaml:"address"`
}

type OTelPipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

// ParseOTelCollectorConfig reads and unmarshals the OTel Collector YAML configuration.
func ParseOTelCollectorConfig(path string) (*OTelConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read otel collector config %s: %w", path, err)
	}

	var cfg OTelConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse otel collector yaml %s: %w", path, err)
	}

	return &cfg, nil
}

// ValidateOTelCollectorConfig ensures the collector YAML exists and parses successfully.
func ValidateOTelCollectorConfig(t *testing.T, path string) *OTelConfig {
	t.Helper()
	cfg, err := ParseOTelCollectorConfig(path)
	require.NoError(t, err, "Failed to parse OTel Collector config at %s", path)
	require.NotEmpty(t, cfg.Receivers, "OTel Collector config has no receivers")
	require.NotEmpty(t, cfg.Exporters, "OTel Collector config has no exporters")
	require.NotEmpty(t, cfg.Service.Pipelines, "OTel Collector config has no service pipelines")
	return cfg
}

// AssertOTelReceivers validates OTLP gRPC and HTTP receiver endpoints and CORS.
func AssertOTelReceivers(t *testing.T, cfg *OTelConfig) {
	t.Helper()
	otlpRaw, ok := cfg.Receivers["otlp"]
	require.True(t, ok, "OTel config must declare 'otlp' receiver")

	otlpMap, ok := otlpRaw.(map[string]interface{})
	require.True(t, ok, "otlp receiver must be a map")

	protoRaw, ok := otlpMap["protocols"]
	require.True(t, ok, "otlp receiver must declare 'protocols'")
	protoMap := protoRaw.(map[string]interface{})

	// gRPC receiver endpoint
	grpcRaw, ok := protoMap["grpc"]
	require.True(t, ok, "otlp protocols must declare 'grpc'")
	grpcMap := grpcRaw.(map[string]interface{})
	require.Equal(t, "0.0.0.0:4317", grpcMap["endpoint"], "gRPC receiver endpoint mismatch")

	// HTTP receiver endpoint and CORS
	httpRaw, ok := protoMap["http"]
	require.True(t, ok, "otlp protocols must declare 'http'")
	httpMap := httpRaw.(map[string]interface{})
	require.Equal(t, "0.0.0.0:4318", httpMap["endpoint"], "HTTP receiver endpoint mismatch")

	corsRaw, ok := httpMap["cors"]
	require.True(t, ok, "otlp HTTP receiver must declare 'cors'")
	corsMap := corsRaw.(map[string]interface{})
	originsRaw, ok := corsMap["allowed_origins"]
	require.True(t, ok, "CORS config must declare 'allowed_origins'")
	origins := originsRaw.([]interface{})
	require.NotEmpty(t, origins, "allowed_origins cannot be empty")
}

// AssertOTelProcessors validates memory_limiter, batch, and resource processors.
func AssertOTelProcessors(t *testing.T, cfg *OTelConfig) {
	t.Helper()
	// memory_limiter
	memLimiterRaw, ok := cfg.Processors["memory_limiter"]
	require.True(t, ok, "processors must contain 'memory_limiter'")
	memMap := memLimiterRaw.(map[string]interface{})
	limitPct, ok := memMap["limit_percentage"].(int)
	require.True(t, ok, "memory_limiter must declare 'limit_percentage'")
	require.True(t, limitPct > 0 && limitPct <= 100, "limit_percentage must be between 1 and 100")

	// batch
	batchRaw, ok := cfg.Processors["batch"]
	require.True(t, ok, "processors must contain 'batch'")
	batchMap := batchRaw.(map[string]interface{})
	sendBatchSize, ok1 := batchMap["send_batch_size"].(int)
	sendBatchMaxSize, ok2 := batchMap["send_batch_max_size"].(int)
	require.True(t, ok1 && ok2, "batch processor must define send_batch_size and send_batch_max_size")
	require.True(t, sendBatchSize <= sendBatchMaxSize, "send_batch_size must be <= send_batch_max_size")

	// resource
	resRaw, ok := cfg.Processors["resource"]
	require.True(t, ok, "processors must contain 'resource'")
	resMap := resRaw.(map[string]interface{})
	attrsRaw, ok := resMap["attributes"].([]interface{})
	require.True(t, ok, "resource processor must define 'attributes'")
	require.NotEmpty(t, attrsRaw, "resource attributes cannot be empty")

	foundNamespace := false
	for _, attrRaw := range attrsRaw {
		attrMap := attrRaw.(map[string]interface{})
		if attrMap["key"] == "service.namespace" && attrMap["value"] == "gate" {
			foundNamespace = true
			break
		}
	}
	require.True(t, foundNamespace, "resource processor must set service.namespace = 'gate'")
}

// AssertOTelExporters validates Tempo, VictoriaMetrics, and Loki exporter configurations.
func AssertOTelExporters(t *testing.T, cfg *OTelConfig) {
	t.Helper()
	// otlp/tempo
	tempoRaw, ok := cfg.Exporters["otlp/tempo"]
	require.True(t, ok, "exporters must declare 'otlp/tempo'")
	tempoMap := tempoRaw.(map[string]interface{})
	require.Equal(t, "tempo:4317", tempoMap["endpoint"], "Tempo exporter endpoint mismatch")

	// prometheusremotewrite
	vmRaw, ok := cfg.Exporters["prometheusremotewrite"]
	require.True(t, ok, "exporters must declare 'prometheusremotewrite'")
	vmMap := vmRaw.(map[string]interface{})
	require.Equal(t, "http://victoriametrics:8428/api/v1/write", vmMap["endpoint"], "VictoriaMetrics exporter endpoint mismatch")

	// otlphttp/loki
	lokiRaw, ok := cfg.Exporters["otlphttp/loki"]
	require.True(t, ok, "exporters must declare 'otlphttp/loki'")
	lokiMap := lokiRaw.(map[string]interface{})
	require.Equal(t, "http://loki:3100/otlp", lokiMap["endpoint"], "Loki exporter endpoint mismatch")
}

// AssertOTelPipelines validates traces, metrics, and logs pipelines in service configuration.
func AssertOTelPipelines(t *testing.T, cfg *OTelConfig) {
	t.Helper()
	// Traces pipeline
	traces, ok := cfg.Service.Pipelines["traces"]
	require.True(t, ok, "service.pipelines must contain 'traces'")
	require.Contains(t, traces.Receivers, "otlp")
	require.Contains(t, traces.Exporters, "otlp/tempo")

	// Metrics pipeline
	metrics, ok := cfg.Service.Pipelines["metrics"]
	require.True(t, ok, "service.pipelines must contain 'metrics'")
	require.Contains(t, metrics.Receivers, "otlp")
	require.Contains(t, metrics.Exporters, "prometheusremotewrite")

	// Logs pipeline
	logs, ok := cfg.Service.Pipelines["logs"]
	require.True(t, ok, "service.pipelines must contain 'logs'")
	require.Contains(t, logs.Receivers, "otlp")
	require.Contains(t, logs.Exporters, "otlphttp/loki")
}

// AssertOTelTelemetryMetricsPort validates that internal telemetry metrics address is bound to 8889.
func AssertOTelTelemetryMetricsPort(t *testing.T, cfg *OTelConfig, expectedPort string) {
	t.Helper()
	require.Equal(t, expectedPort, cfg.Service.Telemetry.Metrics.Address,
		"Collector internal telemetry metrics address must be %s to prevent collision with SeaweedFS on 8888", expectedPort)
}
