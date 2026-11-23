package middleware

import (
	"log/slog"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRateLimit(t *testing.T) {
	for _, tc := range []struct {
		name, path    string
		limit         int
		authenticated bool
	}{
		{"anonymous", "/problems", 300, false},
		{"user", "/problems", 600, true},
		{"submissions", "/submissions", 60, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := RateLimit(ratelimit.New(nil), "", ResponseErrorHandler(slog.Default()))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			ctx := context.Background()
			if tc.authenticated {
				ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: uuid.New()})
			}
			send := func(addr string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, tc.path, nil).WithContext(ctx)
				req.RemoteAddr = addr
				req.Header.Set("X-Forwarded-For", uuid.NewString())
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				return rr
			}
			for range tc.limit {
				require.Equal(t, http.StatusNoContent, send("192.0.2.1:1234").Code)
			}
			rr := send("192.0.2.1:5678")
			require.Equal(t, http.StatusTooManyRequests, rr.Code)
			require.NotEmpty(t, rr.Header().Get("Retry-After"))
			require.Equal(t, "application/problem+json", rr.Header().Get("Content-Type"))
			var body map[string]any
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			require.Equal(t, "rate_limited", body["code"])
			if tc.authenticated {
				require.Equal(t, http.StatusTooManyRequests, send("192.0.2.2:1234").Code)
				ctx = accessmodel.WithUser(ctx, accessmodel.User{Id: uuid.New()})
				require.Equal(t, http.StatusNoContent, send("192.0.2.1:1234").Code)
			} else {
				require.Equal(t, http.StatusNoContent, send("192.0.2.2:1234").Code)
			}
		})
	}
}
