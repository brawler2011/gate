package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

// RedactedValue is the replacement string used for masked sensitive values.
const RedactedValue = "[REDACTED]"

// sensitiveHeaders contains lowercase names of HTTP headers that must be redacted.
var sensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
	"x-auth-token":        {},
	"x-session-id":        {},
	"x-api-key":           {},
	"x-access-token":      {},
	"x-csrf-token":        {},
	"x-xsrf-token":        {},
}

// exactSensitiveKeys contains lowercase normalized keys that are always redacted.
var exactSensitiveKeys = map[string]struct{}{
	"password":       {},
	"passwd":         {},
	"pass_word":      {},
	"password_hash":  {},
	"passwordhash":   {},
	"secret":         {},
	"token":          {},
	"session_id":     {},
	"sessionid":      {},
	"session":        {},
	"authorization":  {},
	"auth":           {},
	"auth_code":      {},
	"authcode":       {},
	"cookie":         {},
	"set_cookie":     {},
	"setcookie":      {},
	"api_key":        {},
	"apikey":         {},
	"access_token":   {},
	"accesstoken":    {},
	"refresh_token":  {},
	"refreshtoken":   {},
	"auth_token":     {},
	"authtoken":      {},
	"jwt_token":      {},
	"jwttoken":       {},
	"id_token":       {},
	"idtoken":        {},
	"csrf_token":     {},
	"csrftoken":      {},
	"xsrf_token":     {},
	"xsrftoken":      {},
	"private_key":    {},
	"privatekey":     {},
	"priv_key":       {},
	"privkey":        {},
	"client_secret":  {},
	"clientsecret":   {},
	"credentials":    {},
	"credential":     {},
	"card_number":    {},
	"cardnumber":     {},
	"cvv":            {},
	"ssn":            {},
}

// sensitiveSuffixes contains suffixes indicating sensitive attributes.
var sensitiveSuffixes = []string{
	"_password",
	"-password",
	"_passwd",
	"-passwd",
	"_secret",
	"-secret",
	"_token",
	"-token",
	"_api_key",
	"-api_key",
	"_apikey",
	"-apikey",
	"_private_key",
	"-private_key",
	"_priv_key",
	"-priv_key",
	"_session_id",
	"-session_id",
	"_sessionid",
	"-sessionid",
	"_session",
	"-session",
	"_credentials",
	"-credentials",
	"_credential",
	"-credential",
}

// sensitivePrefixes contains prefixes indicating sensitive attributes.
var sensitivePrefixes = []string{
	"password_",
	"password-",
	"passwd_",
	"passwd-",
	"secret_",
	"secret-",
	"token_",
	"token-",
	"api_key_",
	"api_key-",
	"apikey_",
	"apikey-",
	"private_key_",
	"private_key-",
	"priv_key_",
	"priv_key-",
	"session_",
	"session-",
	"session_id_",
	"session_id-",
	"sessionid_",
	"sessionid-",
	"auth_",
	"auth-",
	"credentials_",
	"credentials-",
	"credential_",
	"credential-",
}

// sensitiveSegments contains individual token words that indicate sensitive attributes when present as delimited segments.
var sensitiveSegments = map[string]struct{}{
	"password":    {},
	"passwd":      {},
	"secret":      {},
	"token":       {},
	"tokens":      {},
	"credential":  {},
	"credentials": {},
	"cvv":         {},
	"ssn":         {},
}

// camelToSnake converts camelCase / PascalCase strings into lower_snake_case.
func camelToSnake(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				b.WriteByte('_')
			} else if i+1 < len(runes) && prev >= 'A' && prev <= 'Z' && runes[i+1] >= 'a' && runes[i+1] <= 'z' {
				b.WriteByte('_')
			}
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// hasSensitiveSegment checks if any delimited segment (split by _, -, ., /) matches a sensitive keyword.
func hasSensitiveSegment(s string) bool {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '/'
	})
	for _, part := range parts {
		if _, exists := sensitiveSegments[part]; exists {
			return true
		}
	}
	return false
}

