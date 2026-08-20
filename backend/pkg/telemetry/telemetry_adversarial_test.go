package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/telemetry"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	collectorlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

// =========================================================================
// 1. Sanitization Adversarial Tests & Bug Reproductions
// =========================================================================

// Safe business keywords that must NEVER be redacted
func TestAdversarial_Sanitization_PreservedKeywords(t *testing.T) {
	safeKeywords := []string{
		"tests_passed",
		"TESTS_PASSED",
		"pass_rate",
		"PASS_RATE",
		"passed",
		"pass_count",
		"test_case_passed",
		"cache_key",
		"CACHE_KEY",
		"cache-key",
		"routing_key",
		"routing-key",
		"key_id",
		"KEY_ID",
		"keyId",
		"id_key",
		"public_key",
		"PUBLIC_KEY",
		"rsa_public_key",
		"aggregate_id",
		"user_id",
		"submission_id",
		"problem_id",
		"duration_ms",
		"cpu_time_limit",
		"memory_limit_bytes",
		"status",
		"created_at",
		"error_message",
	}

	for _, kw := range safeKeywords {
		t.Run("SafeKey_"+kw, func(t *testing.T) {
			assert.False(t, telemetry.IsSensitiveKey(kw), "Keyword %q should NOT be sensitive", kw)

			attr := slog.String(kw, "safe_value_123")
			sanitized := telemetry.SanitizeAttr(attr)
			assert.Equal(t, "safe_value_123", sanitized.Value.String(), "Value for %q should not be redacted", kw)
		})
	}
}

type testCustomLogValuer struct {
	username string
	password string
}

func (v testCustomLogValuer) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("username", v.username),
		slog.String("password", v.password),
	)
}

// TestAdversarial_Sanitization_LogValuerBypass verifies that custom slog.LogValuer structs are properly resolved and redacted
func TestAdversarial_Sanitization_LogValuerBypass(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, nil)
	sanitizingHandler := telemetry.NewSanitizingHandler(baseHandler)
	logger := slog.New(sanitizingHandler)

	creds := testCustomLogValuer{
		username: "admin_user",
		password: "super_secret_password_123",
	}

	logger.Info("authenticating user", "creds", creds)

	output := buf.String()
	assert.NotContains(t, output, "super_secret_password_123", "LogValuer must NOT leak plaintext password")
	assert.Contains(t, output, telemetry.RedactedValue, "Password inside LogValuer must be redacted")
	assert.Contains(t, output, "admin_user", "Safe field inside LogValuer must be preserved")
}

// TestAdversarial_Sanitization_InfixAndCamelCaseBypass verifies that infix and camelCase keys are properly redacted
func TestAdversarial_Sanitization_InfixAndCamelCaseBypass(t *testing.T) {
	bypasses := []struct {
		key         string
		description string
	}{
		{"accessToken", "camelCase token without underscore"},
		{"refreshToken", "camelCase refresh token"},
		{"authToken", "camelCase auth token"},
		{"jwtToken", "camelCase jwt token"},
		{"idToken", "camelCase id token"},
		{"csrfToken", "camelCase csrf token"},
		{"xsrfToken", "camelCase xsrf token"},
		{"privateKey", "camelCase private key"},
		{"cardNumber", "camelCase card number"},
		{"clientSecret", "camelCase client secret"},
		{"user_password_hash", "infix keyword between prefix and suffix"},
		{"client_secret_id", "infix secret keyword before _id"},
		{"db_password_plaintext", "infix password keyword before _plaintext"},
		{"session_id_l2", "missing session_id_ prefix or infix match"},
		{"auth_code", "missing auth_ prefix in sensitivePrefixes"},
	}

	for _, tc := range bypasses {
		t.Run("Hardened_"+tc.key, func(t *testing.T) {
			isSens := telemetry.IsSensitiveKey(tc.key)
			assert.True(t, isSens, "%s (%s) MUST be recognized as sensitive key", tc.key, tc.description)

			attr := slog.String(tc.key, "super-secret-value-xyz")
			sanitized := telemetry.SanitizeAttr(attr)
			assert.Equal(t, telemetry.RedactedValue, sanitized.Value.String(), "Value for %q must be redacted", tc.key)
		})
	}
}

