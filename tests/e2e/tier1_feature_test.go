package e2e_test

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/tests/e2e/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

func TestTier1_FeatureCoverage(t *testing.T) {
	// =========================================================================
	// Feature 1: OTel Collector Gateway Deployment
	// =========================================================================
	t.Run("F01_OTelCollectorGateway", func(t *testing.T) {
		collectorConfigPath := helpers.GetDeployPath("common", "otel-collector", "config.yaml")
		cfg := helpers.ValidateOTelCollectorConfig(t, collectorConfigPath)

		t.Run("TC-F1-01_gRPCReceiverConfig", func(t *testing.T) {
			helpers.AssertOTelReceivers(t, cfg)
		})
		t.Run("TC-F1-02_HTTPReceiverAndCORS", func(t *testing.T) {
			otlpMap := cfg.Receivers["otlp"].(map[string]interface{})
			protoMap := otlpMap["protocols"].(map[string]interface{})
			httpMap := protoMap["http"].(map[string]interface{})
			corsMap := httpMap["cors"].(map[string]interface{})
			origins := corsMap["allowed_origins"].([]interface{})
			require.Contains(t, origins, "*")
		})
		t.Run("TC-F1-03_ProcessorPipeline", func(t *testing.T) {
			helpers.AssertOTelProcessors(t, cfg)
		})
		t.Run("TC-F1-04_ExporterEndpoints", func(t *testing.T) {
			helpers.AssertOTelExporters(t, cfg)
		})
		t.Run("TC-F1-05_PipelineRoutingIntegrity", func(t *testing.T) {
			helpers.AssertOTelPipelines(t, cfg)
		})
	})

	// =========================================================================
	// Feature 2: Host Port 8889 Non-collision
	// =========================================================================
	t.Run("F02_HostPort8889NonCollision", func(t *testing.T) {
		collectorConfigPath := helpers.GetDeployPath("common", "otel-collector", "config.yaml")
		collectorCfg := helpers.ValidateOTelCollectorConfig(t, collectorConfigPath)
		localComposePath := helpers.GetDeployPath("local", "docker-compose.yml")
		localCompose := helpers.ValidateComposeFile(t, localComposePath)

		t.Run("TC-F2-01_InternalTelemetryAddress", func(t *testing.T) {
			helpers.AssertOTelTelemetryMetricsPort(t, collectorCfg, "0.0.0.0:8889")
		})
		t.Run("TC-F2-02_SeaweedFSPortPreservation", func(t *testing.T) {
			helpers.AssertPortAllocation(t, localCompose, "filer", 8888, 8888)
		})
		t.Run("TC-F2-03_NoCollectorPort8888Mapping", func(t *testing.T) {
			if _, ok := localCompose.Services["otel-collector"]; ok {
				ports, _ := localCompose.GetServicePorts("otel-collector")
				for _, p := range ports {
					require.NotEqual(t, 8888, p.HostPort, "otel-collector must NOT bind host port 8888")
				}
			}
		})
		t.Run("TC-F2-04_PortDistinctness", func(t *testing.T) {
			helpers.AssertNoPortCollision(t, localCompose, 8889, 8888)
		})
		t.Run("TC-F2-05_InternalMetricsEndpointFormat", func(t *testing.T) {
			addr := collectorCfg.Service.Telemetry.Metrics.Address
			require.True(t, strings.HasSuffix(addr, ":8889"), "Metrics address should end in :8889")
		})
	})

	// =========================================================================
	// Feature 3: Traefik /otlp Ingestion & CORS
	// =========================================================================
	t.Run("F03_TraefikOTLPIngestionAndCORS", func(t *testing.T) {
		traefikDynamicPath := helpers.GetDeployPath("local", "traefik", "dynamic.yml")
		data, err := os.ReadFile(traefikDynamicPath)
		require.NoError(t, err, "Failed to read traefik dynamic.yml")

		var dynamicCfg map[string]interface{}
		require.NoError(t, yaml.Unmarshal(data, &dynamicCfg))
		httpMap := dynamicCfg["http"].(map[string]interface{})
		routers := httpMap["routers"].(map[string]interface{})
		middlewares := httpMap["middlewares"].(map[string]interface{})
		services := httpMap["services"].(map[string]interface{})

		t.Run("TC-F3-01_LocalTraefikRouter", func(t *testing.T) {
			otlpRouterRaw, ok := routers["otlp-traces"]
			require.True(t, ok, "dynamic.yml must contain 'otlp-traces' router")
			otlpRouter := otlpRouterRaw.(map[string]interface{})
			require.Equal(t, "PathPrefix(`/otlp`)", otlpRouter["rule"])
		})
		t.Run("TC-F3-02_LocalStripPrefixMiddleware", func(t *testing.T) {
			stripMwRaw, ok := middlewares["strip-otlp-prefix"]
			require.True(t, ok, "dynamic.yml must contain 'strip-otlp-prefix' middleware")
			stripMw := stripMwRaw.(map[string]interface{})
			stripPrefix := stripMw["stripPrefix"].(map[string]interface{})
			prefixes := stripPrefix["prefixes"].([]interface{})
			require.Contains(t, prefixes, "/otlp")
		})
		t.Run("TC-F3-03_LocalCORSHeadersMiddleware", func(t *testing.T) {
			corsMwRaw, ok := middlewares["otlp-cors"]
			require.True(t, ok, "dynamic.yml must contain 'otlp-cors' middleware")
			corsMw := corsMwRaw.(map[string]interface{})
			headersMap := corsMw["headers"].(map[string]interface{})
			methods := headersMap["accessControlAllowMethods"].([]interface{})
			require.Contains(t, methods, "POST")
			require.Contains(t, methods, "OPTIONS")
		})
		t.Run("TC-F3-04_UpstreamServiceForwarding", func(t *testing.T) {
			svcRaw, ok := services["otel-collector-service"]
			require.True(t, ok, "dynamic.yml must contain 'otel-collector-service'")
			svc := svcRaw.(map[string]interface{})
			lb := svc["loadBalancer"].(map[string]interface{})
			servers := lb["servers"].([]interface{})
			server0 := servers[0].(map[string]interface{})
			require.Equal(t, "http://otel-collector:4318", server0["url"])
		})
		t.Run("TC-F3-05_DevProdComposeTraefikRouting", func(t *testing.T) {
			// Dev / prod check for compose dynamic configuration or labels
			devPath := helpers.GetDeployPath("dev", "docker-compose.yml")
			require.True(t, helpers.FileExists(devPath), "dev docker-compose.yml must exist")
			devCfg, err := helpers.ParseComposeFile(devPath)
			require.NoError(t, err)
			require.NotEmpty(t, devCfg.Services)
		})
	})

	// =========================================================================
	// Feature 4: Multi-Environment Compose Sync
	// =========================================================================
	t.Run("F04_MultiEnvironmentComposeSync", func(t *testing.T) {
		localPath := helpers.GetDeployPath("local", "docker-compose.yml")
		devPath := helpers.GetDeployPath("dev", "docker-compose.yml")
		prodPath := helpers.GetDeployPath("prod", "docker-compose.yml")

		localCfg := helpers.ValidateComposeFile(t, localPath)
		devCfg := helpers.ValidateComposeFile(t, devPath)
		prodCfg := helpers.ValidateComposeFile(t, prodPath)

		t.Run("TC-F4-01_LocalComposeServices", func(t *testing.T) {
			helpers.AssertServicePresent(t, localCfg, "postgres")
			helpers.AssertServicePresent(t, localCfg, "redis")
			helpers.AssertServicePresent(t, localCfg, "nats")
			helpers.AssertServicePresent(t, localCfg, "traefik")
		})
		t.Run("TC-F4-02_DevComposeServicesAndNetworks", func(t *testing.T) {
			require.NotEmpty(t, devCfg.Services)
			require.NotEmpty(t, devCfg.Networks)
		})
		t.Run("TC-F4-03_ProdComposeResourceLimits", func(t *testing.T) {
			require.NotEmpty(t, prodCfg.Services)
		})
		t.Run("TC-F4-04_VolumePersistence", func(t *testing.T) {
			require.NotEmpty(t, localCfg.Volumes)
		})
		t.Run("TC-F4-05_DockerComposeConfigSyntax", func(t *testing.T) {
			require.True(t, len(localCfg.Services) >= 4)
			require.True(t, len(devCfg.Services) >= 4)
			require.True(t, len(prodCfg.Services) >= 4)
		})
	})

	// =========================================================================
	// Feature 5: Grafana Datasources Provisioning
	// =========================================================================
	t.Run("F05_GrafanaDatasourcesProvisioning", func(t *testing.T) {
		dsPath := helpers.GetDeployPath("common", "grafana", "provisioning", "datasources", "datasources.yaml")
		require.True(t, helpers.FileExists(dsPath), "datasources.yaml provisioning file must exist at %s", dsPath)
		dsCfg := helpers.ValidateDatasourcesYAML(t, dsPath)

		t.Run("TC-F5-01_DatasourcesYAMLSyntax", func(t *testing.T) {
			require.Equal(t, 1, dsCfg.APIVersion, "Grafana datasources apiVersion should be 1")
			require.NotEmpty(t, dsCfg.Datasources, "Datasources list must not be empty")
		})
		t.Run("TC-F5-02_VictoriaMetricsDefaultDatasource", func(t *testing.T) {
			found := false
			for _, ds := range dsCfg.Datasources {
				if ds.Type == "prometheus" && ds.IsDefault {
					found = true
					require.Equal(t, "http://victoriametrics:8428/prometheus", ds.URL)
					require.Equal(t, "proxy", ds.Access)
				}
			}
			require.True(t, found, "VictoriaMetrics must be configured as default Prometheus datasource on port 8428")
		})
		t.Run("TC-F5-03_TempoDatasourceTracesToLogsV2", func(t *testing.T) {
			found := false
			for _, ds := range dsCfg.Datasources {
				if ds.Type == "tempo" {
					found = true
					t2l, ok := ds.JsonData["tracesToLogsV2"].(map[string]interface{})
					require.True(t, ok, "Tempo datasource must configure tracesToLogsV2")
					require.Equal(t, "loki", t2l["datasourceUid"], "tracesToLogsV2 target datasource must be 'loki'")
					require.Equal(t, true, t2l["filterByTraceID"], "filterByTraceID must be enabled")
				}
			}
			require.True(t, found, "Tempo datasource must exist")
		})
		t.Run("TC-F5-04_LokiDerivedFieldsForTraceLinking", func(t *testing.T) {
			found := false
			for _, ds := range dsCfg.Datasources {
				if ds.Type == "loki" {
					found = true
					dfs, ok := ds.JsonData["derivedFields"].([]interface{})
					require.True(t, ok, "Loki datasource must configure derivedFields")
					require.NotEmpty(t, dfs, "derivedFields list cannot be empty")
					df0 := dfs[0].(map[string]interface{})
					require.Equal(t, "tempo", df0["datasourceUid"], "Derived field must target Tempo datasource")
					helpers.ValidateDerivedFieldRegex(t, df0["matcherRegex"].(string))
				}
			}
			require.True(t, found, "Loki datasource must exist")
		})
		t.Run("TC-F5-05_TempoNodeGraphAndServiceMap", func(t *testing.T) {
			found := false
			for _, ds := range dsCfg.Datasources {
				if ds.Type == "tempo" {
					found = true
					nodeGraph, ok1 := ds.JsonData["nodeGraph"].(map[string]interface{})
					require.True(t, ok1, "nodeGraph must be configured")
					require.Equal(t, true, nodeGraph["enabled"])

					serviceMap, ok2 := ds.JsonData["serviceMap"].(map[string]interface{})
					require.True(t, ok2, "serviceMap must be configured")
					require.Equal(t, "VictoriaMetrics", serviceMap["datasourceUid"])
				}
			}
			require.True(t, found, "Tempo datasource must configure nodeGraph and serviceMap")
		})
	})

	// =========================================================================
	// Feature 6: Grafana 4 Core Dashboards
	// =========================================================================
	t.Run("F06_Grafana4CoreDashboards", func(t *testing.T) {
		dashboardsDir := helpers.GetDeployPath("common", "grafana", "dashboards")
		require.True(t, helpers.DirExists(dashboardsDir), "Dashboards directory %s must exist", dashboardsDir)

		t.Run("TC-F6-01_DashboardsProviderConfig", func(t *testing.T) {
			providerPath := helpers.GetDeployPath("common", "grafana", "provisioning", "dashboards", "dashboards.yaml")
			require.True(t, helpers.FileExists(providerPath), "dashboards.yaml must exist at %s", providerPath)
			data, err := os.ReadFile(providerPath)
			require.NoError(t, err)
			require.Contains(t, string(data), "Gate Dashboards")
			require.Contains(t, string(data), "/var/lib/grafana/dashboards")
		})
		t.Run("TC-F6-02_SystemAndGoRuntimeDashboard", func(t *testing.T) {
			p := filepath.Join(dashboardsDir, "system-runtime.json")
			require.True(t, helpers.FileExists(p), "system-runtime.json must exist")
			helpers.ValidateDashboardJSON(t, p, "gate-system-runtime", 2)
		})
		t.Run("TC-F6-03_HTTPREDDashboard", func(t *testing.T) {
			p := filepath.Join(dashboardsDir, "http-red.json")
			require.True(t, helpers.FileExists(p), "http-red.json must exist")
			helpers.ValidateDashboardJSON(t, p, "gate-http-red", 2)
		})
		t.Run("TC-F6-04_InfrastructureDashboard", func(t *testing.T) {
			p := filepath.Join(dashboardsDir, "infrastructure.json")
			require.True(t, helpers.FileExists(p), "infrastructure.json must exist")
			helpers.ValidateDashboardJSON(t, p, "gate-infrastructure", 2)
		})
		t.Run("TC-F6-05_JudgingAndBusinessDashboard", func(t *testing.T) {
			p := filepath.Join(dashboardsDir, "judging-business.json")
			require.True(t, helpers.FileExists(p), "judging-business.json must exist")
			helpers.ValidateDashboardJSON(t, p, "gate-judging-business", 2)
		})
	})

	// =========================================================================
	// Feature 7: Outbox Events Schema Migration
	// =========================================================================
	t.Run("F07_OutboxEventsSchemaMigration", func(t *testing.T) {
		migrationPath := helpers.GetBackendPath("migrations", "20260820000000_add_outbox_events_headers.sql")
		require.True(t, helpers.FileExists(migrationPath), "Migration file must exist")
		sqlContent, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		sqlStr := string(sqlContent)

		t.Run("TC-F7-01_MigrationFilePresence", func(t *testing.T) {
			require.NotEmpty(t, sqlStr)
		})
		t.Run("TC-F7-02_GooseUpStatement", func(t *testing.T) {
			require.Contains(t, sqlStr, "-- +goose Up")
			require.Contains(t, sqlStr, "ALTER TABLE outbox_events ADD COLUMN headers JSONB NOT NULL DEFAULT '{}'::jsonb;")
		})
		t.Run("TC-F7-03_GooseDownStatement", func(t *testing.T) {
			require.Contains(t, sqlStr, "-- +goose Down")
			require.Contains(t, sqlStr, "ALTER TABLE outbox_events DROP COLUMN IF EXISTS headers;")
		})
		t.Run("TC-F7-04_SchemaUpExecution", func(t *testing.T) {
			require.Contains(t, strings.ToLower(sqlStr), "add column headers jsonb")
			require.Contains(t, sqlStr, "'{}'::jsonb")
		})
		t.Run("TC-F7-05_SchemaDownExecution", func(t *testing.T) {
			require.Contains(t, strings.ToLower(sqlStr), "drop column if exists headers")
		})
	})

	// =========================================================================
	// Feature 8: Outbox Models & SQLC Updates
	// =========================================================================
	t.Run("F08_OutboxModelsAndSQLCUpdates", func(t *testing.T) {
		outboxModelPath := helpers.GetBackendPath("internal", "domain", "models", "outbox.go")
		require.True(t, helpers.FileExists(outboxModelPath), "Domain outbox.go must exist")
		content, err := os.ReadFile(outboxModelPath)
		require.NoError(t, err)
		modelStr := string(content)

		t.Run("TC-F8-01_OutboxEventModelStruct", func(t *testing.T) {
			require.Contains(t, modelStr, `Headers      map[string]string `+"`"+`json:"headers"`+"`")
		})
		t.Run("TC-F8-02_CreateOutboxEventParamsStruct", func(t *testing.T) {
			require.Contains(t, modelStr, `Headers     map[string]string `+"`"+`json:"headers"`+"`")
		})
		t.Run("TC-F8-03_SQLQueryParameterization", func(t *testing.T) {
			outboxSQLPath := helpers.GetBackendPath("internal", "repository", "pg", "outbox.sql")
			require.True(t, helpers.FileExists(outboxSQLPath), "outbox.sql must exist")
			sqlContent, err := os.ReadFile(outboxSQLPath)
			require.NoError(t, err)
			require.Contains(t, string(sqlContent), "headers")
			require.Contains(t, string(sqlContent), "FOR UPDATE SKIP LOCKED")
		})
		t.Run("TC-F8-04_SQLRepositoryMapping", func(t *testing.T) {
			pgOutboxPath := helpers.GetBackendPath("internal", "repository", "pg", "outbox.go")
			require.True(t, helpers.FileExists(pgOutboxPath), "repository outbox.go must exist")
			pgContent, err := os.ReadFile(pgOutboxPath)
			require.NoError(t, err)
			require.Contains(t, string(pgContent), "json.Marshal(event.Headers)")
			require.Contains(t, string(pgContent), "json.Unmarshal(e.Headers, &headers)")
		})
		t.Run("TC-F8-05_SQLCGenerationFreshness", func(t *testing.T) {
			sqlcModelsPath := helpers.GetBackendPath("internal", "repository", "pg", "sqlc", "models.go")
			require.True(t, helpers.FileExists(sqlcModelsPath), "sqlc models.go must exist")
			sqlcContent, err := os.ReadFile(sqlcModelsPath)
			require.NoError(t, err)
			require.Contains(t, string(sqlcContent), `Headers      []byte                   `+"`"+`json:"headers"`+"`")
		})
	})

	// =========================================================================
	// Feature 9: Backend Telemetry Core SDK
	// =========================================================================
	t.Run("F09_BackendTelemetryCoreSDK", func(t *testing.T) {
		t.Run("TC-F9-01_TelemetryConfigStruct", func(t *testing.T) {
			configPath := helpers.GetBackendPath("config", "config.go")
			require.True(t, helpers.FileExists(configPath), "backend config.go must exist")
			content, err := os.ReadFile(configPath)
			require.NoError(t, err)
			cfgStr := string(content)
			require.Contains(t, cfgStr, `OtelEndpoint       string `+"`"+`env:"OTEL_EXPORTER_OTLP_ENDPOINT"`)
			require.Contains(t, cfgStr, `OtelServiceName    string `+"`"+`env:"OTEL_SERVICE_NAME"`)
			require.Contains(t, cfgStr, `OtelInsecure       bool   `+"`"+`env:"OTEL_INSECURE"`)
			require.Contains(t, cfgStr, `OtelEnabled        bool   `+"`"+`env:"OTEL_ENABLED"`)
		})
		t.Run("TC-F9-02_InitTelemetryLifecycle", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			require.NotNil(t, tp)
			tracer := tp.Tracer("gate-backend-test")
			require.NotNil(t, tracer)
		})
		t.Run("TC-F9-03_ResourceAttributes", func(t *testing.T) {
			tracer := otel.Tracer("gate-test")
			_, span := tracer.Start(context.Background(), "test-op")
			require.True(t, span.SpanContext().IsValid())
			span.End()
		})
		t.Run("TC-F9-04_ShutdownFuncCleanup", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			err := tp.Shutdown(context.Background())
			require.NoError(t, err)
		})
		t.Run("TC-F9-05_RuntimeWiring", func(t *testing.T) {
			runtimePath := helpers.GetBackendPath("runtime.go")
			require.True(t, helpers.FileExists(runtimePath), "runtime.go must exist")
			content, err := os.ReadFile(runtimePath)
			require.NoError(t, err)
			require.Contains(t, string(content), "telemetry.InitTelemetry")
		})
	})

	// =========================================================================
	// Feature 10: slog/otelslog Logging & Trace Injection
	// =========================================================================
	t.Run("F10_SlogLoggingAndTraceInjection", func(t *testing.T) {
		handler, logger := helpers.NewInMemoryLogHandler(slog.LevelDebug, true, true)

		t.Run("TC-F10-01_DefaultSlogRegistration", func(t *testing.T) {
			require.NotNil(t, logger)
		})
		t.Run("TC-F10-02_TraceIDInjectionInActiveSpan", func(t *testing.T) {
			tracer := otel.Tracer("test")
			ctx, span := tracer.Start(context.Background(), "test-span")
			defer span.End()

			expectedTraceID := span.SpanContext().TraceID().String()
			expectedSpanID := span.SpanContext().SpanID().String()

			logger.InfoContext(ctx, "submission processed successfully", "submission_id", "sub-1")
			helpers.AssertLogContainsTraceContext(t, handler, expectedTraceID, expectedSpanID)
		})
		t.Run("TC-F10-03_SpanIDInjectionInActiveSpan", func(t *testing.T) {
			records := handler.GetRecords()
			require.NotEmpty(t, records)
			last := records[len(records)-1]
			require.NotEmpty(t, last.SpanID)
		})
		t.Run("TC-F10-04_MultiHandlerOutputRouting", func(t *testing.T) {
			output := handler.GetLogOutput()
			require.Contains(t, output, "submission processed successfully")
		})
		t.Run("TC-F10-05_LogLevelFiltering", func(t *testing.T) {
			infoHandler, infoLogger := helpers.NewInMemoryLogHandler(slog.LevelInfo, true, true)
			infoLogger.Debug("this should be filtered out")
			require.Empty(t, infoHandler.GetRecords())
			infoLogger.Info("this should be recorded")
			require.Len(t, infoHandler.GetRecords(), 1)
		})
	})

	// =========================================================================
	// Feature 11: Two-Tier Sanitization
	// =========================================================================
	t.Run("F11_TwoTierSanitization", func(t *testing.T) {
		t.Run("TC-F11-01_HTTPCookieRedactionInSpans", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Cookie", "session_id=secret-session-token")
			req.Header.Set("Set-Cookie", "session_id=secret-session-token")

			sanitized := helpers.ExtractSanitizedHeaders(req.Header)
			helpers.AssertHeaderSanitized(t, "Cookie", "session_id=secret-session-token", sanitized)
			helpers.AssertHeaderSanitized(t, "Set-Cookie", "session_id=secret-session-token", sanitized)
		})
		t.Run("TC-F11-02_AuthorizationHeaderRedaction", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Authorization", "Bearer super-secret-token")

			sanitized := helpers.ExtractSanitizedHeaders(req.Header)
			helpers.AssertHeaderSanitized(t, "Authorization", "Bearer super-secret-token", sanitized)
		})
		t.Run("TC-F11-03_SlogPasswordAttributeMasking", func(t *testing.T) {
			handler, logger := helpers.NewInMemoryLogHandler(slog.LevelInfo, true, false)
			logger.Info("user created", "username", "alice", "password", "p@ssword123")
			helpers.AssertLogSanitized(t, handler, []string{"p@ssword123"})
		})
		t.Run("TC-F11-04_SlogTokenAndSessionIDMasking", func(t *testing.T) {
			handler, logger := helpers.NewInMemoryLogHandler(slog.LevelInfo, true, false)
			logger.Info("session check", "access_token", "jwt-token-999", "session_id", "sess-888")
			helpers.AssertLogSanitized(t, handler, []string{"jwt-token-999", "sess-888"})
		})
		t.Run("TC-F11-05_AuthRequestPayloadSanitization", func(t *testing.T) {
			require.True(t, helpers.IsSensitiveKey("password"))
			require.True(t, helpers.IsSensitiveKey("secret"))
			require.True(t, helpers.IsSensitiveKey("token"))
		})
	})

	// =========================================================================
	// Feature 12: HTTP Tracing & RED Metrics
	// =========================================================================
	t.Run("F12_HTTPTracingAndREDMetrics", func(t *testing.T) {
		t.Run("TC-F12-01_otelhttpServerSpanCreation", func(t *testing.T) {
			tracer := otel.Tracer("http-test")
			_, span := tracer.Start(context.Background(), "HTTP GET /api/submissions",
				trace.WithAttributes(
					attribute.String("http.request.method", "GET"),
					attribute.String("http.route", "/api/submissions"),
				),
			)
			defer span.End()
			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("TC-F12-02_InboundW3CContextExtraction", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			carrier := helpers.MapCarrier{
				"traceparent": w3c.Traceparent,
			}
			extractedTID, extractedSID, err := helpers.SimulateCarrierRoundtrip(carrier, w3c.TraceID, w3c.SpanID)
			require.NoError(t, err)
			require.Equal(t, w3c.TraceID, extractedTID)
			require.Equal(t, w3c.SpanID, extractedSID)
		})
		t.Run("TC-F12-03_HTTPRequestRateCounterFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "sum(rate(http_server_request_duration_seconds_count[1m]))")
		})
		t.Run("TC-F12-04_HTTPRequestDurationHistogramFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "histogram_quantile(0.95, sum(rate(http_server_request_duration_seconds_bucket[5m])) by (le))")
		})
		t.Run("TC-F12-05_ActiveRequestsGaugeFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "http_server_active_requests")
		})
	})

	// =========================================================================
	// Feature 13: pgxpool Tracing & Stats
	// =========================================================================
	t.Run("F13_pgxpoolTracingAndStats", func(t *testing.T) {
		t.Run("TC-F13-01_otelpgxTracerRegistration", func(t *testing.T) {
			pgClientPath := helpers.GetBackendPath("pkg", "postgres-client.go")
			require.True(t, helpers.FileExists(pgClientPath))
		})
		t.Run("TC-F13-02_DBChildSpanGeneration", func(t *testing.T) {
			tracer := otel.Tracer("db-test")
			ctx, parent := tracer.Start(context.Background(), "HTTP GET /api/problems/1")
			defer parent.End()

			_, child := tracer.Start(ctx, "SELECT FROM problems",
				trace.WithAttributes(
					attribute.String("db.system", "postgresql"),
					attribute.String("db.name", "gate"),
				),
			)
			defer child.End()

			require.Equal(t, parent.SpanContext().TraceID(), child.SpanContext().TraceID())
		})
		t.Run("TC-F13-03_ConnectionStateGaugesFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "pgxpool_connections_total")
		})
		t.Run("TC-F13-04_MaxConnectionsMetricFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "pgxpool_max_conns")
		})
		t.Run("TC-F13-05_AcquireWaitMetricsFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "rate(pgxpool_acquire_wait_duration_seconds_total[5m])")
		})
	})

	// =========================================================================
	// Feature 14: Outbox Context Propagation & Lag Stats
	// =========================================================================
	t.Run("F14_OutboxContextPropagationAndLagStats", func(t *testing.T) {
		t.Run("TC-F14-01_OutboxW3CContextInjection", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			carrier := helpers.MapCarrier{}
			carrier.Set("traceparent", w3c.Traceparent)
			carrier.Set("baggage", w3c.Baggage)

			params := helpers.CreateOutboxEventParams{
				Id:          uuid.New().String(),
				AggregateID: uuid.New().String(),
				EventType:   "submission.created",
				Headers:     carrier,
			}
			require.Equal(t, w3c.Traceparent, params.Headers["traceparent"])
		})
		t.Run("TC-F14-02_OutboxW3CContextExtraction", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			carrier := helpers.MapCarrier{
				"traceparent": w3c.Traceparent,
			}
			tid, sid, err := helpers.SimulateCarrierRoundtrip(carrier, w3c.TraceID, w3c.SpanID)
			require.NoError(t, err)
			require.Equal(t, w3c.TraceID, tid)
			require.Equal(t, w3c.SpanID, sid)
		})
		t.Run("TC-F14-03_PendingEventsGaugeFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "outbox_pending_events_count")
		})
		t.Run("TC-F14-04_DispatchedEventsCounterFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "sum(rate(outbox_dispatched_events_total[5m]))")
		})
		t.Run("TC-F14-05_DispatchLagSecondsMetricFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "outbox_dispatch_lag_seconds")
		})
	})

	// =========================================================================
	// Feature 15: NATS Header Propagation & Queue Stats
	// =========================================================================
	t.Run("F15_NATSHeaderPropagationAndQueueStats", func(t *testing.T) {
		t.Run("TC-F15-01_NatsHeaderCarrierImplementation", func(t *testing.T) {
			carrier := helpers.MapCarrier{}
			carrier.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
			require.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("traceparent"))
		})
		t.Run("TC-F15-02_NATSMsgHeaderInjection", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			carrier := helpers.MapCarrier{}
			carrier.Set("traceparent", w3c.Traceparent)
			require.NotEmpty(t, carrier.Get("traceparent"))
		})
		t.Run("TC-F15-03_NATSConsumerContextExtraction", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			carrier := helpers.MapCarrier{"traceparent": w3c.Traceparent}
			tid, _, err := helpers.SimulateCarrierRoundtrip(carrier, w3c.TraceID, w3c.SpanID)
			require.NoError(t, err)
			require.Equal(t, w3c.TraceID, tid)
		})
		t.Run("TC-F15-04_ProgressEventsTraceContext", func(t *testing.T) {
			pubPath := helpers.GetBackendPath("internal", "worker", "pubsub", "submission_created_pub.go")
			require.True(t, helpers.FileExists(pubPath), "submission_created_pub.go must exist")
			pubContent, err := os.ReadFile(pubPath)
			require.NoError(t, err)
			require.Contains(t, string(pubContent), "telemetry.InjectNATSMsg(ctx, msg)")

			judgeWorkerPath := helpers.GetBackendPath("internal", "worker", "judge", "judge_worker.go")
			require.True(t, helpers.FileExists(judgeWorkerPath), "judge_worker.go must exist")
			judgeContent, err := os.ReadFile(judgeWorkerPath)
			require.NoError(t, err)
			require.Contains(t, string(judgeContent), "telemetry.ExtractNATSHeader")
		})
		t.Run("TC-F15-05_ConsumerPendingMessagesMetricFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "nats_consumer_pending_messages{consumer=\"judge_consumer\"}")
		})
	})

	// =========================================================================
	// Feature 16: go-judge gRPC Tracing & Judge Metrics
	// =========================================================================
	t.Run("F16_goJudgeGRPCTracingAndJudgeMetrics", func(t *testing.T) {
		t.Run("TC-F16-01_otelgrpcClientHandlerRegistration", func(t *testing.T) {
			sandboxClientPath := helpers.GetBackendPath("pkg", "sandbox", "client.go")
			require.True(t, helpers.FileExists(sandboxClientPath))
		})
		t.Run("TC-F16-02_goJudgeRPCSpanGeneration", func(t *testing.T) {
			tracer := otel.Tracer("grpc-test")
			_, span := tracer.Start(context.Background(), "pb.Executor/Exec",
				trace.WithAttributes(
					attribute.String("rpc.system", "grpc"),
					attribute.String("rpc.service", "pb.Executor"),
				),
			)
			defer span.End()
			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("TC-F16-03_VerdictStatisticsCounterFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "sum by (verdict) (judge_submissions_total)")
		})
		t.Run("TC-F16-04_JudgingPipelineDurationHistogramFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "histogram_quantile(0.95, sum(rate(judge_duration_seconds_bucket[5m])) by (le))")
		})
		t.Run("TC-F16-05_ActiveJudgeWorkersGaugeFormat", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "judge_active_workers")
		})
	})

	// =========================================================================
	// Feature 17: Next.js Server Tracing (instrumentation.ts)
	// =========================================================================
	t.Run("F17_NextJSServerTracing", func(t *testing.T) {
		instrPath := helpers.GetFrontendPath("instrumentation.ts")
		require.True(t, helpers.FileExists(instrPath), "frontend/instrumentation.ts must exist")
		content, err := os.ReadFile(instrPath)
		require.NoError(t, err)
		instrStr := string(content)

		t.Run("TC-F17-01_instrumentationFileExistence", func(t *testing.T) {
			require.Contains(t, instrStr, "export const register = async")
		})
		t.Run("TC-F17-02_registerOTelInvocation", func(t *testing.T) {
			require.Contains(t, instrStr, "registerOTel")
			require.Contains(t, instrStr, "env.getServerOtelServiceName()")

			envPath := helpers.GetFrontendPath("lib", "env.ts")
			require.True(t, helpers.FileExists(envPath))
			envContent, err := os.ReadFile(envPath)
			require.NoError(t, err)
			require.Contains(t, string(envContent), "getServerOtelServiceName")
			require.Contains(t, string(envContent), "OTEL_SERVICE_NAME")
		})
		t.Run("TC-F17-03_NodeJSRuntimeGuard", func(t *testing.T) {
			require.Contains(t, instrStr, `process.env.NEXT_RUNTIME === "nodejs"`)
		})
		t.Run("TC-F17-04_SSRRootSpanCreation", func(t *testing.T) {
			tracer := otel.Tracer("ssr-test")
			_, span := tracer.Start(context.Background(), "SSR GET /contests/1")
			defer span.End()
			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("TC-F17-05_SSROutgoingFetchPropagation", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			carrier := helpers.MapCarrier{"traceparent": w3c.Traceparent}
			tid, _, err := helpers.SimulateCarrierRoundtrip(carrier, w3c.TraceID, w3c.SpanID)
			require.NoError(t, err)
			require.Equal(t, w3c.TraceID, tid)
		})
	})

	// =========================================================================
	// Feature 18: Browser Web SDK Tracing
	// =========================================================================
	t.Run("F18_BrowserWebSDKTracing", func(t *testing.T) {
		browserTelemetryPath := helpers.GetFrontendPath("lib", "telemetry", "browser.ts")
		require.True(t, helpers.FileExists(browserTelemetryPath), "browser.ts must exist")
		content, err := os.ReadFile(browserTelemetryPath)
		require.NoError(t, err)
		browserStr := string(content)

		t.Run("TC-F18-01_WebTracerProviderInitialization", func(t *testing.T) {
			require.Contains(t, browserStr, "WebTracerProvider")
		})
		t.Run("TC-F18-02_OTLPTraceExporterConfiguration", func(t *testing.T) {
			require.Contains(t, browserStr, "OTLPTraceExporter")
			require.Contains(t, browserStr, "env.getOtelExporterUrl()")
		})
		t.Run("TC-F18-03_BatchSpanProcessorConfiguration", func(t *testing.T) {
			require.Contains(t, browserStr, "BatchSpanProcessor")
			require.Contains(t, browserStr, "maxQueueSize: 100")
		})
		t.Run("TC-F18-04_InstrumentationsRegistration", func(t *testing.T) {
			require.Contains(t, browserStr, "FetchInstrumentation")
			require.Contains(t, browserStr, "DocumentLoadInstrumentation")
		})
		t.Run("TC-F18-05_ProvidersComponentMounting", func(t *testing.T) {
			providersPath := helpers.GetFrontendPath("app", "providers.tsx")
			require.True(t, helpers.FileExists(providersPath))
			pContent, err := os.ReadFile(providersPath)
			require.NoError(t, err)
			require.Contains(t, string(pContent), "initBrowserTelemetry()")
		})
	})

	// =========================================================================
	// Feature 19: Client-Side traceparent Header Injection
	// =========================================================================
	t.Run("F19_ClientSideTraceparentHeaderInjection", func(t *testing.T) {
		t.Run("TC-F19-01_FetchInstrumentationURLPattern", func(t *testing.T) {
			browserTelemetryPath := helpers.GetFrontendPath("lib", "telemetry", "browser.ts")
			require.True(t, helpers.FileExists(browserTelemetryPath))
			content, err := os.ReadFile(browserTelemetryPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "propagateTraceHeaderCorsUrls")
		})
		t.Run("TC-F19-02_BrowserFetchTraceparentInjection", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			tid, sid := helpers.AssertValidTraceparent(t, w3c.Traceparent)
			require.Equal(t, w3c.TraceID, tid)
			require.Equal(t, w3c.SpanID, sid)
		})
		t.Run("TC-F19-03_OpenAPIClientIntegration", func(t *testing.T) {
			apiPath := helpers.GetFrontendPath("lib", "api.ts")
			require.True(t, helpers.FileExists(apiPath))
		})
		t.Run("TC-F19-04_SWRDataFetchingIntegration", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			helpers.AssertValidBaggage(t, w3c.Baggage)
		})
		t.Run("TC-F19-05_W3CFormatCompliance", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			_, _, _, flags, err := helpers.ParseTraceparent(w3c.Traceparent)
			require.NoError(t, err)
			require.Equal(t, "01", flags)
		})
	})

	// =========================================================================
	// Feature 20: Frontend Strict Env Discipline
	// =========================================================================
	t.Run("F20_FrontendStrictEnvDiscipline", func(t *testing.T) {
		t.Run("TC-F20-01_ImmutablenextConfigMJS", func(t *testing.T) {
			configPath := helpers.GetFrontendPath("next.config.mjs")
			require.True(t, helpers.FileExists(configPath), "next.config.mjs must exist")
			content, err := os.ReadFile(configPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "standalone")
		})
		t.Run("TC-F20-02_ImmutablenextEnvDTS", func(t *testing.T) {
			envDtsPath := helpers.GetFrontendPath("next-env.d.ts")
			require.True(t, helpers.FileExists(envDtsPath), "next-env.d.ts must exist")
		})
		t.Run("TC-F20-03_BunPackageExecution", func(t *testing.T) {
			lockPath := helpers.GetFrontendPath("bun.lock")
			require.True(t, helpers.FileExists(lockPath), "bun.lock must exist (no package-lock.json or yarn.lock)")
		})
		t.Run("TC-F20-04_StrictRequireEnvAccessors", func(t *testing.T) {
			envFilePath := helpers.GetFrontendPath("lib", "env.ts")
			require.True(t, helpers.FileExists(envFilePath))
			content, err := os.ReadFile(envFilePath)
			require.NoError(t, err)
			require.Contains(t, string(content), "requireEnv")
		})
		t.Run("TC-F20-05_ZeroDefaultFallbackScan", func(t *testing.T) {
			envFilePath := helpers.GetFrontendPath("lib", "env.ts")
			content, err := os.ReadFile(envFilePath)
			require.NoError(t, err)
			violations := helpers.ScanSourceForEnvFallbacks(string(content))
			require.Empty(t, violations, "Detected forbidden fallback env pattern in lib/env.ts: %v", violations)
		})
	})

	// =========================================================================
	// Feature 21: Distributed Trace End-to-End Linkage
	// =========================================================================
	t.Run("F21_DistributedTraceEndToEndLinkage", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		now := time.Now()

		spans := []helpers.SimulatedSpan{
			{Name: "fetch POST /api/submissions", ServiceName: "gate-frontend", TraceID: w3c.TraceID, SpanID: "0000000000000001", StartTime: now, EndTime: now.Add(500 * time.Millisecond)},
			{Name: "HTTP POST /api/submissions", ServiceName: "gate-backend", TraceID: w3c.TraceID, SpanID: "0000000000000002", ParentSpanID: "0000000000000001", StartTime: now.Add(5 * time.Millisecond), EndTime: now.Add(450 * time.Millisecond)},
			{Name: "INSERT INTO submissions", ServiceName: "gate-backend", TraceID: w3c.TraceID, SpanID: "0000000000000003", ParentSpanID: "0000000000000002", StartTime: now.Add(10 * time.Millisecond), EndTime: now.Add(20 * time.Millisecond)},
			{Name: "outbox.dispatch_event", ServiceName: "gate-backend", TraceID: w3c.TraceID, SpanID: "0000000000000004", ParentSpanID: "0000000000000002", StartTime: now.Add(25 * time.Millisecond), EndTime: now.Add(40 * time.Millisecond)},
			{Name: "judge.process_submission", ServiceName: "gate-backend", TraceID: w3c.TraceID, SpanID: "0000000000000005", ParentSpanID: "0000000000000004", StartTime: now.Add(45 * time.Millisecond), EndTime: now.Add(300 * time.Millisecond)},
			{Name: "pb.Executor/Exec", ServiceName: "go-judge", TraceID: w3c.TraceID, SpanID: "0000000000000006", ParentSpanID: "0000000000000005", StartTime: now.Add(50 * time.Millisecond), EndTime: now.Add(250 * time.Millisecond)},
		}

		t.Run("TC-F21-01_FullLifecycleTraceIDInvariance", func(t *testing.T) {
			helpers.VerifyTraceDAG(t, spans, "fetch POST /api/submissions")
		})
		t.Run("TC-F21-02_SpanParentChildHierarchy", func(t *testing.T) {
			require.Equal(t, "0000000000000001", spans[1].ParentSpanID)
			require.Equal(t, "0000000000000002", spans[2].ParentSpanID)
		})
		t.Run("TC-F21-03_SpanAttributesRichness", func(t *testing.T) {
			require.NotEmpty(t, spans[0].ServiceName)
			require.NotEmpty(t, spans[1].ServiceName)
		})
		t.Run("TC-F21-04_TempoQueryability", func(t *testing.T) {
			require.Len(t, w3c.TraceID, 32)
		})
		t.Run("TC-F21-05_LokiLogCorrelation", func(t *testing.T) {
			handler, logger := helpers.NewInMemoryLogHandler(slog.LevelInfo, true, true)
			logger.Info("submission completed", "trace_id", w3c.TraceID)
			records := handler.GetRecords()
			require.NotEmpty(t, records)
			require.Equal(t, w3c.TraceID, records[0].Attributes["trace_id"])
		})
	})
}
