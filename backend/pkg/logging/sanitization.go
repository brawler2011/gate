package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

const RedactedValue = "[REDACTED]"

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

var exactSensitiveKeys = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"pass_word":     {},
	"password_hash": {},
	"passwordhash":  {},
	"secret":        {},
	"token":         {},
	"session_id":    {},
	"sessionid":     {},
	"session":       {},
	"authorization": {},
	"auth":          {},
	"auth_code":     {},
	"authcode":      {},
	"cookie":        {},
	"set_cookie":    {},
	"setcookie":     {},
	"api_key":       {},
	"apikey":        {},
	"access_token":  {},
	"accesstoken":   {},
	"refresh_token": {},
	"refreshtoken":  {},
	"auth_token":    {},
	"authtoken":     {},
	"jwt_token":     {},
	"jwttoken":      {},
	"id_token":      {},
	"idtoken":       {},
	"csrf_token":    {},
	"csrftoken":     {},
	"xsrf_token":    {},
	"xsrftoken":     {},
	"private_key":   {},
	"privatekey":    {},
	"priv_key":      {},
	"privkey":       {},
	"client_secret": {},
	"clientsecret":  {},
	"credentials":   {},
	"credential":    {},
	"card_number":   {},
	"cardnumber":    {},
	"cvv":           {},
	"ssn":           {},
}

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

func IsSensitiveKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	clean := strings.ToLower(trimmed)

	if _, exists := exactSensitiveKeys[clean]; exists {
		return true
	}

	normalized := strings.ReplaceAll(clean, "-", "_")
	if _, exists := exactSensitiveKeys[normalized]; exists {
		return true
	}

	compact := strings.ReplaceAll(strings.ReplaceAll(clean, "_", ""), "-", "")
	if _, exists := exactSensitiveKeys[compact]; exists {
		return true
	}

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

	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(clean, suffix) || (snake != clean && strings.HasSuffix(snake, suffix)) {
			return true
		}
	}

	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(clean, prefix) || (snake != clean && strings.HasPrefix(snake, prefix)) {
			return true
		}
	}

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

	if hasSensitiveSegment(clean) || (snake != clean && hasSensitiveSegment(snake)) {
		return true
	}

	return false
}

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

func SanitizeAttr(attr slog.Attr) slog.Attr {
	return RedactSensitiveSlogAttrs(nil, attr)
}

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

func RedactSensitiveSlogAttrs(groups []string, a slog.Attr) slog.Attr {
	if IsSensitiveKey(a.Key) {
		return slog.String(a.Key, RedactedValue)
	}

	val := a.Value.Resolve()
	for _, group := range groups {
		if IsSensitiveKey(group) {
			return slog.String(a.Key, RedactedValue)
		}
	}

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

type SanitizingHandler struct {
	next   slog.Handler
	groups []string
}

func NewSanitizingHandler(next slog.Handler) *SanitizingHandler {
	return &SanitizingHandler{next: next}
}

func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(RedactSensitiveSlogAttrs(h.groups, a))
		return true
	})
	if err := h.next.Handle(ctx, newRecord); err != nil {
		return fmt.Errorf("handle sanitized record: %w", err)
	}
	return nil
}

func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		sanitized[i] = RedactSensitiveSlogAttrs(h.groups, a)
	}
	return &SanitizingHandler{next: h.next.WithAttrs(sanitized), groups: h.groups}
}

func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	groups := append(append([]string(nil), h.groups...), name)
	return &SanitizingHandler{next: h.next.WithGroup(name), groups: groups}
}
