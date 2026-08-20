package helpers

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// DefaultSensitiveKeys lists keys that MUST be redacted across headers, spans, and slog records.
var DefaultSensitiveKeys = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"access_token",
	"refresh_token",
	"auth_token",
	"authorization",
	"cookie",
	"set-cookie",
	"session_id",
	"sessionid",
	"private_key",
	"api_key",
	"apikey",
}

// RedactedPlaceholder standard string used when redacting secrets.
const RedactedPlaceholder = "[REDACTED]"

// IsSensitiveKey returns true if key matches any known sensitive field name.
// It ensures benign substrings like "token_count" or "passwords_updated_total" are NOT treated as sensitive keys.
func IsSensitiveKey(key string) bool {
	norm := strings.ToLower(strings.TrimSpace(key))
	norm = strings.ReplaceAll(norm, "-", "_")
	norm = strings.ReplaceAll(norm, ".", "_")

	// Exact matches
	for _, sens := range DefaultSensitiveKeys {
		sensNorm := strings.ReplaceAll(strings.ToLower(sens), "-", "_")
		if norm == sensNorm {
			return true
		}
	}

	// Suffix/Prefix patterns for composite keys (e.g. "user_password", "db_passwd", "admin_apikey", "auth_token")
	if strings.HasSuffix(norm, "_password") || strings.HasSuffix(norm, "_passwd") ||
		strings.HasSuffix(norm, "_secret") || strings.HasSuffix(norm, "_token") ||
		strings.HasSuffix(norm, "_key") || strings.HasSuffix(norm, "_apikey") ||
		strings.HasSuffix(norm, "_auth_token") ||
		strings.HasSuffix(norm, "_session_id") || strings.HasSuffix(norm, "_sessionid") {
		// Verify not a benign counter/metric like "total_token_count" or "token_count"
		if !strings.HasSuffix(norm, "_count") && !strings.HasSuffix(norm, "_total") && !strings.HasSuffix(norm, "_rate") {
			return true
		}
	}

	return false
}

// SanitizeSlogAttribute recursively sanitizes slog.Attr according to two-tier sanitization rules.
func SanitizeSlogAttribute(attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindGroup {
		groupAttrs := attr.Value.Group()
		sanitizedGroup := make([]slog.Attr, len(groupAttrs))
		for i, a := range groupAttrs {
			sanitizedGroup[i] = SanitizeSlogAttribute(a)
		}
		return slog.Attr{
			Key:   attr.Key,
			Value: slog.GroupValue(sanitizedGroup...),
		}
	}

	if IsSensitiveKey(attr.Key) {
		return slog.String(attr.Key, RedactedPlaceholder)
	}

	return attr
}

// AssertHeaderSanitized checks that sensitive HTTP headers are masked or omitted in recorded attributes.
func AssertHeaderSanitized(t *testing.T, headerName string, headerValue string, recordedAttributes map[string]string) {
	t.Helper()
	found := false
	for attrKey, attrVal := range recordedAttributes {
		if strings.EqualFold(attrKey, headerName) || strings.HasSuffix(strings.ToLower(attrKey), "."+strings.ToLower(headerName)) {
			found = true
			require.NotEqual(t, headerValue, attrVal, "Sensitive header '%s' value leaked in attribute '%s'", headerName, attrKey)
			require.Equal(t, RedactedPlaceholder, attrVal, "Sensitive header '%s' should be masked as '%s'", headerName, RedactedPlaceholder)
		}
	}
	require.True(t, found, "Header '%s' was not found in recorded attributes", headerName)
}

// AssertSpanSanitization checks that no known secrets leak into span attributes or span events.
func AssertSpanSanitization(t *testing.T, spans []sdktrace.ReadOnlySpan, knownSecrets []string) {
	t.Helper()
	for _, span := range spans {
		for _, attr := range span.Attributes() {
			valStr := attr.Value.AsString()
			for _, secret := range knownSecrets {
				if secret == "" {
					continue
				}
				require.False(t, strings.Contains(valStr, secret),
					"Secret '%s' leaked in span '%s' attribute '%s' (value: %s)",
					secret, span.Name(), attr.Key, valStr)
			}
			if IsSensitiveKey(string(attr.Key)) {
				require.Equal(t, RedactedPlaceholder, valStr,
					"Sensitive attribute '%s' in span '%s' not masked as [REDACTED]", attr.Key, span.Name())
			}
		}

		for _, event := range span.Events() {
			for _, attr := range event.Attributes {
				valStr := attr.Value.AsString()
				for _, secret := range knownSecrets {
					if secret == "" {
						continue
					}
					require.False(t, strings.Contains(valStr, secret),
						"Secret '%s' leaked in span '%s' event '%s' attribute '%s'",
						secret, span.Name(), event.Name, attr.Key)
				}
			}
		}
	}
}

// AssertLogOutputSanitization checks that raw log text or structured JSON does not contain known plaintext secrets.
func AssertLogOutputSanitization(t *testing.T, logOutput string, knownSecrets []string) {
	t.Helper()
	for _, secret := range knownSecrets {
		if secret == "" {
			continue
		}
		require.False(t, strings.Contains(logOutput, secret),
			"Secret '%s' leaked into log output:\n%s", secret, logOutput)
	}
}

// ExtractSanitizedHeaders maps standard http.Header into safe dictionary.
func ExtractSanitizedHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for k, vals := range h {
		if IsSensitiveKey(k) {
			result[k] = RedactedPlaceholder
		} else {
			result[k] = strings.Join(vals, ", ")
		}
	}
	return result
}

// ScanSourceForEnvFallbacks statically checks code files for forbidden env fallback patterns (e.g. process.env.VAR || "default" or ?? "default").
func ScanSourceForEnvFallbacks(content string) (violatingLines []string) {
	re := regexp.MustCompile(`process\.env\.[A-Z0-9_]+\s*(\|\||\?\?)\s*["'].+["']`)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue // Skip comments
		}
		if re.MatchString(trimmed) {
			violatingLines = append(violatingLines, trimmed)
		}
	}
	return violatingLines
}