// IsSensitiveHeader checks if an HTTP header name is sensitive (case-insensitive).
func IsSensitiveHeader(header string) bool {
	lower := strings.ToLower(strings.TrimSpace(header))
	if lower == "" {
		return false
	}
	if _, exists := sensitiveHeaders[lower]; exists {
		return true
	}
	if IsSensitiveKey(header) {
		return true
	}
	if strings.HasPrefix(lower, "x-") {
		trimmed := strings.TrimPrefix(lower, "x-")
		if IsSensitiveKey(trimmed) {
			return true
		}
	}
	return false
}

// SanitizeHTTPHeaders returns a sanitized copy of HTTP headers with sensitive values masked.
func SanitizeHTTPHeaders(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}
	sanitized := make(http.Header, len(headers))
	for k, v := range headers {
		if IsSensitiveHeader(k) {
			sanitized[k] = []string{RedactedValue}
		} else {
			cp := make([]string, len(v))
			copy(cp, v)
			sanitized[k] = cp
		}
	}
	return sanitized
}

// IsSensitiveKey checks whether an attribute key name represents sensitive information.
func IsSensitiveKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	clean := strings.ToLower(trimmed)

	// 1. Direct exact match
	if _, exists := exactSensitiveKeys[clean]; exists {
		return true
	}

	// 2. Normalized separator match (e.g., session-id -> session_id)
	normalized := strings.ReplaceAll(clean, "-", "_")
	if _, exists := exactSensitiveKeys[normalized]; exists {
		return true
	}

	// 3. Compact match for camelCase / mixed conversions (e.g., sessionId -> sessionid, apiKey -> apikey)
	compact := strings.ReplaceAll(strings.ReplaceAll(clean, "_", ""), "-", "")
	if _, exists := exactSensitiveKeys[compact]; exists {
		return true
	}

	// 4. CamelCase conversion match (e.g. accessToken -> access_token, clientSecret -> client_secret)
	snake := camelToSnake(trimmed)
	if snake != clean {
		if _, exists := exactSensitiveKeys[snake]; exists {
			return true
		}
		snakeNorm := strings.ReplaceAll(snake, "-", "_")
		if _, exists := exactSensitiveKeys[snakeNorm]; exists {
			return true
		}
	}

	// 5. Suffix match for compound field names (e.g., user_password, client_secret, db-password)
	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(clean, suffix) || (snake != clean && strings.HasSuffix(snake, suffix)) {
			return true
		}
	}

	// 6. Prefix match for compound field names (e.g., secret_key, session_id_l2, auth_code)
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(clean, prefix) || (snake != clean && strings.HasPrefix(snake, prefix)) {
			return true
		}
	}

	// 7. Infix multi-word compound patterns
	if strings.Contains(normalized, "_session_id_") ||
		strings.Contains(normalized, "_private_key_") ||
		strings.Contains(normalized, "_api_key_") ||
		strings.Contains(normalized, "_secret_key_") ||
		strings.Contains(normalized, "_client_secret_") {
		return true
	}
	if snake != clean {
		snakeNorm := strings.ReplaceAll(snake, "-", "_")
		if strings.Contains(snakeNorm, "_session_id_") ||
			strings.Contains(snakeNorm, "_private_key_") ||
			strings.Contains(snakeNorm, "_api_key_") ||
			strings.Contains(snakeNorm, "_secret_key_") ||
			strings.Contains(snakeNorm, "_client_secret_") {
			return true
		}
	}

	// 8. Delimited segment match for individual sensitive keywords (e.g. user_password_hash, client_secret_id, db_password_plaintext)
	if hasSensitiveSegment(clean) || (snake != clean && hasSensitiveSegment(snake)) {
		return true
	}

	return false
}

// SanitizeString inspects a string value for sensitive patterns (e.g. Bearer tokens, Basic auth).
func SanitizeString(val string) string {
	lower := strings.ToLower(val)
	if strings.HasPrefix(lower, "bearer ") {
		return "Bearer " + RedactedValue
	}
	if strings.HasPrefix(lower, "basic ") {
		return "Basic " + RedactedValue
	}
	return val
}

