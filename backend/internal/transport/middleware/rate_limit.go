package middleware

import (
	"log/slog"
	"net/http"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
)

// RateLimit runs after UsersMiddleware, so quotas follow authenticated users
// across connections. Forwarded client IPs are accepted only from the configured proxy.
func RateLimit(limiter *ratelimit.Limiter, trustedProxy string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := accessmodel.UserFromContext(r.Context())
			key, limit := "rl:api:user:"+user.Id.String(), 600
			if user.IsGuest() {
				key, limit = "rl:api:ip:"+rateLimitClientIP(r, trustedProxy), 300
			}
			err := limiter.Allow(r.Context(), key, ratelimit.Quota{Limit: limit, Window: time.Minute})
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
