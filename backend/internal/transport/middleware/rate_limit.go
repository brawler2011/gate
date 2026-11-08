package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
)

// RateLimit runs after UsersMiddleware, so quotas follow authenticated users
// across connections. Anonymous quotas use the peer IP, never untrusted headers.
func RateLimit(limiter *ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := accessmodel.UserFromContext(r.Context())
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			key, limit := "rl:api:ip:"+ip, 300
			if !user.IsGuest() {
				key, limit = "rl:api:user:"+user.Id.String(), 600
			}
			err = limiter.Allow(r.Context(), key, ratelimit.Quota{Limit: limit, Window: time.Minute})
			if err == nil && r.Method == http.MethodPost && r.URL.Path == "/submissions" && !user.IsGuest() {
				err = limiter.Allow(r.Context(), "rl:sub:"+user.Id.String(), ratelimit.SubmissionQuotas...)
			}
			if err != nil {
				ResponseErrorHandler(slog.Default())(r.Context(), w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
