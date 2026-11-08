package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRateLimitClientIP(t *testing.T) {
	for _, tc := range []struct {
		name, trusted, peer string
		forwarded           []string
		want                string
	}{
		{"direct ignores spoof", "", "192.0.2.1:1234", []string{"203.0.113.1"}, "192.0.2.1"},
		{"untrusted peer", "192.0.2.254", "192.0.2.1:1234", []string{"203.0.113.1"}, "192.0.2.1"},
		{"trusted appended IP", "192.0.2.254", "192.0.2.254:1234", []string{"198.51.100.1, 203.0.113.1"}, "203.0.113.1"},
		{"multiple headers", "192.0.2.254", "192.0.2.254:1234", []string{"198.51.100.1", "203.0.113.1"}, "203.0.113.1"},
		{"invalid forwarded", "192.0.2.254", "192.0.2.254:1234", []string{"198.51.100.1, invalid"}, "192.0.2.254"},
		{"missing forwarded", "192.0.2.254", "192.0.2.254:1234", nil, "192.0.2.254"},
		{"IPv6", "::1", "[::1]:1234", []string{"2001:db8::1"}, "2001:db8::1"},
		{"mapped IPv4", "::ffff:192.0.2.254", "192.0.2.254:1234", []string{"::ffff:203.0.113.1"}, "203.0.113.1"},
		{"proxy DNS name", "localhost", "127.0.0.1:1234", []string{"203.0.113.1"}, "203.0.113.1"},
		{"DNS failure", "proxy.invalid", "192.0.2.254:1234", []string{"203.0.113.1"}, "192.0.2.254"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/health", nil)
			r.RemoteAddr = tc.peer
			for _, header := range tc.forwarded {
				r.Header.Add("X-Forwarded-For", header)
			}
			require.Equal(t, tc.want, rateLimitClientIP(r, tc.trusted))
		})
	}
}
