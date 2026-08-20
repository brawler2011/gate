package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/pkg/telemetry"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	collectorlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// --- Mock gRPC OTLP Services for in-memory end-to-end tests ---

type mockTraceServer struct {
	collectortracepb.UnimplementedTraceServiceServer
}

func (m *mockTraceServer) Export(ctx context.Context, req *collectortracepb.ExportTraceServiceRequest) (*collectortracepb.ExportTraceServiceResponse, error) {
	return &collectortracepb.ExportTraceServiceResponse{}, nil
}

type mockMetricsServer struct {
	collectormetricspb.UnimplementedMetricsServiceServer
}

func (m *mockMetricsServer) Export(ctx context.Context, req *collectormetricspb.ExportMetricsServiceRequest) (*collectormetricspb.ExportMetricsServiceResponse, error) {
	return &collectormetricspb.ExportMetricsServiceResponse{}, nil
}

type mockLogsServer struct {
	collectorlogspb.UnimplementedLogsServiceServer
}

func (m *mockLogsServer) Export(ctx context.Context, req *collectorlogspb.ExportLogsServiceRequest) (*collectorlogspb.ExportLogsServiceResponse, error) {
	return &collectorlogspb.ExportLogsServiceResponse{}, nil
}

// --- Tier 1 & Tier 2 Sanitization Tests ---

func TestSanitization_IsSensitiveKey(t *testing.T) {
	positiveCases := []string{
		"password",
		"Password",
		"PASS_WORD",
		"user_password",
		"db-password",
		"secret",
		"client_secret",
		"clientSecret",
		"secret_key",
		"jwt_token",
		"jwtToken",
		"access_token",
		"accessToken",
		"refresh_token",
		"refreshToken",
		"auth_token",
		"authToken",
		"csrf_token",
		"csrfToken",
		"id_token",
		"idToken",
		"xsrf_token",
		"xsrfToken",
		"session_id",
		"sessionId",
		"sessionID",
		"session-id",
		"SESSION_ID",
		"session_id_l2",
		"authorization",
		"auth",
		"auth_code",
		"cookie",
		"set_cookie",
		"api_key",
		"apiKey",
		"api_key_header",
		"private_key",
		"privateKey",
		"credentials",
		"card_number",
		"cardNumber",
		"cvv",
		"ssn",
		"user_password_hash",
		"client_secret_id",
		"db_password_plaintext",
	}

	for _, k := range positiveCases {
		assert.True(t, telemetry.IsSensitiveKey(k), "expected key %q to be sensitive", k)
	}

	negativeCases := []string{
		"user_id",
		"submission_id",
		"problem_id",
		"event_type",
		"status",
		"duration_ms",
		"tests_passed",
		"passed",
		"pass_count",
		"cache_key",
		"routing_key",
		"aggregate_id",
		"created_at",
		"ip",
		"method",
		"path",
	}

	for _, k := range negativeCases {
		assert.False(t, telemetry.IsSensitiveKey(k), "expected key %q NOT to be sensitive", k)
	}
}

func TestSanitization_IsSensitiveHeader(t *testing.T) {
	positive := []string{
		"Authorization",
		"authorization",
		"Cookie",
		"cookie",
		"Set-Cookie",
		"set-cookie",
		"Proxy-Authorization",
		"X-Auth-Token",
		"X-Session-ID",
		"X-API-Key",
		"X-CSRF-Token",
		"X-XSRF-Token",
		"X-Token",
		"x-token",
		"X-Session",
		"X-Api-Secret",
		"X-Password",
		"X-Auth",
	}
	for _, h := range positive {
		assert.True(t, telemetry.IsSensitiveHeader(h), "expected header %q to be sensitive", h)
	}

	negative := []string{
		"Content-Type",
		"Accept",
		"X-Request-ID",
		"User-Agent",
		"Host",
		"Origin",
	}
	for _, h := range negative {
		assert.False(t, telemetry.IsSensitiveHeader(h), "expected header %q NOT to be sensitive", h)
	}
}