// SanitizeSpanAttribute sanitizes an OpenTelemetry span attribute.
func SanitizeSpanAttribute(kv attribute.KeyValue) attribute.KeyValue {
	kStr := string(kv.Key)
	if IsSensitiveKey(kStr) || IsSensitiveHeader(kStr) || strings.HasPrefix(kStr, "http.request.header.") {
		// Check sub-header key if prefixed
		headerName := strings.TrimPrefix(kStr, "http.request.header.")
		if IsSensitiveHeader(headerName) || IsSensitiveKey(kStr) || IsSensitiveKey(headerName) {
			return attribute.String(kStr, RedactedValue)
		}
	}

	if kv.Value.Type() == attribute.STRING {
		strVal := kv.Value.AsString()
		if sanitized := SanitizeString(strVal); sanitized != strVal {
			return attribute.String(kStr, sanitized)
		}
	}

	return kv
}

// SanitizeSpanAttributes sanitizes a slice of OpenTelemetry span attributes.
func SanitizeSpanAttributes(attrs []attribute.KeyValue) []attribute.KeyValue {
	if attrs == nil {
		return nil
	}
	sanitized := make([]attribute.KeyValue, len(attrs))
	for i, kv := range attrs {
		sanitized[i] = SanitizeSpanAttribute(kv)
	}
	return sanitized
}

// SanitizeAttr masks sensitive slog.Attr keys and nested group attributes.
func SanitizeAttr(attr slog.Attr) slog.Attr {
	return RedactSensitiveSlogAttrs(nil, attr)
}

// SanitizeAttrs sanitizes a slice of slog attributes.
func SanitizeAttrs(attrs []slog.Attr) []slog.Attr {
	if attrs == nil {
		return nil
	}
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		sanitized[i] = SanitizeAttr(a)
	}
	return sanitized
}

// RedactSensitiveSlogAttrs is a slog.HandlerOptions ReplaceAttr function that masks sensitive attributes.
func RedactSensitiveSlogAttrs(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "" {
		return a
	}

	val := a.Value.Resolve()

	// Handle nested attribute groups (including groups resolved from slog.LogValuer)
	if val.Kind() == slog.KindGroup {
		groupAttrs := val.Group()
		sanitized := make([]slog.Attr, len(groupAttrs))
		for i, inner := range groupAttrs {
			sanitized[i] = RedactSensitiveSlogAttrs(append(groups, a.Key), inner)
		}
		return slog.Attr{
			Key:   a.Key,
			Value: slog.GroupValue(sanitized...),
		}
	}

	// Check if the key is sensitive
	if IsSensitiveKey(a.Key) {
		return slog.String(a.Key, RedactedValue)
	}

	// Check if the string value contains sensitive authentication prefixes
	if val.Kind() == slog.KindString {
		strVal := val.String()
		if sanitized := SanitizeString(strVal); sanitized != strVal {
			return slog.String(a.Key, sanitized)
		}
	}

	return slog.Attr{
		Key:   a.Key,
		Value: val,
	}
}

// SanitizingHandler wraps an existing slog.Handler and redacts sensitive attributes.
type SanitizingHandler struct {
	next slog.Handler
}

// NewSanitizingHandler creates a new SanitizingHandler wrapping next.
func NewSanitizingHandler(next slog.Handler) *SanitizingHandler {
	return &SanitizingHandler{next: next}
}

// Enabled implements slog.Handler.
func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (h *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(RedactSensitiveSlogAttrs(nil, a))
		return true
	})
	if err := h.next.Handle(ctx, newRecord); err != nil {
		return fmt.Errorf("handle sanitized record: %w", err)
	}
	return nil
}

// WithAttrs implements slog.Handler.
func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		sanitized[i] = RedactSensitiveSlogAttrs(nil, a)
	}
	return &SanitizingHandler{next: h.next.WithAttrs(sanitized)}
}

// WithGroup implements slog.Handler.
func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{next: h.next.WithGroup(name)}
}