func TestAdversarial_Sanitization_DeepNestedGroups(t *testing.T) {
	deepGroup := slog.Group("level1",
		slog.String("safe_l1", "val1"),
		slog.Group("level2",
			slog.String("token", "secret_token_l2"),
			slog.Group("level3",
				slog.Int("tests_passed", 42),
				slog.Group("level4",
					slog.String("password", "secret_pass_l4"),
					slog.Group("level5",
						slog.String("user_id", "u-555"),
						slog.Group("level6",
							slog.String("session_id", "sess-666"),
							slog.Group("level7",
								slog.String("cache_key", "cache-777"),
								slog.Group("level8",
									slog.String("api_key", "key-888"),
									slog.Group("level9",
										slog.String("client_secret", "sec-999"),
										slog.Group("level10",
											slog.String("deepest_safe", "safe_10"),
											slog.String("secret_key", "sec-1010"),
										),
									),
								),
							),
						),
					),
				),
			),
		),
	)

	sanitized := telemetry.SanitizeAttr(deepGroup)

	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	sanitizedHandler := telemetry.NewSanitizingHandler(handler)
	logger := slog.New(sanitizedHandler)

	logger.Info("deep test", sanitized)

	output := buf.String()

	assert.NotContains(t, output, "secret_token_l2")
	assert.NotContains(t, output, "secret_pass_l4")
	assert.NotContains(t, output, "sess-666")
	assert.NotContains(t, output, "key-888")
	assert.NotContains(t, output, "sec-999")
	assert.NotContains(t, output, "sec-1010")

	assert.Contains(t, output, "val1")
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "u-555")
	assert.Contains(t, output, "cache-777")
	assert.Contains(t, output, "safe_10")
}

func TestAdversarial_Sanitization_HTTPHeadersEdgeCases(t *testing.T) {
	assert.Nil(t, telemetry.SanitizeHTTPHeaders(nil))

	empty := telemetry.SanitizeHTTPHeaders(http.Header{})
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	raw := make(http.Header)
	raw.Add("aUtHoRiZaTiOn", "Bearer token1")
	raw.Add("aUtHoRiZaTiOn", "Bearer token2")
	raw.Add("COOKIE", "sess=123")
	raw.Add("COOKIE", "pref=dark")
	raw.Add("Set-Cookie", "id=abc; Secure; HttpOnly")
	raw.Add("X-CsRf-ToKeN", "csrf-secret-value")
	raw.Add("X-SESSION-ID", "sess-999")
	raw.Add("X-Api-Key", "api-key-val")
	raw.Add("Proxy-Authorization", "Basic dXNlcjpwYXNz")
	raw.Add("X-Custom-Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	raw.Add("Content-Type", "application/json; charset=utf-8")
	raw.Add("Accept-Language", "en-US")
	raw.Add("Accept-Language", "en;q=0.9")

	sanitized := telemetry.SanitizeHTTPHeaders(raw)
	require.NotNil(t, sanitized)

	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized.Values("Authorization"))
	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized.Values("Cookie"))
	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized.Values("Set-Cookie"))
	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized.Values("X-Csrf-Token"))
	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized.Values("X-Session-Id"))
	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized.Values("X-Api-Key"))
	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized.Values("Proxy-Authorization"))

	assert.Equal(t, []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"}, sanitized.Values("X-Custom-Traceparent"))
	assert.Equal(t, []string{"application/json; charset=utf-8"}, sanitized.Values("Content-Type"))
	assert.Equal(t, []string{"en-US", "en;q=0.9"}, sanitized.Values("Accept-Language"))
}

func TestAdversarial_Sanitization_SpanAttributesEdgeCases(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.String("http.request.header.authorization", "Bearer my-secret-jwt"),
		attribute.String("http.request.header.cookie", "session=xyz"),
		attribute.String("http.request.header.x-api-key", "key-12345"),
		attribute.String("http.request.header.x-custom-header", "custom-safe-value"),
		attribute.String("bearer_test_lower", "bearer secret_lowercase"),
		attribute.String("basic_test_lower", "basic dXNlcjpwYXNz"),
		attribute.Int64("password", 123456),
		attribute.Bool("secret", true),
		attribute.Float64("pass_rate", 98.5),
		attribute.Int64("tests_passed", 50),
		attribute.String("cache_key", "user:profile:100"),
	}

	sanitized := telemetry.SanitizeSpanAttributes(attrs)
	require.Len(t, sanitized, len(attrs))

	assert.Equal(t, telemetry.RedactedValue, sanitized[0].Value.AsString())
	assert.Equal(t, telemetry.RedactedValue, sanitized[1].Value.AsString())
	assert.Equal(t, telemetry.RedactedValue, sanitized[2].Value.AsString())
	assert.Equal(t, "custom-safe-value", sanitized[3].Value.AsString())
	assert.Equal(t, "Bearer "+telemetry.RedactedValue, sanitized[4].Value.AsString())
	assert.Equal(t, "Basic "+telemetry.RedactedValue, sanitized[5].Value.AsString())
	assert.Equal(t, telemetry.RedactedValue, sanitized[6].Value.AsString())
	assert.Equal(t, telemetry.RedactedValue, sanitized[7].Value.AsString())
	assert.InDelta(t, 98.5, sanitized[8].Value.AsFloat64(), 0.001)
	assert.Equal(t, int64(50), sanitized[9].Value.AsInt64())
	assert.Equal(t, "user:profile:100", sanitized[10].Value.AsString())
}