func TestSanitization_SanitizeHTTPHeaders(t *testing.T) {
	assert.Nil(t, telemetry.SanitizeHTTPHeaders(nil))

	headers := http.Header{
		"Authorization":                        []string{"Bearer secret_jwt_token_123"},
		"Cookie":                               []string{"session_id=abc; other=123"},
		"Content-Type":                         []string{"application/json"},
		http.CanonicalHeaderKey("X-Request-ID"): []string{"req-uuid-456"},
	}

	sanitized := telemetry.SanitizeHTTPHeaders(headers)
	require.NotNil(t, sanitized)

	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized["Authorization"])
	assert.Equal(t, []string{telemetry.RedactedValue}, sanitized["Cookie"])
	assert.Equal(t, []string{"application/json"}, sanitized["Content-Type"])
	assert.Equal(t, []string{"req-uuid-456"}, sanitized[http.CanonicalHeaderKey("X-Request-ID")])
}

func TestSanitization_SanitizeSpanAttributes(t *testing.T) {
	assert.Nil(t, telemetry.SanitizeSpanAttributes(nil))

	attrs := []attribute.KeyValue{
		attribute.String("password", "supersecret123"),
		attribute.String("token", "tok-abc-def"),
		attribute.String("http.request.header.authorization", "Bearer my-token"),
		attribute.String("http.route", "/api/v1/submissions"),
		attribute.Int("http.response.status_code", 200),
		attribute.String("raw_auth", "Bearer eyJhbGciOi..."),
		attribute.String("basic_auth", "Basic dXNlcjpwYXNz"),
	}

	sanitized := telemetry.SanitizeSpanAttributes(attrs)
	require.Len(t, sanitized, len(attrs))

	assert.Equal(t, telemetry.RedactedValue, sanitized[0].Value.AsString())
	assert.Equal(t, telemetry.RedactedValue, sanitized[1].Value.AsString())
	assert.Equal(t, telemetry.RedactedValue, sanitized[2].Value.AsString())
	assert.Equal(t, "/api/v1/submissions", sanitized[3].Value.AsString())
	assert.Equal(t, int64(200), sanitized[4].Value.AsInt64())
	assert.Equal(t, "Bearer "+telemetry.RedactedValue, sanitized[5].Value.AsString())
	assert.Equal(t, "Basic "+telemetry.RedactedValue, sanitized[6].Value.AsString())
}

func TestSanitization_RedactSensitiveSlogAttrs(t *testing.T) {
	// Top-level sensitive attr
	attrPass := slog.String("password", "plain_password")
	redactedPass := telemetry.RedactSensitiveSlogAttrs(nil, attrPass)
	assert.Equal(t, telemetry.RedactedValue, redactedPass.Value.String())

	// Top-level session_id
	attrSession := slog.String("session_id", "sess-12345")
	redactedSession := telemetry.RedactSensitiveSlogAttrs(nil, attrSession)
	assert.Equal(t, telemetry.RedactedValue, redactedSession.Value.String())

	// Non-sensitive attr
	attrUser := slog.String("user_id", "usr-999")
	keptUser := telemetry.RedactSensitiveSlogAttrs(nil, attrUser)
	assert.Equal(t, "usr-999", keptUser.Value.String())

	// String with Bearer token on non-sensitive key
	attrBearer := slog.String("custom_header", "Bearer secret-payload")
	redactedBearer := telemetry.RedactSensitiveSlogAttrs(nil, attrBearer)
	assert.Equal(t, "Bearer "+telemetry.RedactedValue, redactedBearer.Value.String())

	// Top-level auth_header (starts with sensitive prefix auth_)
	attrAuthHeader := slog.String("auth_header", "some-auth-value")
	redactedAuthHeader := telemetry.RedactSensitiveSlogAttrs(nil, attrAuthHeader)
	assert.Equal(t, telemetry.RedactedValue, redactedAuthHeader.Value.String())

	// Empty key attr
	emptyAttr := slog.Attr{}
	assert.Equal(t, emptyAttr, telemetry.RedactSensitiveSlogAttrs(nil, emptyAttr))

	// Nested group
	groupAttr := slog.Group("details",
		slog.String("email", "user@example.com"),
		slog.String("token", "secret_token_val"),
	)
	redactedGroup := telemetry.RedactSensitiveSlogAttrs(nil, groupAttr)
	groupVal := redactedGroup.Value.Group()
	require.Len(t, groupVal, 2)
	assert.Equal(t, "user@example.com", groupVal[0].Value.String())
	assert.Equal(t, telemetry.RedactedValue, groupVal[1].Value.String())
}

