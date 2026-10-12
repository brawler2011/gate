package logging_test

import (
	"bytes"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/http"
	"testing"
)

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
			assert.False(t, logging.IsSensitiveKey(kw), "Keyword %q should NOT be sensitive", kw)

			attr := slog.String(kw, "safe_value_123")
			sanitized := logging.SanitizeAttr(attr)
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
	sanitizingHandler := logging.NewSanitizingHandler(baseHandler)
	logger := slog.New(sanitizingHandler)

	creds := testCustomLogValuer{
		username: "admin_user",
		password: "super_secret_password_123",
	}

	logger.Info("authenticating user", "creds", creds)

	output := buf.String()
	assert.NotContains(t, output, "super_secret_password_123", "LogValuer must NOT leak plaintext password")
	assert.Contains(t, output, logging.RedactedValue, "Password inside LogValuer must be redacted")
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
			isSens := logging.IsSensitiveKey(tc.key)
			assert.True(t, isSens, "%s (%s) MUST be recognized as sensitive key", tc.key, tc.description)

			attr := slog.String(tc.key, "super-secret-value-xyz")
			sanitized := logging.SanitizeAttr(attr)
			assert.Equal(t, logging.RedactedValue, sanitized.Value.String(), "Value for %q must be redacted", tc.key)
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

	sanitized := logging.SanitizeAttr(deepGroup)

	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	sanitizedHandler := logging.NewSanitizingHandler(handler)
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
	assert.Nil(t, logging.SanitizeHTTPHeaders(nil))

	empty := logging.SanitizeHTTPHeaders(http.Header{})
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

	sanitized := logging.SanitizeHTTPHeaders(raw)
	require.NotNil(t, sanitized)

	assert.Equal(t, []string{logging.RedactedValue}, sanitized.Values("Authorization"))
	assert.Equal(t, []string{logging.RedactedValue}, sanitized.Values("Cookie"))
	assert.Equal(t, []string{logging.RedactedValue}, sanitized.Values("Set-Cookie"))
	assert.Equal(t, []string{logging.RedactedValue}, sanitized.Values("X-Csrf-Token"))
	assert.Equal(t, []string{logging.RedactedValue}, sanitized.Values("X-Session-Id"))
	assert.Equal(t, []string{logging.RedactedValue}, sanitized.Values("X-Api-Key"))
	assert.Equal(t, []string{logging.RedactedValue}, sanitized.Values("Proxy-Authorization"))

	assert.Equal(t, []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"}, sanitized.Values("X-Custom-Traceparent"))
	assert.Equal(t, []string{"application/json; charset=utf-8"}, sanitized.Values("Content-Type"))
	assert.Equal(t, []string{"en-US", "en;q=0.9"}, sanitized.Values("Accept-Language"))
}
