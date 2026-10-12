package middleware

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/felixge/httpsnoop"
	"github.com/google/uuid"
	"github.com/ogen-go/ogen/ogenerrors"
)

type errorResponse struct {
	Err       string `json:"error"`
	Msg       string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// RequestLoggerMiddleware preserves the writer's optional interfaces for streaming and upgrades.
func RequestLoggerMiddleware(logger *slog.Logger, route func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := r.Header.Get(requestid.Header)
			if !requestid.Valid(id) {
				id = uuid.NewString()
			}
			w.Header().Set(requestid.Header, id)
			ctx := requestid.With(r.Context(), id)
			r = r.WithContext(ctx)
			status := http.StatusOK
			written := false
			sw := httpsnoop.Wrap(w, httpsnoop.Hooks{
				WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
					return func(code int) {
						if !written && (code == 101 || code >= 200) {
							status = code
							written = true
						}
						next(code)
					}
				},
				Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
					return func(p []byte) (int, error) { written = true; return next(p) }
				},
				ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
					return func(r io.Reader) (int64, error) { written = true; return next(r) }
				},
				Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
					return func() { written = true; next() }
				},
				Hijack: func(next httpsnoop.HijackFunc) httpsnoop.HijackFunc {
					return func() (net.Conn, *bufio.ReadWriter, error) {
						conn, rw, err := next()
						if err == nil {
							status = http.StatusSwitchingProtocols
							written = true
						}
						return conn, rw, err
					}
				},
			})
			completed := false
			defer func() {
				if !completed && !written {
					status = http.StatusInternalServerError
				}
				level := slog.LevelInfo
				if status >= 500 {
					level = slog.LevelError
				} else if status >= 400 {
					level = slog.LevelWarn
				}
				logger.Log(ctx, level, "http_request_completed", "event", "http_request_completed",
					"request_id", id, "method", r.Method, "route", route(r), "status", status, "duration_ms", time.Since(start).Milliseconds())
			}()
			next.ServeHTTP(sw, r)
			completed = true

		})
	}
}

// ResponseErrorHandler handles errors returned by handlers and ogen validation/decoding
func ResponseErrorHandler(logger *slog.Logger) func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		// Retrieve Request Id from context
		if ctx == nil {
			ctx = r.Context()
		}
		requestID := requestid.Get(ctx)

		statusCode := http.StatusInternalServerError
		resp := errorResponse{
			Err:       http.StatusText(statusCode),
			Msg:       "",
			RequestID: requestID,
		}

		var cErr *pkg.CustomError
		if errors.As(err, &cErr) {
			statusCode = pkg.ToREST(err)
			resp.Err = http.StatusText(statusCode)
			resp.Msg = cErr.Message
		} else {
			statusCode = pkg.ToREST(err)
			if statusCode == http.StatusInternalServerError {
				if code := ogenerrors.ErrorCode(err); code != http.StatusInternalServerError {
					statusCode = code
				}
			}
			resp.Err = http.StatusText(statusCode)
			resp.Msg = err.Error()
		}

		logAttrs := []any{
			slog.String("request_id", requestID),
			slog.String("operation", "http_response"),
			slog.Int("status", statusCode),
			slog.String("error_type", fmt.Sprintf("%T", err)),
		}

		logLevel := slog.LevelError
		if statusCode < 500 {
			logLevel = slog.LevelWarn
		}
		if statusCode == http.StatusNotFound {
			logLevel = slog.LevelInfo
		}
		logger.Log(ctx, logLevel, "http_request_error", logAttrs...)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.ErrorContext(ctx, "Failed to encode error response", "error", err)
		}
	}
}