func TestSanitization_SanitizingHandler(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	sanitizingHandler := telemetry.NewSanitizingHandler(baseHandler)
	assert.True(t, sanitizingHandler.Enabled(context.Background(), slog.LevelInfo))

	logger := slog.New(sanitizingHandler)

	logger.With("token", "ctx-token-123").
		WithGroup("meta").
		Info("user action",
			"user_id", "u-1",
			"password", "secret-password",
			"session_id", "sess-abc",
			"tests_passed", 15,
		)

	output := buf.String()
	assert.NotContains(t, output, "secret-password")
	assert.NotContains(t, output, "sess-abc")
	assert.NotContains(t, output, "ctx-token-123")
	assert.Contains(t, output, telemetry.RedactedValue)
	assert.Contains(t, output, "u-1")
	assert.Contains(t, output, `"tests_passed":15`)

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, telemetry.RedactedValue, parsed["token"])

	metaMap, ok := parsed["meta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, telemetry.RedactedValue, metaMap["password"])
	assert.Equal(t, telemetry.RedactedValue, metaMap["session_id"])
	assert.Equal(t, "u-1", metaMap["user_id"])
}

// --- NATS Carrier Adapter Tests ---

func TestNATSCarrier_InterfaceAndOperations(t *testing.T) {
	var _ propagation.TextMapCarrier = telemetry.NATSHeaderCarrier{}
	var _ propagation.TextMapCarrier = (*telemetry.NATSMessageCarrier)(nil)

	header := make(nats.Header)
	carrier := telemetry.NATSHeaderCarrier(header)

	carrier.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	carrier.Set("Custom-Header", "custom-value")

	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("traceparent"))
	assert.Equal(t, "custom-value", carrier.Get("custom-header"))
	assert.Equal(t, "custom-value", carrier.Get("Custom-Header"))

	keys := carrier.Keys()
	assert.Contains(t, keys, "traceparent")
	assert.Contains(t, keys, "Custom-Header")
}

func TestNATSCarrier_MessageCarrierAutoInit(t *testing.T) {
	msg := &nats.Msg{Subject: "submissions.created"}
	carrier := telemetry.NewNATSMessageCarrier(msg)

	carrier.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	assert.NotNil(t, msg.Header)
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", msg.Header.Get("traceparent"))
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("traceparent"))
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", carrier.Get("Traceparent"))
	assert.Contains(t, carrier.Keys(), "traceparent")
}

