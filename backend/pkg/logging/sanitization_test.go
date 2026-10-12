package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/http"
	"testing"
)

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
		assert.True(t, logging.IsSensitiveKey(k), "expected key %q to be sensitive", k)
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
		assert.False(t, logging.IsSensitiveKey(k), "expected key %q NOT to be sensitive", k)
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
		assert.True(t, logging.IsSensitiveHeader(h), "expected header %q to be sensitive", h)
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
		assert.False(t, logging.IsSensitiveHeader(h), "expected header %q NOT to be sensitive", h)
	}
}

func TestSanitization_SanitizeHTTPHeaders(t *testing.T) {
	assert.Nil(t, logging.SanitizeHTTPHeaders(nil))

	headers := http.Header{
		"Authorization":                         []string{"Bearer secret_jwt_token_123"},
		"Cookie":                                []string{"session_id=abc; other=123"},
		"Content-Type":                          []string{"application/json"},
		http.CanonicalHeaderKey("X-Request-ID"): []string{"req-uuid-456"},
	}

	sanitized := logging.SanitizeHTTPHeaders(headers)
	require.NotNil(t, sanitized)

	assert.Equal(t, []string{logging.RedactedValue}, sanitized["Authorization"])
	assert.Equal(t, []string{logging.RedactedValue}, sanitized["Cookie"])
	assert.Equal(t, []string{"application/json"}, sanitized["Content-Type"])
	assert.Equal(t, []string{"req-uuid-456"}, sanitized[http.CanonicalHeaderKey("X-Request-ID")])
}

func TestSanitization_RedactSensitiveSlogAttrs(t *testing.T) {
	// Top-level sensitive attr
	attrPass := slog.String("password", "plain_password")
	redactedPass := logging.RedactSensitiveSlogAttrs(nil, attrPass)
	assert.Equal(t, logging.RedactedValue, redactedPass.Value.String())

	// Top-level session_id
	attrSession := slog.String("session_id", "sess-12345")
	redactedSession := logging.RedactSensitiveSlogAttrs(nil, attrSession)
	assert.Equal(t, logging.RedactedValue, redactedSession.Value.String())

	// Non-sensitive attr
	attrUser := slog.String("user_id", "usr-999")
	keptUser := logging.RedactSensitiveSlogAttrs(nil, attrUser)
	assert.Equal(t, "usr-999", keptUser.Value.String())

	// String with Bearer token on non-sensitive key
	attrBearer := slog.String("custom_header", "Bearer secret-payload")
	redactedBearer := logging.RedactSensitiveSlogAttrs(nil, attrBearer)
	assert.Equal(t, "Bearer "+logging.RedactedValue, redactedBearer.Value.String())

	// Top-level auth_header (starts with sensitive prefix auth_)
	attrAuthHeader := slog.String("auth_header", "some-auth-value")
	redactedAuthHeader := logging.RedactSensitiveSlogAttrs(nil, attrAuthHeader)
	assert.Equal(t, logging.RedactedValue, redactedAuthHeader.Value.String())

	// Empty key attr
	emptyAttr := slog.Attr{}
	assert.Equal(t, emptyAttr, logging.RedactSensitiveSlogAttrs(nil, emptyAttr))

	// Nested group
	groupAttr := slog.Group("details",
		slog.String("email", "user@example.com"),
		slog.String("token", "secret_token_val"),
	)
	redactedGroup := logging.RedactSensitiveSlogAttrs(nil, groupAttr)
	groupVal := redactedGroup.Value.Group()
	require.Len(t, groupVal, 2)
	assert.Equal(t, "user@example.com", groupVal[0].Value.String())
	assert.Equal(t, logging.RedactedValue, groupVal[1].Value.String())
}

func TestSanitization_SanitizingHandler(t *testing.T) {
	var buf bytes.Buffer
	baseHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	sanitizingHandler := logging.NewSanitizingHandler(baseHandler)
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
	assert.Contains(t, output, logging.RedactedValue)
	assert.Contains(t, output, "u-1")
	assert.Contains(t, output, `"tests_passed":15`)

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, logging.RedactedValue, parsed["token"])

	metaMap, ok := parsed["meta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, logging.RedactedValue, metaMap["password"])
	assert.Equal(t, logging.RedactedValue, metaMap["session_id"])
	assert.Equal(t, "u-1", metaMap["user_id"])
}

// --- NATS Carrier Adapter Tests ---
