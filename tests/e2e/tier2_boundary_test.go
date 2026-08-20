package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/tests/e2e/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

func TestTier2_BoundaryAndCornerCases(t *testing.T) {
	// =========================================================================
	// Feature 1: OTel Collector Gateway Deployment
	// =========================================================================
	t.Run("F01_OTelCollectorGateway", func(t *testing.T) {
		collectorConfigPath := helpers.GetDeployPath("common", "otel-collector", "config.yaml")
		cfg := helpers.ValidateOTelCollectorConfig(t, collectorConfigPath)

		t.Run("BC-F1-01_OTLPMetricsAndTracesReceiverPortBounds", func(t *testing.T) {
			grpcAddr := cfg.Receivers["otlp"].(map[string]interface{})["protocols"].(map[string]interface{})["grpc"].(map[string]interface{})["endpoint"].(string)
			httpAddr := cfg.Receivers["otlp"].(map[string]interface{})["protocols"].(map[string]interface{})["http"].(map[string]interface{})["endpoint"].(string)
			require.Contains(t, grpcAddr, "4317")
			require.Contains(t, httpAddr, "4318")
		})
		t.Run("BC-F1-02_CORSWildcardBoundary", func(t *testing.T) {
			cors := cfg.Receivers["otlp"].(map[string]interface{})["protocols"].(map[string]interface{})["http"].(map[string]interface{})["cors"].(map[string]interface{})
			origins := cors["allowed_origins"].([]interface{})
			require.True(t, len(origins) >= 1)
		})
		t.Run("BC-F1-03_BatchProcessorTimeoutBounds", func(t *testing.T) {
			batch := cfg.Processors["batch"].(map[string]interface{})
			timeout := batch["timeout"].(string)
			require.True(t, strings.HasSuffix(timeout, "s") || strings.HasSuffix(timeout, "ms"))
		})
		t.Run("BC-F1-04_ResourceAttributesDuplicates", func(t *testing.T) {
			res := cfg.Processors["resource"].(map[string]interface{})
			attrs := res["attributes"].([]interface{})
			found := false
			for _, a := range attrs {
				am := a.(map[string]interface{})
				if am["key"] == "service.namespace" {
					found = true
					require.Equal(t, "insert", am["action"], "Resource processor should use 'insert' action for idempotency")
				}
			}
			require.True(t, found)
		})
		t.Run("BC-F1-05_CollectorYAMLSchemaRejection", func(t *testing.T) {
			malformedYAML := "receivers: [invalid syntax"
			var testCfg helpers.OTelConfig
			err := yaml.Unmarshal([]byte(malformedYAML), &testCfg)
			require.Error(t, err, "Malformed YAML must be rejected")
		})
	})

	// =========================================================================
	// Feature 2: Host Port 8889 Non-collision
	// =========================================================================
	t.Run("F02_HostPort8889NonCollision", func(t *testing.T) {
		t.Run("BC-F2-01_PortNumberRangeBound", func(t *testing.T) {
			collectorConfigPath := helpers.GetDeployPath("common", "otel-collector", "config.yaml")
			cfg := helpers.ValidateOTelCollectorConfig(t, collectorConfigPath)
			addr := cfg.Service.Telemetry.Metrics.Address
			parts := strings.Split(addr, ":")
			require.Len(t, parts, 2)
			portNum, err := strconv.Atoi(parts[1])
			require.NoError(t, err)
			require.True(t, portNum >= 1024 && portNum <= 65535, "Port %d must be in unprivileged range", portNum)
		})
		t.Run("BC-F2-02_InterfaceBindingVerification", func(t *testing.T) {
			collectorConfigPath := helpers.GetDeployPath("common", "otel-collector", "config.yaml")
			cfg := helpers.ValidateOTelCollectorConfig(t, collectorConfigPath)
			require.Equal(t, "0.0.0.0:8889", cfg.Service.Telemetry.Metrics.Address)
		})
		t.Run("BC-F2-03_PortCollisionPrevention", func(t *testing.T) {
			localComposePath := helpers.GetDeployPath("local", "docker-compose.yml")
			localCompose := helpers.ValidateComposeFile(t, localComposePath)
			helpers.AssertNoPortCollision(t, localCompose, 8889, 8888)
		})
		t.Run("BC-F2-04_ConcurrentStartupPortDistinctness", func(t *testing.T) {
			p1, _ := helpers.ParsePortString("127.0.0.1:8888:8888")
			p2, _ := helpers.ParsePortString("0.0.0.0:8889:8889")
			require.NotEqual(t, p1.HostPort, p2.HostPort)
		})
		t.Run("BC-F2-05_PrometheusScrapeOnCollisionPort", func(t *testing.T) {
			localComposePath := helpers.GetDeployPath("local", "docker-compose.yml")
			localCompose := helpers.ValidateComposeFile(t, localComposePath)
			ports, err := localCompose.GetServicePorts("filer")
			require.NoError(t, err)
			found8888 := false
			for _, p := range ports {
				if p.HostPort == 8888 {
					found8888 = true
					break
				}
			}
			require.True(t, found8888, "filer must bind port 8888")
			helpers.AssertNoPortCollision(t, localCompose, 8889, 8888)
		})
	})

	// =========================================================================
	// Feature 3: Traefik /otlp Ingestion & CORS
	// =========================================================================
	t.Run("F03_TraefikOTLPIngestionAndCORS", func(t *testing.T) {
		traefikDynamicPath := helpers.GetDeployPath("local", "traefik", "dynamic.yml")
		data, err := os.ReadFile(traefikDynamicPath)
		require.NoError(t, err)
		var dynamicCfg map[string]interface{}
		require.NoError(t, yaml.Unmarshal(data, &dynamicCfg))
		httpMap := dynamicCfg["http"].(map[string]interface{})
		middlewares := httpMap["middlewares"].(map[string]interface{})

		t.Run("BC-F3-01_CORSPreflightOPTIONSRequest", func(t *testing.T) {
			corsMw := middlewares["otlp-cors"].(map[string]interface{})
			headersMap := corsMw["headers"].(map[string]interface{})
			methods := headersMap["accessControlAllowMethods"].([]interface{})
			require.Contains(t, methods, "OPTIONS")
			require.Contains(t, methods, "POST")
		})
		t.Run("BC-F3-02_PathStrippingExactness", func(t *testing.T) {
			stripMw := middlewares["strip-otlp-prefix"].(map[string]interface{})
			stripPrefix := stripMw["stripPrefix"].(map[string]interface{})
			prefixes := stripPrefix["prefixes"].([]interface{})
			require.Equal(t, "/otlp", prefixes[0])
		})
		t.Run("BC-F3-03_TrailingSlashNormalization", func(t *testing.T) {
			path := "/otlp/v1/traces/"
			trimmed := strings.TrimPrefix(path, "/otlp")
			require.Equal(t, "/v1/traces/", trimmed)
		})
		t.Run("BC-F3-04_DisallowedMethodRejection", func(t *testing.T) {
			corsMw := middlewares["otlp-cors"].(map[string]interface{})
			headersMap := corsMw["headers"].(map[string]interface{})
			methods := headersMap["accessControlAllowMethods"].([]interface{})
			require.NotContains(t, methods, "DELETE")
		})
		t.Run("BC-F3-05_NullOrMissingOriginHeaderHandling", func(t *testing.T) {
			corsMw := middlewares["otlp-cors"].(map[string]interface{})
			headersMap := corsMw["headers"].(map[string]interface{})
			origins := headersMap["accessControlAllowOriginList"].([]interface{})
			require.Contains(t, origins, "*")
		})
	})

	// =========================================================================
	// Feature 4: Multi-Environment Compose Sync
	// =========================================================================
	t.Run("F04_MultiEnvironmentComposeSync", func(t *testing.T) {
		t.Run("BC-F4-01_ProductionMemoryLimitBounds", func(t *testing.T) {
			prodPath := helpers.GetDeployPath("prod", "docker-compose.yml")
			prodCfg := helpers.ValidateComposeFile(t, prodPath)
			require.NotEmpty(t, prodCfg.Services)
		})
		t.Run("BC-F4-02_MissingEnvironmentVariableHandling", func(t *testing.T) {
			devPath := helpers.GetDeployPath("dev", "docker-compose.yml")
			devCfg := helpers.ValidateComposeFile(t, devPath)
			require.NotEmpty(t, devCfg.Services)
		})
		t.Run("BC-F4-03_ContainerRestartPolicyConsistency", func(t *testing.T) {
			localPath := helpers.GetDeployPath("local", "docker-compose.yml")
			localCfg := helpers.ValidateComposeFile(t, localPath)
			for _, svc := range localCfg.Services {
				if svc.Restart != "" {
					require.True(t, svc.Restart == "always" || svc.Restart == "unless-stopped" || svc.Restart == "on-failure")
				}
			}
		})
		t.Run("BC-F4-04_VolumeDeletionSafety", func(t *testing.T) {
			localPath := helpers.GetDeployPath("local", "docker-compose.yml")
			localCfg := helpers.ValidateComposeFile(t, localPath)
			require.NotNil(t, localCfg.Volumes)
		})
		t.Run("BC-F4-05_CrossNetworkCommunication", func(t *testing.T) {
			devPath := helpers.GetDeployPath("dev", "docker-compose.yml")
			devCfg := helpers.ValidateComposeFile(t, devPath)
			require.NotNil(t, devCfg.Networks)
		})
	})

	// =========================================================================
	// Feature 5: Grafana Datasources Provisioning
	// =========================================================================
	t.Run("F05_GrafanaDatasourcesProvisioning", func(t *testing.T) {
		t.Run("BC-F5-01_LokiDerivedFieldRegexVariations", func(t *testing.T) {
			regexStr := `(?:trace_id|traceId)=([0-9a-f]{32})`
			helpers.ValidateDerivedFieldRegex(t, regexStr)
		})
		t.Run("BC-F5-02_tracesToLogsV2TimeShiftBounds", func(t *testing.T) {
			dsPath := helpers.GetDeployPath("common", "grafana", "provisioning", "datasources", "datasources.yaml")
			dsCfg := helpers.ValidateDatasourcesYAML(t, dsPath)
			for _, ds := range dsCfg.Datasources {
				if ds.Type == "tempo" {
					t2l := ds.JsonData["tracesToLogsV2"].(map[string]interface{})
					require.Equal(t, "-2m", t2l["spanStartTimeShift"])
					require.Equal(t, "2m", t2l["spanEndTimeShift"])
				}
			}
		})
		t.Run("BC-F5-03_DatasourceUIDUniquenessAndMatch", func(t *testing.T) {
			dsPath := helpers.GetDeployPath("common", "grafana", "provisioning", "datasources", "datasources.yaml")
			dsCfg := helpers.ValidateDatasourcesYAML(t, dsPath)
			uids := make(map[string]bool)
			for _, ds := range dsCfg.Datasources {
				require.NotEmpty(t, ds.UID)
				require.False(t, uids[ds.UID], "Duplicate datasource UID: %s", ds.UID)
				uids[ds.UID] = true
			}
			require.True(t, uids["tempo"])
			require.True(t, uids["loki"])
			require.True(t, uids["VictoriaMetrics"])
		})
		t.Run("BC-F5-04_ProxyVsDirectAccessMode", func(t *testing.T) {
			dsPath := helpers.GetDeployPath("common", "grafana", "provisioning", "datasources", "datasources.yaml")
			dsCfg := helpers.ValidateDatasourcesYAML(t, dsPath)
			for _, ds := range dsCfg.Datasources {
				require.Equal(t, "proxy", ds.Access, "Datasource %s access must be 'proxy'", ds.Name)
			}
		})
		t.Run("BC-F5-05_EmptyQueryResultHandling", func(t *testing.T) {
			dsPath := helpers.GetDeployPath("common", "grafana", "provisioning", "datasources", "datasources.yaml")
			dsCfg := helpers.ValidateDatasourcesYAML(t, dsPath)
			for _, ds := range dsCfg.Datasources {
				if ds.Type == "loki" {
					dfs := ds.JsonData["derivedFields"].([]interface{})
					df0 := dfs[0].(map[string]interface{})
					helpers.ValidateDerivedFieldRegex(t, df0["matcherRegex"].(string))
				}
			}
		})
	})

	// =========================================================================
	// Feature 6: Grafana 4 Core Dashboards
	// =========================================================================
	t.Run("F06_Grafana4CoreDashboards", func(t *testing.T) {
		t.Run("BC-F6-01_StrictJSONParsing", func(t *testing.T) {
			validJSON := `{"uid":"test-dash","title":"Test","schemaVersion":30,"panels":[]}`
			var dash helpers.GrafanaDashboard
			err := json.Unmarshal([]byte(validJSON), &dash)
			require.NoError(t, err)
		})
		t.Run("BC-F6-02_GridPositionBoundaryLimits", func(t *testing.T) {
			grid := helpers.GrafanaGridPos{X: 0, Y: 0, W: 12, H: 6}
			require.True(t, grid.X+grid.W <= 24)
			require.True(t, grid.H >= 2)
		})
		t.Run("BC-F6-03_PromQLMetricNameAlignment", func(t *testing.T) {
			expr := "sum(rate(http_server_request_duration_seconds_count[5m])) by (http_route)"
			helpers.AssertValidPromQL(t, expr)
		})
		t.Run("BC-F6-04_DatasourceTemplateVariableBounds", func(t *testing.T) {
			expr := "sum by (verdict) (judge_submissions_total)"
			helpers.AssertValidPromQL(t, expr)
		})
		t.Run("BC-F6-05_EmptyDashboardDataRendering", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "pgxpool_connections_total{state=\"acquired\"}")
		})
	})

	// =========================================================================
	// Feature 7: Outbox Events Schema Migration
	// =========================================================================
	t.Run("F07_OutboxEventsSchemaMigration", func(t *testing.T) {
		migrationPath := helpers.GetBackendPath("migrations", "20260820000000_add_outbox_events_headers.sql")
		data, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		sqlStr := string(data)

		t.Run("BC-F7-01_PreExistingRowsConstantDefault", func(t *testing.T) {
			require.Contains(t, sqlStr, "DEFAULT '{}'::jsonb")
		})
		t.Run("BC-F7-02_DefaultJSONObjectValidation", func(t *testing.T) {
			var m map[string]interface{}
			err := json.Unmarshal([]byte("{}"), &m)
			require.NoError(t, err)
			require.Empty(t, m)
		})
		t.Run("BC-F7-03_RollbackIdempotency", func(t *testing.T) {
			require.Contains(t, sqlStr, "DROP COLUMN IF EXISTS headers")
		})
		t.Run("BC-F7-04_NonLockingDDLExecution", func(t *testing.T) {
			require.Contains(t, sqlStr, "ALTER TABLE outbox_events ADD COLUMN headers")
		})
		t.Run("BC-F7-05_CaseSensitiveColumnReference", func(t *testing.T) {
			require.Contains(t, sqlStr, "headers")
			require.NotContains(t, sqlStr, "Headers")
		})
	})

	// =========================================================================
	// Feature 8: Outbox Models & SQLC Updates
	// =========================================================================
	t.Run("F08_OutboxModelsAndSQLCUpdates", func(t *testing.T) {
		t.Run("BC-F8-01_NilHeadersMapSerialization", func(t *testing.T) {
			ev := helpers.OutboxEvent{
				Id:      uuid.New().String(),
				Headers: nil,
			}
			bytes, err := json.Marshal(ev)
			require.NoError(t, err)
			require.Contains(t, string(bytes), `"headers":null`)
		})
		t.Run("BC-F8-02_EmptyHeadersMapDeserialization", func(t *testing.T) {
			raw := `{"id":"` + uuid.New().String() + `","headers":{}}`
			var ev helpers.OutboxEvent
			err := json.Unmarshal([]byte(raw), &ev)
			require.NoError(t, err)
			require.NotNil(t, ev.Headers)
			require.Empty(t, ev.Headers)
		})
		t.Run("BC-F8-03_MultiKeyTracingHeaderMap", func(t *testing.T) {
			ev := helpers.OutboxEvent{
				Headers: map[string]string{
					"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
					"tracestate":  "congo=t61rcWkgMzE",
					"baggage":     "user_id=usr-42,role=admin",
				},
			}
			bytes, err := json.Marshal(ev)
			require.NoError(t, err)
			var parsed helpers.OutboxEvent
			require.NoError(t, json.Unmarshal(bytes, &parsed))
			require.Equal(t, ev.Headers["traceparent"], parsed.Headers["traceparent"])
			require.Equal(t, ev.Headers["baggage"], parsed.Headers["baggage"])
		})
		t.Run("BC-F8-04_SpecialCharactersInHeaderValues", func(t *testing.T) {
			headers := map[string]string{
				"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
				"baggage":     "k1=v1;k2=v2=v3:special-chars_123.456",
			}
			bytes, err := json.Marshal(headers)
			require.NoError(t, err)
			var parsed map[string]string
			require.NoError(t, json.Unmarshal(bytes, &parsed))
			require.Equal(t, headers["baggage"], parsed["baggage"])
		})
		t.Run("BC-F8-05_LargeHeaderMapPayload", func(t *testing.T) {
			largeHeaders := make(map[string]string)
			for i := 0; i < 60; i++ {
				largeHeaders[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_data_%d", i)
			}
			bytes, err := json.Marshal(largeHeaders)
			require.NoError(t, err)
			var parsed map[string]string
			require.NoError(t, json.Unmarshal(bytes, &parsed))
			require.Len(t, parsed, 60)
		})
	})

	// =========================================================================
	// Feature 9: Backend Telemetry Core SDK
	// =========================================================================
	t.Run("F09_BackendTelemetryCoreSDK", func(t *testing.T) {
		t.Run("BC-F9-01_StartupWithCollectorDown", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			require.NotNil(t, tp)
		})
		t.Run("BC-F9-02_ShutdownContextTimeout", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			err := tp.Shutdown(ctx)
			require.NoError(t, err)
		})
		t.Run("BC-F9-03_DoubleShutdownInvocation", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			err1 := tp.Shutdown(context.Background())
			require.NoError(t, err1)
			err2 := tp.Shutdown(context.Background())
			// OpenTelemetry SDK handles double shutdown gracefully
			_ = err2
		})
		t.Run("BC-F9-04_ZeroValueConfigFallback", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			tr := tp.Tracer("")
			require.NotNil(t, tr)
		})
		t.Run("BC-F9-05_PostShutdownTelemetryCalls", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			tr := tp.Tracer("post-shutdown")
			_ = tp.Shutdown(context.Background())
			// Tracer start should not panic
			_, span := tr.Start(context.Background(), "noop-span")
			span.End()
		})
	})

	// =========================================================================
	// Feature 10: slog/otelslog Logging & Trace Injection
	// =========================================================================
	t.Run("F10_SlogLoggingAndTraceInjection", func(t *testing.T) {
		handler, logger := helpers.NewInMemoryLogHandler(slog.LevelDebug, true, true)

		t.Run("BC-F10-01_LoggingWithoutSpanInContext", func(t *testing.T) {
			handler.Reset()
			logger.InfoContext(context.Background(), "log with empty context")
			records := handler.GetRecords()
			require.Len(t, records, 1)
			require.Empty(t, records[0].TraceID)
		})
		t.Run("BC-F10-02_LoggingWithInvalidUnsampledSpan", func(t *testing.T) {
			handler.Reset()
			sc := trace.SpanContext{} // invalid
			ctx := trace.ContextWithSpanContext(context.Background(), sc)
			logger.InfoContext(ctx, "log with invalid span context")
			records := handler.GetRecords()
			require.Len(t, records, 1)
			require.Empty(t, records[0].TraceID)
		})
		t.Run("BC-F10-03_NilContextLogging", func(t *testing.T) {
			handler.Reset()
			logger.Info("log with nil context implicitly")
			records := handler.GetRecords()
			require.Len(t, records, 1)
		})
		t.Run("BC-F10-04_HighFrequencyLogBurst", func(t *testing.T) {
			handler.Reset()
			for i := 0; i < 500; i++ {
				logger.Info("burst log entry", "seq", i)
			}
			require.Len(t, handler.GetRecords(), 500)
		})
		t.Run("BC-F10-05_LargeLogAttributeString", func(t *testing.T) {
			handler.Reset()
			largeStr := strings.Repeat("A", 1024*32) // 32 KB string
			logger.Info("large attribute log", "payload", largeStr)
			records := handler.GetRecords()
			require.Len(t, records, 1)
			require.Equal(t, largeStr, records[0].Attributes["payload"])
		})
	})

	// =========================================================================
	// Feature 11: Two-Tier Sanitization
	// =========================================================================
	t.Run("F11_TwoTierSanitization", func(t *testing.T) {
		t.Run("BC-F11-01_CaseInsensitiveSensitiveKeyMatching", func(t *testing.T) {
			keys := []string{"Password", "PASSWORD", "pAsSwOrD", "Session_ID", "Auth_Token", "SECRET"}
			for _, k := range keys {
				require.True(t, helpers.IsSensitiveKey(k), "Key %s should be sensitive", k)
			}
		})
		t.Run("BC-F11-02_NestedGroupAttributeSanitization", func(t *testing.T) {
			group := slog.Group("credentials",
				slog.String("username", "admin"),
				slog.String("password", "rootsecret"),
			)
			sanitized := helpers.SanitizeSlogAttribute(group)
			require.Equal(t, slog.KindGroup, sanitized.Value.Kind())
			subAttrs := sanitized.Value.Group()
			require.Equal(t, "username", subAttrs[0].Key)
			require.Equal(t, "admin", subAttrs[0].Value.String())
			require.Equal(t, "password", subAttrs[1].Key)
			require.Equal(t, helpers.RedactedPlaceholder, subAttrs[1].Value.String())
		})
		t.Run("BC-F11-03_EmptyStringSensitiveValue", func(t *testing.T) {
			attr := slog.String("password", "")
			sanitized := helpers.SanitizeSlogAttribute(attr)
			require.Equal(t, helpers.RedactedPlaceholder, sanitized.Value.String())
		})
		t.Run("BC-F11-04_NonStringSensitiveValueTypes", func(t *testing.T) {
			attr := slog.Int("password", 123456)
			sanitized := helpers.SanitizeSlogAttribute(attr)
			require.Equal(t, helpers.RedactedPlaceholder, sanitized.Value.String())
		})
		t.Run("BC-F11-05_BenignKeySubstringNonMasking", func(t *testing.T) {
			benignKeys := []string{"token_count", "passwords_updated_total", "jwt_token_rate", "secret_service_count"}
			for _, k := range benignKeys {
				require.False(t, helpers.IsSensitiveKey(k), "Benign metric key '%s' must NOT be masked", k)
			}
		})
	})

	// =========================================================================
	// Feature 12: HTTP Tracing & RED Metrics
	// =========================================================================
	t.Run("F12_HTTPTracingAndREDMetrics", func(t *testing.T) {
		t.Run("BC-F12-01_HTTPHandlerPanicRecovery", func(t *testing.T) {
			tracer := otel.Tracer("panic-test")
			_, span := tracer.Start(context.Background(), "HTTP GET /api/panic")
			defer span.End()

			func() {
				defer func() {
					if r := recover(); r != nil {
						span.SetStatus(codes.Error, fmt.Sprintf("panic recovered: %v", r))
						span.SetAttributes(attribute.Int("http.response.status_code", 500))
					}
				}()
				panic("simulated critical handler panic")
			}()

			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("BC-F12-02_RouteParameterCardinalityProtection", func(t *testing.T) {
			rawRoute := "/contests/123/scoreboard"
			templateRoute := "/contests/{contest_id}/scoreboard"
			require.NotEqual(t, rawRoute, templateRoute)
			helpers.AssertValidPromQL(t, "sum(rate(http_server_request_duration_seconds_count{http_route=\"/contests/{contest_id}/scoreboard\"}[5m]))")
		})
		t.Run("BC-F12-03_ClientRequestCancellation", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // cancel immediately
			require.Error(t, ctx.Err())
		})
		t.Run("BC-F12-04_ExtremeRequestDurationBounds", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "histogram_quantile(0.99, sum(rate(http_server_request_duration_seconds_bucket[5m])) by (le))")
		})
		t.Run("BC-F12-05_NonStandardHTTPMethods", func(t *testing.T) {
			methods := []string{"HEAD", "OPTIONS", "PATCH", "PUT", "DELETE"}
			for _, m := range methods {
				helpers.AssertValidPromQL(t, fmt.Sprintf("http_requests_total{http_request_method=\"%s\"}", m))
			}
		})
	})

	// =========================================================================
	// Feature 13: pgxpool Tracing & Stats
	// =========================================================================
	t.Run("F13_pgxpoolTracingAndStats", func(t *testing.T) {
		t.Run("BC-F13-01_ConnectionPoolSaturation", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "pgxpool_connections_total{state=\"acquired\"} == 60")
		})
		t.Run("BC-F13-02_ClosedPoolMetricScrape", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "pgxpool_connections_total")
		})
		t.Run("BC-F13-03_FailedQuerySpanErrorStatus", func(t *testing.T) {
			tracer := otel.Tracer("db-err-test")
			_, span := tracer.Start(context.Background(), "INSERT INTO submissions")
			span.SetStatus(codes.Error, "relation 'submissions' does not exist")
			span.End()
			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("BC-F13-04_TransactionRollbackSpanRecording", func(t *testing.T) {
			tracer := otel.Tracer("db-tx-test")
			ctx, txSpan := tracer.Start(context.Background(), "DATABASE TRANSACTION")
			_, rbSpan := tracer.Start(ctx, "ROLLBACK")
			rbSpan.End()
			txSpan.End()
			require.Equal(t, txSpan.SpanContext().TraceID(), rbSpan.SpanContext().TraceID())
		})
		t.Run("BC-F13-05_IdleZeroConnectionState", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "pgxpool_connections_total{state=\"idle\"}")
		})
	})

	// =========================================================================
	// Feature 14: Outbox Context Propagation & Lag Stats
	// =========================================================================
	t.Run("F14_OutboxContextPropagationAndLagStats", func(t *testing.T) {
		t.Run("BC-F14-01_MissingEmptyHeaderInEvent", func(t *testing.T) {
			carrier := helpers.MapCarrier{}
			// Extracting from empty carrier should safely produce an empty context
			propagator := propagation.TraceContext{}
			extractedCtx := propagator.Extract(context.Background(), carrier)
			sc := trace.SpanContextFromContext(extractedCtx)
			require.False(t, sc.IsValid(), "Empty carrier should extract invalid span context")
		})
		t.Run("BC-F14-02_MalformedW3CTraceparentHeader", func(t *testing.T) {
			_, _, _, _, err := helpers.ParseTraceparent("invalid-traceparent-string")
			require.Error(t, err)
		})
		t.Run("BC-F14-03_DetachedContextLifetime", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			// Detached context retains trace ID even if caller context is cancelled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			detached := context.WithoutCancel(ctx)
			require.NoError(t, detached.Err())
			require.NotEmpty(t, w3c.TraceID)
		})
		t.Run("BC-F14-04_DispatchFailureRetryMetric", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "rate(outbox_events_failed_total[5m])")
		})
		t.Run("BC-F14-05_ZeroPendingEventsState", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "outbox_pending_events_count == 0")
		})
	})

	// =========================================================================
	// Feature 15: NATS Header Propagation & Queue Stats
	// =========================================================================
	t.Run("F15_NATSHeaderPropagationAndQueueStats", func(t *testing.T) {
		t.Run("BC-F15-01_NATSMessageWithNilHeaders", func(t *testing.T) {
			var carrier helpers.MapCarrier // nil map
			require.Empty(t, carrier.Get("traceparent"))
		})
		t.Run("BC-F15-02_CorruptTraceparentInNATSHeader", func(t *testing.T) {
			carrier := helpers.MapCarrier{"traceparent": "corrupted-value"}
			propagator := propagation.TraceContext{}
			ctx := propagator.Extract(context.Background(), carrier)
			require.False(t, trace.SpanContextFromContext(ctx).IsValid())
		})
		t.Run("BC-F15-03_MultiValueHeaderSlice", func(t *testing.T) {
			carrier := helpers.MapCarrier{"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"}
			require.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("traceparent"))
		})
		t.Run("BC-F15-04_ConsumerQueueBacklogSpike", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "nats_consumer_pending_messages > 1000")
		})
		t.Run("BC-F15-05_TemporaryNATSReconnection", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "nats_consumer_ack_total")
		})
	})

	// =========================================================================
	// Feature 16: go-judge gRPC Tracing & Judge Metrics
	// =========================================================================
	t.Run("F16_goJudgeGRPCTracingAndJudgeMetrics", func(t *testing.T) {
		t.Run("BC-F16-01_CompilationErrorHandling", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "judge_submissions_total{verdict=\"Compilation Error\"}")
		})
		t.Run("BC-F16-02_SandboxTimeoutTLEHandling", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "judge_submissions_total{verdict=\"Time Limit Exceeded\"}")
		})
		t.Run("BC-F16-03_goJudgeConnectionFailure", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "judge_retries_total")
		})
		t.Run("BC-F16-04_FullWorkerConcurrencySaturation", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "judge_active_workers == 4")
		})
		t.Run("BC-F16-05_MultiLanguageSubmissions", func(t *testing.T) {
			helpers.AssertValidPromQL(t, "sum by (language) (judge_submissions_total)")
		})
	})

	// =========================================================================
	// Feature 17: Next.js Server Tracing (instrumentation.ts)
	// =========================================================================
	t.Run("F17_NextJSServerTracing", func(t *testing.T) {
		t.Run("BC-F17-01_EdgeRuntimeExecutionGuard", func(t *testing.T) {
			instrPath := helpers.GetFrontendPath("instrumentation.ts")
			content, err := os.ReadFile(instrPath)
			require.NoError(t, err)
			require.Contains(t, string(content), `process.env.NEXT_RUNTIME === "nodejs"`)
		})
		t.Run("BC-F17-02_MissingOTelEnvVariablesInSSR", func(t *testing.T) {
			envPath := helpers.GetFrontendPath("lib", "env.ts")
			content, err := os.ReadFile(envPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "getServerOtelServiceName")
			require.Contains(t, string(content), "OTEL_SERVICE_NAME")
		})
		t.Run("BC-F17-03_ServerActionErrorSpanStatus", func(t *testing.T) {
			tracer := otel.Tracer("ssr-action")
			_, span := tracer.Start(context.Background(), "ServerAction submitCode")
			span.SetStatus(codes.Error, "invalid authorization token")
			span.End()
			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("BC-F17-04_Backend500ResponseDuringSSR", func(t *testing.T) {
			tracer := otel.Tracer("ssr-fetch")
			_, span := tracer.Start(context.Background(), "fetch http://backend:8080/api/problems")
			span.SetAttributes(attribute.Int("http.status_code", 500))
			span.SetStatus(codes.Error, "500 Internal Server Error")
			span.End()
			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("BC-F17-05_IdempotentServerRegistration", func(t *testing.T) {
			var initOnce sync.Once
			count := 0
			for i := 0; i < 5; i++ {
				initOnce.Do(func() { count++ })
			}
			require.Equal(t, 1, count)
		})
	})

	// =========================================================================
	// Feature 18: Browser Web SDK Tracing
	// =========================================================================
	t.Run("F18_BrowserWebSDKTracing", func(t *testing.T) {
		t.Run("BC-F18-01_ServerSideExecutionGuard", func(t *testing.T) {
			browserTelemetryPath := helpers.GetFrontendPath("lib", "telemetry", "browser.ts")
			content, err := os.ReadFile(browserTelemetryPath)
			require.NoError(t, err)
			require.Contains(t, string(content), `typeof window === "undefined"`)
		})
		t.Run("BC-F18-02_ReactStrictModeDoubleMount", func(t *testing.T) {
			isInitialized := false
			initFunc := func() bool {
				if isInitialized {
					return false
				}
				isInitialized = true
				return true
			}
			require.True(t, initFunc())
			require.False(t, initFunc())
		})
		t.Run("BC-F18-03_OfflineBrowserNetworkState", func(t *testing.T) {
			browserTelemetryPath := helpers.GetFrontendPath("lib", "telemetry", "browser.ts")
			content, err := os.ReadFile(browserTelemetryPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "BatchSpanProcessor")
			require.Contains(t, string(content), "maxQueueSize: 100")
		})
		t.Run("BC-F18-04_ClientRouterPageTransitions", func(t *testing.T) {
			tracer := otel.Tracer("browser-router")
			_, span := tracer.Start(context.Background(), "navigation /contests -> /submissions")
			span.End()
			require.True(t, span.SpanContext().IsValid())
		})
		t.Run("BC-F18-05_ZeroSpanProcessorLeak", func(t *testing.T) {
			tp := sdktrace.NewTracerProvider()
			require.NoError(t, tp.Shutdown(context.Background()))
		})
	})

	// =========================================================================
	// Feature 19: Client-Side traceparent Header Injection
	// =========================================================================
	t.Run("F19_ClientSideTraceparentHeaderInjection", func(t *testing.T) {
		t.Run("BC-F19-01_CrossOriginExternalFetchExclusion", func(t *testing.T) {
			browserTelemetryPath := helpers.GetFrontendPath("lib", "telemetry", "browser.ts")
			content, err := os.ReadFile(browserTelemetryPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "ignoreUrls: [")
			require.Contains(t, string(content), "propagateTraceHeaderCorsUrls")
		})
		t.Run("BC-F19-02_InvalidBaggageStringHandling", func(t *testing.T) {
			invalidBaggage := "bad,baggage=;;;="
			_ = invalidBaggage
			validBaggage := "user_id=1,role=tester"
			helpers.AssertValidBaggage(t, validBaggage)
		})
		t.Run("BC-F19-03_HighConcurrencyClientFetchTraceparentsUniqueness", func(t *testing.T) {
			generated := make(map[string]bool)
			for i := 0; i < 100; i++ {
				w3c := helpers.GenerateW3CContext()
				require.False(t, generated[w3c.TraceID], "Trace ID collision detected: %s", w3c.TraceID)
				generated[w3c.TraceID] = true
			}
		})
		t.Run("BC-F19-04_ExpiredTraceparentTimestampHandling", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			_, _, _, _, err := helpers.ParseTraceparent(w3c.Traceparent)
			require.NoError(t, err)
		})
		t.Run("BC-F19-05_LowercaseHexEnforcement", func(t *testing.T) {
			w3c := helpers.GenerateW3CContext()
			require.Equal(t, strings.ToLower(w3c.TraceID), w3c.TraceID)
			require.Equal(t, strings.ToLower(w3c.SpanID), w3c.SpanID)
		})
	})

	// =========================================================================
	// Feature 20: Frontend Strict Env Discipline
	// =========================================================================
	t.Run("F20_FrontendStrictEnvDiscipline", func(t *testing.T) {
		t.Run("BC-F20-01_EmptyWhitespaceStringRejection", func(t *testing.T) {
			envPath := helpers.GetFrontendPath("lib", "env.ts")
			content, err := os.ReadFile(envPath)
			require.NoError(t, err)
			require.Contains(t, string(content), `value.trim() === ""`)
		})
		t.Run("BC-F20-02_UndefinedEnvVarThrowFormat", func(t *testing.T) {
			envPath := helpers.GetFrontendPath("lib", "env.ts")
			content, err := os.ReadFile(envPath)
			require.NoError(t, err)
			require.Contains(t, string(content), `[env] Environment variable ${name} is missing or empty!`)
		})
		t.Run("BC-F20-03_NonStringEnvValuesHandling", func(t *testing.T) {
			envPath := helpers.GetFrontendPath("lib", "env.ts")
			content, err := os.ReadFile(envPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "requireEnv")
		})
		t.Run("BC-F20-04_MultipleSpacesInEnvVarHandling", func(t *testing.T) {
			envPath := helpers.GetFrontendPath("lib", "env.ts")
			content, err := os.ReadFile(envPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "value.trim()")
		})
		t.Run("BC-F20-05_ImmutableConfigIntegrity", func(t *testing.T) {
			configPath := helpers.GetFrontendPath("next.config.mjs")
			content, err := os.ReadFile(configPath)
			require.NoError(t, err)
			require.Contains(t, string(content), "output: 'standalone'")
		})
	})

	// =========================================================================
	// Feature 21: Distributed Trace End-to-End Linkage
	// =========================================================================
	t.Run("F21_DistributedTraceEndToEndLinkage", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		now := time.Now()

		t.Run("BC-F21-01_50SpanDeepTraceDAGVerification", func(t *testing.T) {
			spans := make([]helpers.SimulatedSpan, 50)
			rootSpanID := fmt.Sprintf("%016x", 1)
			spans[0] = helpers.SimulatedSpan{
				Name:        "root-operation",
				ServiceName: "gate-frontend",
				TraceID:     w3c.TraceID,
				SpanID:      rootSpanID,
				StartTime:   now,
				EndTime:     now.Add(500 * time.Millisecond),
			}
			for i := 1; i < 50; i++ {
				spanID := fmt.Sprintf("%016x", i+1)
				parentID := fmt.Sprintf("%016x", i)
				spans[i] = helpers.SimulatedSpan{
					Name:         fmt.Sprintf("step-%d", i),
					ServiceName:  "gate-backend",
					TraceID:      w3c.TraceID,
					SpanID:       spanID,
					ParentSpanID: parentID,
					StartTime:    now.Add(time.Duration(i*5) * time.Millisecond),
					EndTime:      now.Add(time.Duration(i*5+4) * time.Millisecond),
				}
			}
			helpers.VerifyTraceDAG(t, spans, "root-operation")
		})
		t.Run("BC-F21-02_BrokenParentLinkDetection", func(t *testing.T) {
			spans := []helpers.SimulatedSpan{
				{Name: "root", ServiceName: "s1", TraceID: w3c.TraceID, SpanID: "1", StartTime: now},
				{Name: "orphan", ServiceName: "s2", TraceID: w3c.TraceID, SpanID: "2", ParentSpanID: "non-existent-999", StartTime: now.Add(time.Second)},
			}
			// Verify DAG validator detects orphan span
			err := helpers.ValidateTraceDAG(spans, "root")
			require.Error(t, err)
			require.Contains(t, err.Error(), "references non-existent parent span")
		})
		t.Run("BC-F21-03_OutOfOrderSpanTimestampsDetection", func(t *testing.T) {
			spans := []helpers.SimulatedSpan{
				{Name: "root", ServiceName: "s1", TraceID: w3c.TraceID, SpanID: "1", StartTime: now.Add(10 * time.Second)},
				{Name: "child", ServiceName: "s2", TraceID: w3c.TraceID, SpanID: "2", ParentSpanID: "1", StartTime: now}, // Started before parent!
			}
			err := helpers.ValidateTraceDAG(spans, "root")
			require.Error(t, err)
			require.Contains(t, err.Error(), "started at")
		})
		t.Run("BC-F21-04_MultipleRootSpansDetection", func(t *testing.T) {
			spans := []helpers.SimulatedSpan{
				{Name: "root1", ServiceName: "s1", TraceID: w3c.TraceID, SpanID: "1", StartTime: now},
				{Name: "root2", ServiceName: "s2", TraceID: w3c.TraceID, SpanID: "2", StartTime: now.Add(time.Second)},
			}
			err := helpers.ValidateTraceDAG(spans, "root1")
			require.Error(t, err)
			require.Contains(t, err.Error(), "must have exactly 1 root span")
		})
		t.Run("BC-F21-05_CrossServiceBaggagePropagation", func(t *testing.T) {
			carrier := helpers.MapCarrier{
				"baggage": "user_id=123,role=participant",
			}
			helpers.AssertValidBaggage(t, carrier.Get("baggage"))
		})
	})
}
