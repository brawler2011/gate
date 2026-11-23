package middleware

import (
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/ratelimit"
)

// Run after UsersMiddleware so authenticated quotas follow users across connections.
func RateLimit(limiter *ratelimit.Limiter, trustedProxy string, onError corev1.ErrorHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := accessmodel.UserFromContext(r.Context())
			key, limit := "api:user:"+user.Id.String(), 600
			if user.IsGuest() {
				key, limit = "api:ip:"+rateLimitClientIP(r, trustedProxy), 300
			}
			err := limiter.Allow(r.Context(), key, ratelimit.Quota{Limit: limit, Window: time.Minute})
			if err == nil && r.Method == http.MethodPost && r.URL.Path == "/submissions" && !user.IsGuest() {
				err = limiter.Allow(r.Context(), "submissions:"+user.Id.String(), ratelimit.SubmissionQuotas...)
			}
			if err != nil {
				onError(r.Context(), w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
