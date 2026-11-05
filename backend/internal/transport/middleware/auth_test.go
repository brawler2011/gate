package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCookies(t *testing.T) {
	sessionID := uuid.New()
	for _, operation := range []struct {
		name   string
		write  func(context.Context)
		value  string
		maxAge int
	}{
		{"set", func(ctx context.Context) { SetSessionCookie(ctx, sessionID) }, sessionID.String(), 7 * 24 * 60 * 60},
		{"clear", ClearSessionCookie, "", -1},
	} {
		t.Run(operation.name, func(t *testing.T) {
			for _, tt := range []struct {
				name           string
				env            string
				url            string
				forwardedProto string
				secure         bool
			}{
				{"local", "local", "http://example.com", "", false},
				{"prod", "prod", "http://example.com", "", true},
				{"staging", "staging", "http://example.com", "", true},
				{"empty", "", "http://example.com", "", true},
				{"uppercase", "LOCAL", "http://example.com", "", true},
				{"local_tls", "local", "https://example.com", "", false},
				{"prod_tls", "prod", "https://example.com", "", true},
				{"local_proxy_https", "local", "http://example.com", "https", false},
				{"prod_proxy_http", "prod", "https://example.com", "http", true},
			} {
				t.Run(tt.name, func(t *testing.T) {
					handler := SessionCookieMiddleware(tt.env != "local")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
						operation.write(r.Context())
					}))
					req := httptest.NewRequest(http.MethodGet, tt.url, nil)
					if tt.forwardedProto != "" {
						req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
						req.Header.Set("Forwarded", "proto="+tt.forwardedProto)
					}
					rr := httptest.NewRecorder()
					handler.ServeHTTP(rr, req)

					cookies := rr.Result().Cookies()
					require.Len(t, cookies, 1)
					cookie := cookies[0]
					assert.Equal(t, "session_id", cookie.Name)
					assert.Equal(t, operation.value, cookie.Value)
					assert.Equal(t, "/", cookie.Path)
					assert.True(t, cookie.HttpOnly)
					assert.Equal(t, tt.secure, cookie.Secure)
					assert.Equal(t, operation.maxAge, cookie.MaxAge)
					assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
					assert.Empty(t, cookie.Domain)
					assert.True(t, cookie.Expires.IsZero())
				})
			}
		})
	}
}