// =========================================================================
// 2. NATS Carrier Adversarial & Stress Tests
// =========================================================================

func TestAdversarial_NATSCarrier_CorruptAndExtremeTraceparents(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	extremeCases := []struct {
		name        string
		traceparent string
		expectValid bool
	}{
		{"empty", "", false},
		{"random_garbage", "not-a-traceparent", false},
		{"short_hex", "00-1234-5678-01", false},
		{"invalid_hex_chars", "00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-yyyyyyyyyyyyyyyy-01", false},
		{"all_zeros_trace_id", "00-00000000000000000000000000000000-00f067aa0ba902b7-01", false},
		{"all_zeros_span_id", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01", false},
		{"future_version_99", "99-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", true},
		{"huge_string", strings.Repeat("A", 65536), false},
		{"null_bytes", "00-\x00\x00-01", false},
		{"valid_standard", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", true},
	}

	for _, tc := range extremeCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &nats.Msg{
				Subject: "test.subject",
				Header:  nats.Header{"traceparent": []string{tc.traceparent}},
			}

			var ctx context.Context
			assert.NotPanics(t, func() {
				ctx = telemetry.ExtractNATSMsg(context.Background(), msg)
			})
			require.NotNil(t, ctx)

			spanCtx := trace.SpanFromContext(ctx).SpanContext()
			if tc.expectValid {
				assert.True(t, spanCtx.IsValid(), "Expected span context to be valid for %s", tc.traceparent)
			} else {
				assert.False(t, spanCtx.IsValid(), "Expected span context to be invalid for %s", tc.traceparent)
			}
		})
	}
}

func TestAdversarial_NATSCarrier_CaseInsensitiveLookups(t *testing.T) {
	hdr := nats.Header{
		"TraceParent":   []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
		"X-bAgGaGe":     []string{"user=admin;role=evaluator"},
		"Custom-Header": []string{"val1", "val2"},
	}

	carrier := telemetry.NATSHeaderCarrier(hdr)

	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("traceparent"))
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("TRACEPARENT"))
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("TraceParent"))
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("tRaCePaReNt"))

	assert.Equal(t, "user=admin;role=evaluator", carrier.Get("x-baggage"))
	assert.Equal(t, "user=admin;role=evaluator", carrier.Get("X-BAGGAGE"))

	assert.Empty(t, carrier.Get("non-existent-header"))
}

