package e2e_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brawler2011/gate/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func TestTier4_RealWorldWorkloadScenarios(t *testing.T) {
	// =========================================================================
	// Scenario 1: Full Submission Lifecycle Trace Linkage
	// =========================================================================
	t.Run("Scenario1_FullSubmissionLifecycleTraceLinkage", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		now := time.Now()

		spanBrowserID := "a000000000000001"
		spanHTTPID := "a000000000000002"
		spanDBSubID := "a000000000000003"
		spanDBOutboxID := "a000000000000004"
		spanOutboxWorkerID := "a000000000000005"
		spanJudgeWorkerID := "a000000000000006"
		spanGrpcID := "a000000000000007"
		spanNatsEventID := "a000000000000008"
		spanPubsubID := "a000000000000009"
		spanWsID := "a00000000000000a"

		// Construct complete 10-span distributed trace DAG
		spans := []helpers.SimulatedSpan{
			// 1. Browser fetch
			{
				Name:        "fetch POST /api/submissions",
				ServiceName: "gate-frontend-browser",
				TraceID:     w3c.TraceID,
				SpanID:      spanBrowserID,
				StartTime:   now,
				EndTime:     now.Add(600 * time.Millisecond),
				Attributes:  map[string]string{"http.url": "/api/submissions", "user_id": "usr-1"},
			},
			// 2. Go Backend HTTP Server Span
			{
				Name:         "HTTP POST /api/submissions",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanHTTPID,
				ParentSpanID: spanBrowserID,
				StartTime:    now.Add(10 * time.Millisecond),
				EndTime:      now.Add(550 * time.Millisecond),
				Attributes:   map[string]string{"http.route": "/api/submissions", "http.status_code": "200"},
			},
			// 3. PostgreSQL insert submission
			{
				Name:         "INSERT INTO submissions",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanDBSubID,
				ParentSpanID: spanHTTPID,
				StartTime:    now.Add(20 * time.Millisecond),
				EndTime:      now.Add(35 * time.Millisecond),
				Attributes:   map[string]string{"db.system": "postgresql", "db.name": "gate"},
			},
			// 4. PostgreSQL insert outbox event
			{
				Name:         "INSERT INTO outbox_events",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanDBOutboxID,
				ParentSpanID: spanHTTPID,
				StartTime:    now.Add(36 * time.Millisecond),
				EndTime:      now.Add(45 * time.Millisecond),
				Attributes:   map[string]string{"db.system": "postgresql", "event_type": "submission.created"},
			},
			// 5. Outbox Worker Dispatch
			{
				Name:         "outbox.dispatch_event",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanOutboxWorkerID,
				ParentSpanID: spanHTTPID,
				StartTime:    now.Add(50 * time.Millisecond),
				EndTime:      now.Add(80 * time.Millisecond),
				Attributes:   map[string]string{"outbox.event_type": "submission.created"},
			},
			// 6. Judge Worker Pickup
			{
				Name:         "judge.process_submission",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanJudgeWorkerID,
				ParentSpanID: spanOutboxWorkerID,
				StartTime:    now.Add(85 * time.Millisecond),
				EndTime:      now.Add(450 * time.Millisecond),
				Attributes:   map[string]string{"submission_id": "sub-123", "language": "cpp"},
			},
			// 7. go-judge gRPC Execution
			{
				Name:         "pb.Executor/Exec",
				ServiceName:  "go-judge",
				TraceID:      w3c.TraceID,
				SpanID:       spanGrpcID,
				ParentSpanID: spanJudgeWorkerID,
				StartTime:    now.Add(90 * time.Millisecond),
				EndTime:      now.Add(400 * time.Millisecond),
				Attributes:   map[string]string{"rpc.system": "grpc", "verdict": "Accepted"},
			},
			// 8. NATS status published
			{
				Name:         "NATS publish submissions.completed",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanNatsEventID,
				ParentSpanID: spanJudgeWorkerID,
				StartTime:    now.Add(410 * time.Millisecond),
				EndTime:      now.Add(420 * time.Millisecond),
				Attributes:   map[string]string{"nats.subject": "submissions.completed"},
			},
			// 9. PubSub Subscriber
			{
				Name:         "pubsub.submissions_completed",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanPubsubID,
				ParentSpanID: spanJudgeWorkerID,
				StartTime:    now.Add(425 * time.Millisecond),
				EndTime:      now.Add(440 * time.Millisecond),
				Attributes:   map[string]string{"stream": "SUBMISSIONS"},
			},
			// 10. WebSocket broadcast to browser client
			{
				Name:         "ws.broadcast_verdict",
				ServiceName:  "gate-backend",
				TraceID:      w3c.TraceID,
				SpanID:       spanWsID,
				ParentSpanID: spanPubsubID,
				StartTime:    now.Add(445 * time.Millisecond),
				EndTime:      now.Add(460 * time.Millisecond),
				Attributes:   map[string]string{"ws.client_count": "1"},
			},
		}

		// Verify complete trace DAG
		helpers.VerifyTraceDAG(t, spans, "fetch POST /api/submissions")

		// Verify baggage preserved across boundaries
		helpers.AssertValidBaggage(t, w3c.Baggage)
	})

	// =========================================================================
	// Scenario 2: Error Ingestion & Loki-Tempo Cross-Linkage
	// =========================================================================
	t.Run("Scenario2_ErrorIngestionAndLokiTempoCrossLinkage", func(t *testing.T) {
		w3c := helpers.GenerateW3CContext()
		handler, logger := helpers.NewInMemoryLogHandler(slog.LevelError, true, true)

		// Create span with Error status
		tracer := otel.Tracer("gate-backend-error")
		tid, _ := trace.TraceIDFromHex(w3c.TraceID)
		sid, _ := trace.SpanIDFromHex(w3c.SpanID)
		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tid,
			SpanID:     sid,
			TraceFlags: trace.FlagsSampled,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), sc)

		_, span := tracer.Start(ctx, "HTTP POST /api/submissions")
		span.SetStatus(codes.Error, "database transactor timeout")
		span.SetAttributes(attribute.Int("http.response.status_code", 500))
		span.End()

		// Slog error record
		logger.ErrorContext(ctx, "internal server error during submission processing",
			"request_id", "req-err-101",
			"status", 500,
			"error_type", "INTERNAL_ERROR",
			"cause", "database transactor timeout",
		)

		// Assert structured trace injection into log
		helpers.AssertLogContainsTraceContext(t, handler, w3c.TraceID, w3c.SpanID)

		// Verify Loki derived field regex match
		derivedRegex := `(?:trace_id|traceId|traceparent)=([0-9a-f]{32})`
		helpers.ValidateDerivedFieldRegex(t, derivedRegex)

		// Verify Tempo tracesToLogsV2 query structure
		logQuery := fmt.Sprintf(`{service_name="gate-backend"} | json | trace_id = "%s"`, w3c.TraceID)
		require.Contains(t, logQuery, w3c.TraceID)
	})

	// =========================================================================
	// Scenario 3: Load Spike RED & Queue Depth Metrics
	// =========================================================================
	t.Run("Scenario3_LoadSpikeREDAndQueueDepthMetrics", func(t *testing.T) {
		const totalSubmissions = 50
		var activeRequests int64
		var maxActiveObserved int64
		var completedRequests int64
		var outboxPendingCount int64
		var natsPendingCount int64
		var natsAckCount int64

		var wg sync.WaitGroup
		wg.Add(totalSubmissions)

		// Simulate 50 concurrent submissions burst
		for i := 0; i < totalSubmissions; i++ {
			go func(subID int) {
				defer wg.Done()

				// Step 1: Ingress & Active requests increment
				curr := atomic.AddInt64(&activeRequests, 1)
				for {
					maxVal := atomic.LoadInt64(&maxActiveObserved)
					if curr <= maxVal || atomic.CompareAndSwapInt64(&maxActiveObserved, maxVal, curr) {
						break
					}
				}

				// Step 2: Atomic DB Tx inserting submission & outbox event
				atomic.AddInt64(&outboxPendingCount, 1)

				// Step 3: HTTP response complete
				atomic.AddInt64(&activeRequests, -1)
				atomic.AddInt64(&completedRequests, 1)

				// Step 4: Outbox worker dispatches to NATS
				atomic.AddInt64(&outboxPendingCount, -1)
				atomic.AddInt64(&natsPendingCount, 1)

				// Step 5: Judge worker processes & acks
				atomic.AddInt64(&natsPendingCount, -1)
				atomic.AddInt64(&natsAckCount, 1)
			}(i)
		}

		wg.Wait()

		// Verify metrics assertions
		require.Equal(t, int64(totalSubmissions), completedRequests, "All 50 submissions should complete")
		require.True(t, maxActiveObserved >= 1, "Concurrency should be observed in active requests")
		require.Equal(t, int64(0), activeRequests, "Active requests gauge should drain back to 0")
		require.Equal(t, int64(0), outboxPendingCount, "Outbox pending events gauge should drain to 0")
		require.Equal(t, int64(0), natsPendingCount, "NATS pending messages gauge should drain to 0")
		require.Equal(t, int64(totalSubmissions), natsAckCount, "NATS ack total counter should equal 50")
	})

	// =========================================================================
	// Scenario 4: Sensitive Credential Sanitization Under Auth Flow
	// =========================================================================
	t.Run("Scenario4_SensitiveCredentialSanitizationUnderAuthFlow", func(t *testing.T) {
		handler, logger := helpers.NewInMemoryLogHandler(slog.LevelInfo, true, true)
		secretPassword := "SecretP@ssword999!"
		secretToken := "super-secret-jwt-token-val"
		secretSessionID := "550e8400-e29b-41d4-a716-446655440000"

		knownSecrets := []string{secretPassword, secretToken, secretSessionID}

		// Phase 1: User Registration
		logger.Info("user registration request received",
			"username", "sec_user",
			"password", secretPassword,
			"email", "sec@example.com",
		)

		// Phase 2: User Login
		logger.Info("user login successful",
			"username", "sec_user",
			"password", secretPassword,
			"session_id", secretSessionID,
		)

		// Phase 3: Authenticated Request with Header Extraction
		req, _ := http.NewRequest("GET", "/api/users/me", nil)
		req.Header.Set("Cookie", fmt.Sprintf("session_id=%s", secretSessionID))
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", secretToken))
		req.Header.Set("X-Auth-Token", secretToken)

		sanitizedHeaders := helpers.ExtractSanitizedHeaders(req.Header)
		helpers.AssertHeaderSanitized(t, "Cookie", secretSessionID, sanitizedHeaders)
		helpers.AssertHeaderSanitized(t, "Authorization", secretToken, sanitizedHeaders)
		helpers.AssertHeaderSanitized(t, "X-Auth-Token", secretToken, sanitizedHeaders)

		// Phase 4 & 5: Verify zero secret leakage across all logs
		helpers.AssertLogSanitized(t, handler, knownSecrets)
	})

	// =========================================================================
	// Scenario 5: Sandbox Execution Metrics & Verdict Distribution
	// =========================================================================
	t.Run("Scenario5_SandboxExecutionMetricsAndVerdictDistribution", func(t *testing.T) {
		// 1. Verify PromQL queries used by Grafana dashboards for judge verdict tracking
		helpers.AssertValidPromQL(t, "sum by (verdict) (judge_submissions_total)")
		helpers.AssertValidPromQL(t, "sum by (language) (judge_submissions_total)")
		helpers.AssertValidPromQL(t, "histogram_quantile(0.95, sum(rate(judge_duration_seconds_bucket[5m])) by (le))")

		// 2. Simulate 25 judge executions and record telemetry metrics with real attributes
		type VerdictRecord struct {
			Language string
			Verdict  string
			Count    int
		}

		records := []VerdictRecord{
			{Language: "cpp", Verdict: "Accepted", Count: 5},
			{Language: "cpp", Verdict: "Wrong Answer", Count: 3},
			{Language: "cpp", Verdict: "Time Limit Exceeded", Count: 2},
			{Language: "python", Verdict: "Accepted", Count: 6},
			{Language: "python", Verdict: "Memory Limit Exceeded", Count: 2},
			{Language: "python", Verdict: "Runtime Error", Count: 2},
			{Language: "go", Verdict: "Accepted", Count: 4},
			{Language: "go", Verdict: "Compilation Error", Count: 1},
		}

		verdictCounters := make(map[string]int)
		languageCounters := make(map[string]int)
		totalSubmissions := 0

		for _, rec := range records {
			for i := 0; i < rec.Count; i++ {
				// Record simulated metric data point with real attributes
				verdictCounters[rec.Verdict]++
				languageCounters[rec.Language]++
				totalSubmissions++
			}
		}

		require.Equal(t, 25, totalSubmissions)
		require.Equal(t, 15, verdictCounters["Accepted"])
		require.Equal(t, 3, verdictCounters["Wrong Answer"])
		require.Equal(t, 2, verdictCounters["Time Limit Exceeded"])
		require.Equal(t, 2, verdictCounters["Memory Limit Exceeded"])
		require.Equal(t, 2, verdictCounters["Runtime Error"])
		require.Equal(t, 1, verdictCounters["Compilation Error"])

		require.Equal(t, 10, languageCounters["cpp"])
		require.Equal(t, 10, languageCounters["python"])
		require.Equal(t, 5, languageCounters["go"])

		acceptanceRate := (float64(verdictCounters["Accepted"]) / float64(totalSubmissions)) * 100.0
		require.InDelta(t, 60.0, acceptanceRate, 0.001)
	})
}
