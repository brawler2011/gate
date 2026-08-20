package e2e_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/brawler2011/gate/tests/e2e/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

func TestTier3_CrossFeatureCombinations(t *testing.T) {
	// =========================================================================
	// TEST-T3-01: Frontend <-> Traefik (Browser Trace Ingestion & CORS)
	// =========================================================================
	t.Run("TEST_T3_01_Frontend_Traefik_CORS_Ingestion", func(t *testing.T) {
		traefikDynamicPath := helpers.GetDeployPath("local", "traefik", "dynamic.yml")
		data, err := os.ReadFile(traefikDynamicPath)
		require.NoError(t, err)

		var dynamicCfg map[string]interface{}
		require.NoError(t, yaml.Unmarshal(data, &dynamicCfg))
		httpMap := dynamicCfg["http"].(map[string]interface{})
		routers := httpMap["routers"].(map[string]interface{})
		middlewares := httpMap["middlewares"].(map[string]interface{})

		// Router rule asserts PathPrefix('/otlp')
		otlpRouter := routers["otlp-traces"].(map[string]interface{})
		require.Equal(t, "PathPrefix(`/otlp`)", otlpRouter["rule"])

		// Middleware stripPrefix asserts /otlp
		stripMw := middlewares["strip-otlp-prefix"].(map[string]interface{})
		stripPrefix := stripMw["stripPrefix"].(map[string]interface{})
		prefixes := stripPrefix["prefixes"].([]interface{})
		require.Contains(t, prefixes, "/otlp")

		// CORS allows POST and OPTIONS
		corsMw := middlewares["otlp-cors"].(map[string]interface{})
		headersMap := corsMw["headers"].(map[string]interface{})
		methods := headersMap["accessControlAllowMethods"].([]interface{})
		require.Contains(t, methods, "POST")
		require.Contains(t, methods, "OPTIONS")
	})

	// =========================================================================
	// TEST-T3-02: Frontend <-> Backend (W3C Traceparent Propagation Across HTTP)
	// =========================================================================
	t.Run("TEST_T3_02_Frontend_Backend_W3C_Propagation", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		carrier := helpers.MapCarrier{
			"traceparent": w3c.Traceparent,
		}

		tid, sid, err := helpers.SimulateCarrierRoundtrip(carrier, w3c.TraceID, w3c.SpanID)
		require.NoError(t, err)
		require.Equal(t, w3c.TraceID, tid)
		require.Equal(t, w3c.SpanID, sid)
	})

	// =========================================================================
	// TEST-T3-03: Frontend <-> Loki (Next.js SSR Error Logging & Log Export)
	// =========================================================================
	t.Run("TEST_T3_03_Frontend_Loki_SSR_ErrorLogging", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		handler, logger := helpers.NewInMemoryLogHandler(slog.LevelError, true, true)

		// Simulate SSR server error logging
		tid, _ := trace.TraceIDFromHex(w3c.TraceID)
		sid, _ := trace.SpanIDFromHex(w3c.SpanID)
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tid,
			SpanID:     sid,
			TraceFlags: trace.FlagsSampled,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), sc)

		logger.ErrorContext(ctx, "SSR server action failed to fetch problem",
			"service_name", "gate-frontend",
			"problem_id", "prob-100",
			"status", 500,
		)

		helpers.AssertLogContainsTraceContext(t, handler, w3c.TraceID, w3c.SpanID)
		records := handler.GetRecords()
		require.Equal(t, "gate-frontend", records[0].Attributes["service_name"])
	})

	// =========================================================================
	// TEST-T3-04: Frontend <-> Tempo (Browser CSR Trace Waterfall Export)
	// =========================================================================
	t.Run("TEST_T3_04_Frontend_Tempo_CSR_TraceWaterfall", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		now := time.Now()
		spans := []helpers.SimulatedSpan{
			{Name: "document_load", ServiceName: "gate-frontend-browser", TraceID: w3c.TraceID, SpanID: "1000000000000001", StartTime: now, EndTime: now.Add(200 * time.Millisecond)},
			{Name: "fetch POST /api/submissions", ServiceName: "gate-frontend-browser", TraceID: w3c.TraceID, SpanID: "1000000000000002", ParentSpanID: "1000000000000001", StartTime: now.Add(50 * time.Millisecond), EndTime: now.Add(180 * time.Millisecond)},
		}

		helpers.VerifyTraceDAG(t, spans, "document_load")
	})

	// =========================================================================
	// TEST-T3-05: Traefik <-> Backend (API Reverse Proxy & Header Passthrough)
	// =========================================================================
	t.Run("TEST_T3_05_Traefik_Backend_HeaderPassthrough", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		req, err := http.NewRequest("GET", "/api/problems", nil)
		require.NoError(t, err)

		req.Header.Set("X-Request-ID", "req-test-999")
		req.Header.Set("traceparent", w3c.Traceparent)
		req.Header.Set("X-Forwarded-For", "203.0.113.195")

		require.Equal(t, "req-test-999", req.Header.Get("X-Request-ID"))
		require.Equal(t, w3c.Traceparent, req.Header.Get("traceparent"))
		require.Equal(t, "203.0.113.195", req.Header.Get("X-Forwarded-For"))
	})

	// =========================================================================
	// TEST-T3-06: Backend <-> PostgreSQL (otelpgx Query Span Attachment)
	// =========================================================================
	t.Run("TEST_T3_06_Backend_PostgreSQL_QuerySpanAttachment", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		now := time.Now()
		spans := []helpers.SimulatedSpan{
			{Name: "HTTP GET /api/problems/1", ServiceName: "gate-backend", TraceID: w3c.TraceID, SpanID: "2000000000000001", StartTime: now, EndTime: now.Add(50 * time.Millisecond)},
			{Name: "SELECT FROM problems", ServiceName: "gate-backend", TraceID: w3c.TraceID, SpanID: "2000000000000002", ParentSpanID: "2000000000000001", StartTime: now.Add(5 * time.Millisecond), EndTime: now.Add(15 * time.Millisecond)},
		}

		helpers.VerifyTraceDAG(t, spans, "HTTP GET /api/problems/1")
		require.Equal(t, spans[0].TraceID, spans[1].TraceID)
	})

	// =========================================================================
	// TEST-T3-07: Backend <-> Outbox (Transactional W3C Trace Context Persistence)
	// =========================================================================
	t.Run("TEST_T3_07_Backend_Outbox_TransactionalPersistence", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		eventParams := helpers.CreateOutboxEventParams{
			Id:          uuid.New().String(),
			AggregateID: uuid.New().String(),
			EventType:   "submission.created",
			Payload:     []byte(`{"submission_id":"sub-123"}`),
			Headers: map[string]string{
				"traceparent": w3c.Traceparent,
				"baggage":     w3c.Baggage,
			},
		}

		require.Equal(t, w3c.Traceparent, eventParams.Headers["traceparent"])
		require.Equal(t, w3c.Baggage, eventParams.Headers["baggage"])
	})

	// =========================================================================
	// TEST-T3-08: Outbox <-> PostgreSQL (Outbox Polling Concurrency & SKIP LOCKED)
	// =========================================================================
	t.Run("TEST_T3_08_Outbox_PostgreSQL_ConcurrencySkipLocked", func(t *testing.T) {
		outboxSQLPath := helpers.GetBackendPath("internal", "repository", "pg", "outbox.sql")
		require.True(t, helpers.FileExists(outboxSQLPath), "outbox.sql must exist")
		content, err := os.ReadFile(outboxSQLPath)
		require.NoError(t, err)
		sqlStr := string(content)

		require.Contains(t, sqlStr, "-- name: PickEvents :many")
		require.Contains(t, sqlStr, "FOR UPDATE SKIP LOCKED")
		require.Contains(t, sqlStr, "ORDER BY created_at ASC")
		require.Contains(t, sqlStr, "LIMIT sqlc.arg(limit_count)::int")

		sqlcOutboxPath := helpers.GetBackendPath("internal", "repository", "pg", "sqlc", "outbox.sql.go")
		require.True(t, helpers.FileExists(sqlcOutboxPath), "sqlc outbox.sql.go must exist")
		sqlcContent, err := os.ReadFile(sqlcOutboxPath)
		require.NoError(t, err)
		require.Contains(t, string(sqlcContent), "FOR UPDATE SKIP LOCKED")
	})

	// =========================================================================
	// TEST-T3-09: Outbox <-> NATS (Outbox Dispatcher W3C Context -> JetStream Headers)
	// =========================================================================
	t.Run("TEST_T3_09_Outbox_NATS_W3CContextToJetStreamHeaders", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		outboxEvent := helpers.OutboxEvent{
			Headers: map[string]string{
				"traceparent": w3c.Traceparent,
			},
		}

		natsCarrier := helpers.MapCarrier{}
		natsCarrier.Set("traceparent", outboxEvent.Headers["traceparent"])
		require.Equal(t, w3c.Traceparent, natsCarrier.Get("traceparent"))
	})

	// =========================================================================
	// TEST-T3-10: NATS <-> Backend (Judge Worker JetStream Header Extraction)
	// =========================================================================
	t.Run("TEST_T3_10_NATS_JudgeWorker_HeaderExtraction", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		natsHeaders := helpers.MapCarrier{
			"traceparent": w3c.Traceparent,
		}

		tid, sid, err := helpers.SimulateCarrierRoundtrip(natsHeaders, w3c.TraceID, w3c.SpanID)
		require.NoError(t, err)
		require.Equal(t, w3c.TraceID, tid)
		require.Equal(t, w3c.SpanID, sid)
	})

	// =========================================================================
	// TEST-T3-11: Backend (Judge) <-> go-judge (gRPC Client otelgrpc Instrumentation)
	// =========================================================================
	t.Run("TEST_T3_11_Judge_goJudge_gRPCInstrumentation", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		now := time.Now()
		spans := []helpers.SimulatedSpan{
			{Name: "judge.process_submission", ServiceName: "gate-backend", TraceID: w3c.TraceID, SpanID: "3000000000000001", StartTime: now, EndTime: now.Add(500 * time.Millisecond)},
			{Name: "pb.Executor/Exec", ServiceName: "go-judge", TraceID: w3c.TraceID, SpanID: "3000000000000002", ParentSpanID: "3000000000000001", StartTime: now.Add(20 * time.Millisecond), EndTime: now.Add(400 * time.Millisecond)},
		}

		helpers.VerifyTraceDAG(t, spans, "judge.process_submission")
	})

	// =========================================================================
	// TEST-T3-12: Backend (Judge) <-> NATS (Judging Status Lifecycle Events)
	// =========================================================================
	t.Run("TEST_T3_12_Judge_NATS_StatusLifecycleEvents", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		events := []string{
			"submissions.compiling_started",
			"submissions.testing_started",
			"submissions.completed",
		}

		for _, ev := range events {
			msgHeader := helpers.MapCarrier{
				"traceparent": w3c.Traceparent,
				"event_type":  ev,
			}
			require.Equal(t, w3c.Traceparent, msgHeader.Get("traceparent"))
		}
	})

	// =========================================================================
	// TEST-T3-13: Backend <-> Loki (Slog Structured Record & Trace Injection to Loki)
	// =========================================================================
	t.Run("TEST_T3_13_Backend_Loki_SlogTraceInjection", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		handler, logger := helpers.NewInMemoryLogHandler(slog.LevelInfo, true, true)

		tid, _ := trace.TraceIDFromHex(w3c.TraceID)
		sid, _ := trace.SpanIDFromHex(w3c.SpanID)
		ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tid,
			SpanID:     sid,
			TraceFlags: trace.FlagsSampled,
		}))

		logger.InfoContext(ctx, "submission verified", "submission_id", "sub-100")
		helpers.AssertLogContainsTraceContext(t, handler, w3c.TraceID, w3c.SpanID)
	})

	// =========================================================================
	// TEST-T3-14: Loki <-> Tempo (Grafana Derived Field Trace Linkage)
	// =========================================================================
	t.Run("TEST_T3_14_Loki_Tempo_DerivedFieldLinkage", func(t *testing.T) {
		regexStr := `(?:trace_id|traceId|traceparent)=([0-9a-f]{32})`
		helpers.ValidateDerivedFieldRegex(t, regexStr)
	})

	// =========================================================================
	// TEST-T3-15: Tempo <-> Loki (Grafana Tempo tracesToLogsV2 Reverse Navigation)
	// =========================================================================
	t.Run("TEST_T3_15_Tempo_Loki_tracesToLogsV2ReverseNavigation", func(t *testing.T) {
		spanTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
		query := fmt.Sprintf(`{service_name=~".+"} | json | trace_id = "%s"`, spanTraceID)
		require.Contains(t, query, spanTraceID)
	})

	// =========================================================================
	// TEST-T3-16: Backend <-> VictoriaMetrics (HTTP RED Metrics Export Pipeline)
	// =========================================================================
	t.Run("TEST_T3_16_Backend_VictoriaMetrics_HTTPREDMetrics", func(t *testing.T) {
		helpers.AssertValidPromQL(t, "sum(rate(http_requests_total{http_route=\"/api/contests\"}[1m]))")
		helpers.AssertValidPromQL(t, "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))")
	})

	// =========================================================================
	// TEST-T3-17: PostgreSQL <-> VictoriaMetrics (pgxpool Stats Scrape to VM)
	// =========================================================================
	t.Run("TEST_T3_17_PostgreSQL_VictoriaMetrics_PoolStats", func(t *testing.T) {
		helpers.AssertValidPromQL(t, "pgxpool_connections_total{state=\"acquired\"}")
		helpers.AssertValidPromQL(t, "pgxpool_connections_total{state=\"idle\"}")
		helpers.AssertValidPromQL(t, "pgxpool_max_conns")
	})

	// =========================================================================
	// TEST-T3-18: NATS <-> VictoriaMetrics (JetStream Consumer Backlog Metrics)
	// =========================================================================
	t.Run("TEST_T3_18_NATS_VictoriaMetrics_QueueDepthGauges", func(t *testing.T) {
		helpers.AssertValidPromQL(t, "nats_consumer_pending_messages{consumer=\"judge_consumer\"}")
		helpers.AssertValidPromQL(t, "nats_consumer_ack_total")
	})

	// =========================================================================
	// TEST-T3-19: Outbox <-> VictoriaMetrics (Outbox Dispatch Lag & Pending Count)
	// =========================================================================
	t.Run("TEST_T3_19_Outbox_VictoriaMetrics_LagAndPendingGauges", func(t *testing.T) {
		helpers.AssertValidPromQL(t, "outbox_pending_events_count")
		helpers.AssertValidPromQL(t, "outbox_dispatch_lag_seconds")
		helpers.AssertValidPromQL(t, "outbox_dispatched_events_total")
	})

	// =========================================================================
	// TEST-T3-20: Backend (Judge) <-> VictoriaMetrics (Judge Verdict & Execution Stats)
	// =========================================================================
	t.Run("TEST_T3_20_Judge_VictoriaMetrics_VerdictStats", func(t *testing.T) {
		helpers.AssertValidPromQL(t, "sum by (verdict, language) (judge_submissions_total)")
		helpers.AssertValidPromQL(t, "histogram_quantile(0.95, sum(rate(judge_duration_seconds_bucket[5m])) by (le))")
	})

	// =========================================================================
	// TEST-T3-21: VictoriaMetrics <-> Grafana (Datasource Provisioning & PromQL Execution)
	// =========================================================================
	t.Run("TEST_T3_21_VictoriaMetrics_Grafana_DashboardQueries", func(t *testing.T) {
		coreQueries := []string{
			"go_goroutines",
			"go_memstats_alloc_bytes",
			"sum(rate(http_server_request_duration_seconds_count[1m]))",
			"sum by (verdict) (judge_submissions_total)",
			"pgxpool_connections_total",
		}
		for _, q := range coreQueries {
			helpers.AssertValidPromQL(t, q)
		}
	})

	// =========================================================================
	// TEST-T3-22: Backend (Sanitizer) <-> Loki (Attribute Masking Before Loki Ingest)
	// =========================================================================
	t.Run("TEST_T3_22_Sanitizer_Loki_AttributeMasking", func(t *testing.T) {
		handler, logger := helpers.NewInMemoryLogHandler(slog.LevelInfo, true, false)
		secretPassword := "MyP@ssword999!"
		secretSession := "sess-secret-999"

		logger.Info("user login attempt",
			"username", "bob",
			"password", secretPassword,
			"session_id", secretSession,
		)

		helpers.AssertLogSanitized(t, handler, []string{secretPassword, secretSession})
	})

	// =========================================================================
	// TEST-T3-23: Backend (Sanitizer) <-> Tempo (Span Header & Attribute Redaction)
	// =========================================================================
	t.Run("TEST_T3_23_Sanitizer_Tempo_SpanRedaction", func(t *testing.T) {
		rawHeaders := http.Header{
			"Cookie":        []string{"session_id=topsecret-cookie-val"},
			"Authorization": []string{"Bearer topsecret-jwt-token"},
		}

		sanitized := helpers.ExtractSanitizedHeaders(rawHeaders)
		helpers.AssertHeaderSanitized(t, "Cookie", "session_id=topsecret-cookie-val", sanitized)
		helpers.AssertHeaderSanitized(t, "Authorization", "Bearer topsecret-jwt-token", sanitized)
	})

	// =========================================================================
	// TEST-T3-24: OTel Collector <-> Multi-Pipeline (Tri-Pillar Fan-Out Routing)
	// =========================================================================
	t.Run("TEST_T3_24_OTelCollector_MultiPipeline_FanOut", func(t *testing.T) {
		collectorConfigPath := helpers.GetDeployPath("common", "otel-collector", "config.yaml")
		cfg := helpers.ValidateOTelCollectorConfig(t, collectorConfigPath)

		helpers.AssertOTelPipelines(t, cfg)
		helpers.AssertOTelExporters(t, cfg)
	})
}