func TestAdversarial_NATSCarrier_ConcurrentReadWrite(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	const numGoroutines = 100
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			traceHex := fmt.Sprintf("4bf92f3577b34da6a3ce929d0e0e%04x", id)
			spanHex := fmt.Sprintf("00f067aa0ba9%04x", id)

			tID, err := trace.TraceIDFromHex(traceHex)
			if err != nil {
				t.Errorf("invalid trace hex: %v", err)
				return
			}
			sID, err := trace.SpanIDFromHex(spanHex)
			if err != nil {
				t.Errorf("invalid span hex: %v", err)
				return
			}

			spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    tID,
				SpanID:     sID,
				TraceFlags: trace.FlagsSampled,
			})
			ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

			for j := 0; j < iterations; j++ {
				msg := &nats.Msg{Subject: "concurrent.test"}
				telemetry.InjectNATSMsg(ctx, msg)

				extractedCtx := telemetry.ExtractNATSMsg(context.Background(), msg)
				extSpan := trace.SpanFromContext(extractedCtx).SpanContext()

				if !extSpan.IsValid() || extSpan.TraceID() != tID || extSpan.SpanID() != sID {
					t.Errorf("mismatch in goroutine %d iteration %d", id, j)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

// =========================================================================
// 3. Composite slog Handler & Concurrency Stress Tests
// =========================================================================

func TestAdversarial_Slog_ConcurrentLogging(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("concurrent-test-tracer")

	var buf bytes.Buffer
	var mu sync.Mutex

	safeWriter := &syncWriter{w: &buf, mu: &mu}
	baseHandler := slog.NewJSONHandler(safeWriter, &slog.HandlerOptions{Level: slog.LevelDebug})
	composite := telemetry.NewCompositeHandler(baseHandler)
	rootLogger := slog.New(composite)

	const goroutines = 50
	const logsPerGoroutine = 40

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()

			subLogger := rootLogger.
				With("worker_id", gID).
				WithGroup(fmt.Sprintf("group_%d", gID))

			for i := 0; i < logsPerGoroutine; i++ {
				ctx, span := tracer.Start(context.Background(), fmt.Sprintf("op_%d_%d", gID, i))

				subLogger.InfoContext(ctx, "executing task",
					"iteration", i,
					"password", "secret_pass_val",
					"token", "secret_token_val",
					"tests_passed", 100+i,
				)

				span.End()
			}
		}(g)
	}

	wg.Wait()

	output := buf.String()

	assert.NotContains(t, output, "secret_pass_val")
	assert.NotContains(t, output, "secret_token_val")
	assert.Contains(t, output, telemetry.RedactedValue)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, goroutines*logsPerGoroutine)

	for _, line := range lines {
		var record map[string]any
		err := json.Unmarshal([]byte(line), &record)
		require.NoError(t, err, "Log line must be valid JSON: %s", line)

		var foundTrace, foundSpan bool
		if record["trace_id"] != nil {
			foundTrace = true
		}
		if record["span_id"] != nil {
			foundSpan = true
		}

		for _, v := range record {
			if gMap, ok := v.(map[string]any); ok {
				if gMap["trace_id"] != nil && gMap["trace_id"] != "" {
					foundTrace = true
				}
				if gMap["span_id"] != nil && gMap["span_id"] != "" {
					foundSpan = true
				}
			}
		}

		assert.True(t, foundTrace, "Each log must have trace_id injected")
		assert.True(t, foundSpan, "Each log must have span_id injected")
	}
}

func TestAdversarial_Lifecycle_TightContextShutdown(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	collectortracepb.RegisterTraceServiceServer(srv, &mockTraceServer{})
	collectormetricspb.RegisterMetricsServiceServer(srv, &mockMetricsServer{})
	collectorlogspb.RegisterLogsServiceServer(srv, &mockLogsServer{})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	cfg := telemetry.Config{
		Endpoint:       lis.Addr().String(),
		ServiceName:    "gate-test-shutdown",
		ServiceVersion: "0.1.0",
		Environment:    "local",
		Insecure:       true,
		Enabled:        true,
	}

	shutdown, err := telemetry.InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	tightCtx, cancelTight := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancelTight()
	_ = shutdown(tightCtx)

	canceledCtx, cancelCanceled := context.WithCancel(context.Background())
	cancelCanceled()
	_ = shutdown(canceledCtx)

	var wg sync.WaitGroup
	var panicCount int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt32(&panicCount, 1)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			_ = shutdown(ctx)
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(0), atomic.LoadInt32(&panicCount), "Concurrent shutdown calls must not panic")
}

