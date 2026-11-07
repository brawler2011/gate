package middleware

import (
	"net/http"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
)

// RequestLimits bounds reads before ogen decodes JSON or parses multipart forms.
// It must run after the session/user middleware and before the generated server.
func RequestLimits(server *corev1.Server, bodyLimit func(string) int64, onError corev1.ErrorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := server.FindPath(r.Method, r.URL)
		if !ok {
			server.ServeHTTP(w, r)
			return
		}
		limit := bodyLimit(route.OperationID())
		if r.ContentLength > limit {
			onError(r.Context(), w, r, &http.MaxBytesError{Limit: limit})
			return
		}
		if err := authorizeBeforeBody(r.Context(), route.OperationID()); err != nil {
			onError(r.Context(), w, r, err)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		server.ServeHTTP(w, r)
	})
}