func TestNATSCarrier_NilSafety(t *testing.T) {
	var nilCarrier telemetry.NATSHeaderCarrier
	assert.Empty(t, nilCarrier.Get("traceparent"))
	assert.Nil(t, nilCarrier.Keys())
	assert.NotPanics(t, func() {
		nilCarrier.Set("key", "val")
	})

	nilMsgCarrier := telemetry.NewNATSMessageCarrier(nil)
	assert.Empty(t, nilMsgCarrier.Get("key"))
	assert.Nil(t, nilMsgCarrier.Keys())
	assert.NotPanics(t, func() {
		nilMsgCarrier.Set("key", "val")
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

func TestNATSCarrier_W3CTraceContextRoundtrip(t *testing.T) {
	// Set global propagator to W3C TraceContext
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Create valid span context
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)

	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     false,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	// Inject into NATS message
	msg := &nats.Msg{Subject: "submissions.created"}
	telemetry.InjectNATSMsg(ctx, msg)

	require.NotNil(t, msg.Header)
	traceparent := msg.Header.Get("traceparent")
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", traceparent)

	// Extract into new context
	extractedCtx := telemetry.ExtractNATSHeader(context.Background(), msg.Header)
	extractedSpan := trace.SpanFromContext(extractedCtx)
	extractedSpanCtx := extractedSpan.SpanContext()

	assert.True(t, extractedSpanCtx.IsValid())
	assert.Equal(t, traceID, extractedSpanCtx.TraceID())
	assert.Equal(t, spanID, extractedSpanCtx.SpanID())
	assert.True(t, extractedSpanCtx.IsSampled())

	// Test InjectNATS and ExtractNATS aliases
	hdr2 := make(nats.Header)
	telemetry.InjectNATS(ctx, hdr2)
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", hdr2.Get("traceparent"))
	extractedCtx2 := telemetry.ExtractNATS(context.Background(), hdr2)
	assert.Equal(t, traceID, trace.SpanFromContext(extractedCtx2).SpanContext().TraceID())
}

// --- Slog Trace/Span ID Context Injection Tests ---

func TestSlogCompositeHandler_WithActiveSpan(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test-tracer")

	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()

	spanCtx := span.SpanContext()
	require.True(t, spanCtx.IsValid())
	expectedTraceID := spanCtx.TraceID().String()
	expectedSpanID := spanCtx.SpanID().String()

	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	compositeHandler := telemetry.NewCompositeHandler(baseHandler)
	logger := slog.New(compositeHandler)

	logger.InfoContext(ctx, "processing order",
		"order_id", 1234,
		"password", "secret-key-123",
	)

	var logged map[string]any
	err := json.Unmarshal(buf.Bytes(), &logged)
	require.NoError(t, err)

	assert.Equal(t, expectedTraceID, logged["trace_id"])
	assert.Equal(t, expectedSpanID, logged["span_id"])
	assert.InDelta(t, 1234.0, logged["order_id"], 0.001)
	assert.Equal(t, telemetry.RedactedValue, logged["password"])
	assert.NotContains(t, buf.String(), "secret-key-123")
}

func TestSlogCompositeHandler_WithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	compositeHandler := telemetry.NewCompositeHandler(baseHandler)
	logger := slog.New(compositeHandler)

	logger.InfoContext(context.Background(), "background worker run", "worker_id", "w-1")

	var logged map[string]any
	err := json.Unmarshal(buf.Bytes(), &logged)
	require.NoError(t, err)

	assert.Equal(t, "w-1", logged["worker_id"])
	assert.Equal(t, "background worker run", logged["msg"])
	assert.Nil(t, logged["trace_id"])
	assert.Nil(t, logged["span_id"])
}

func TestSlogCompositeHandler_WithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	compositeHandler := telemetry.NewCompositeHandler(baseHandler)
	logger := slog.New(compositeHandler).
		With("app_token", "secret-token").
		WithGroup("service")

	logger.Info("service starting", "db_password", "super-secret-pass")

	output := buf.String()
	assert.NotContains(t, output, "secret-token")
	assert.NotContains(t, output, "super-secret-pass")

	var logged map[string]any
	err := json.Unmarshal(buf.Bytes(), &logged)
	require.NoError(t, err)
	assert.Equal(t, telemetry.RedactedValue, logged["app_token"])

	svcMap, ok := logged["service"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, telemetry.RedactedValue, svcMap["db_password"])
}

// --- Telemetry Lifecycle & Helpers Tests ---

func TestInitTelemetry_Disabled(t *testing.T) {
	cfg := telemetry.Config{
		Enabled:     false,
		Environment: "local",
	}

	shutdown, err := telemetry.InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	err = shutdown(context.Background())
	require.NoError(t, err)

	// Ensure global slog logger works
	assert.NotNil(t, telemetry.Logger())
	slog.Info("telemetry disabled test log", "key", "val")
}

func TestInitTelemetry_EmptyEndpoint(t *testing.T) {
	cfg := telemetry.Config{
		Enabled:     true,
		Endpoint:    "",
		Environment: "dev",
	}

	shutdown, err := telemetry.InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	err = shutdown(context.Background())
	require.NoError(t, err)
}

func TestInitTelemetry_Enabled(t *testing.T) {
	// Start in-process gRPC server with mock OTLP services
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
		ServiceName:    "gate-backend-test",
		ServiceVersion: "1.0.0",
		Environment:    "dev",
		Insecure:       true,
		Enabled:        true,
	}

	shutdown, err := telemetry.InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Ensure logger was set
	logger := telemetry.Logger()
	require.NotNil(t, logger)

	// Test logging with active span through composite handler and OTel log bridge
	tracer := telemetry.Tracer("test-tracer")
	ctx, span := tracer.Start(context.Background(), "test-span")
	logger.InfoContext(ctx, "test log message", "key", "val", "password", "secret")
	span.End()

	// Shutdown should flush and complete without error
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = shutdown(shutdownCtx)
	require.NoError(t, err)
}

func TestTracerAndMeter_Helpers(t *testing.T) {
	tracer := telemetry.Tracer("test-tracer")
	assert.NotNil(t, tracer)

	meter := telemetry.Meter("test-meter")
	assert.NotNil(t, meter)

	logger := telemetry.Logger()
	assert.NotNil(t, logger)
}