func TestAdversarial_NilAndDefensiveBoundaries(t *testing.T) {
	emptyComposite := telemetry.NewCompositeHandler(nil, nil, nil)
	require.NotNil(t, emptyComposite)
	assert.False(t, emptyComposite.Enabled(context.Background(), slog.LevelInfo))
	require.NoError(t, emptyComposite.Handle(context.Background(), slog.Record{}))
	assert.NotNil(t, emptyComposite.WithAttrs(nil))
	assert.NotNil(t, emptyComposite.WithGroup("test"))

	var nilSlogAttrs []slog.Attr
	assert.Nil(t, telemetry.SanitizeAttrs(nilSlogAttrs))
	assert.Empty(t, telemetry.SanitizeAttrs([]slog.Attr{}))

	assert.Nil(t, telemetry.SanitizeHTTPHeaders(nil))
	emptyHeaders := make(http.Header)
	sanitizedEmpty := telemetry.SanitizeHTTPHeaders(emptyHeaders)
	assert.NotNil(t, sanitizedEmpty)
	assert.Empty(t, sanitizedEmpty)

	assert.Nil(t, telemetry.SanitizeSpanAttributes(nil))
	assert.Empty(t, telemetry.SanitizeSpanAttributes([]attribute.KeyValue{}))

	var nilHeaderCarrier telemetry.NATSHeaderCarrier
	assert.Empty(t, nilHeaderCarrier.Get("traceparent"))
	assert.Nil(t, nilHeaderCarrier.Keys())
	assert.NotPanics(t, func() {
		nilHeaderCarrier.Set("k", "v")
	})

	var nilMsgCarrier *telemetry.NATSMessageCarrier
	assert.Empty(t, nilMsgCarrier.Get("traceparent"))
	assert.Nil(t, nilMsgCarrier.Keys())
	assert.NotPanics(t, func() {
		nilMsgCarrier.Set("k", "v")
	})

	nilWrappedCarrier := telemetry.NewNATSMessageCarrier(nil)
	assert.Empty(t, nilWrappedCarrier.Get("traceparent"))
	assert.Nil(t, nilWrappedCarrier.Keys())
	assert.NotPanics(t, func() {
		nilWrappedCarrier.Set("k", "v")
	})

	assert.NotPanics(t, func() {
		telemetry.InjectNATSMsg(context.Background(), nil)
		_ = telemetry.ExtractNATSMsg(context.Background(), nil)
		telemetry.InjectNATSHeader(context.Background(), nil)
		_ = telemetry.ExtractNATSHeader(context.Background(), nil)
		telemetry.InjectNATS(context.Background(), nil)
		_ = telemetry.ExtractNATS(context.Background(), nil)
	})
}

func TestAdversarial_UnreachableCollector_GracefulDegradation(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedAddr := lis.Addr().String()
	_ = lis.Close()

	cfg := telemetry.Config{
		Endpoint:       closedAddr,
		ServiceName:    "gate-backend-adversarial",
		ServiceVersion: "0.1.0",
		Environment:    "dev",
		Insecure:       true,
		Enabled:        true,
	}

	shutdown, err := telemetry.InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	logger := telemetry.Logger()
	require.NotNil(t, logger)

	tracer := telemetry.Tracer("unreachable-test")
	ctx, span := tracer.Start(context.Background(), "unreachable-span")
	logger.InfoContext(ctx, "message during collector outage",
		"event", "order_placed",
		"password", "secret123",
	)
	span.End()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = shutdown(shutdownCtx)
}

func BenchmarkCompositeHandler_Handle(b *testing.B) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	composite := telemetry.NewCompositeHandler(baseHandler)
	logger := slog.New(composite)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		logger.InfoContext(ctx, "benchmark log message",
			"user_id", "usr-12345",
			"password", "super-secret-password",
			"session_id", "sess-abcdef-123456",
			"tests_passed", 42,
			"duration_ms", 128.5,
		)
	}
}

func BenchmarkSanitizeAttr_Primitive(b *testing.B) {
	attr := slog.String("password", "plain-secret-password")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = telemetry.SanitizeAttr(attr)
	}
}

func BenchmarkSanitizeAttr_DeepGroup(b *testing.B) {
	group := slog.Group("level1",
		slog.String("token", "tok-123"),
		slog.Group("level2",
			slog.String("password", "secret"),
			slog.Int("tests_passed", 100),
		),
	)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = telemetry.SanitizeAttr(group)
	}
}

func BenchmarkSanitizeSpanAttribute(b *testing.B) {
	attr := attribute.String("http.request.header.authorization", "Bearer eyJhbGciOi...")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = telemetry.SanitizeSpanAttribute(attr)
	}
}

func BenchmarkNATSHeaderCarrier_InjectExtract(b *testing.B) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)
	msg := &nats.Msg{Subject: "bench.topic"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg.Header = nil
		telemetry.InjectNATSMsg(ctx, msg)
		_ = telemetry.ExtractNATSMsg(context.Background(), msg)
	}
}

func BenchmarkNATSHeaderCarrier_GetCaseInsensitive(b *testing.B) {
	hdr := nats.Header{
		"X-Trace-Context": []string{"val1"},
		"Authorization":   []string{"val2"},
		"TraceParent":     []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
	}
	carrier := telemetry.NATSHeaderCarrier(hdr)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = carrier.Get("traceparent")
	}
}

type syncWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	if s.mu != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	return s.w.Write(p)
}

