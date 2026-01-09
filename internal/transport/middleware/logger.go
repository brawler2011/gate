package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type errorResponse struct {
	Err       string `json:"error"`
	Msg       string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

const (
	requestIDKey contextKey = "request_id"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

// RequestLoggerMiddleware logs all incoming requests with timing and context
func RequestLoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate or retrieve request Id
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}
			w.Header().Set("X-Request-ID", requestID)

			// Store requestID in context so it propagates to the entire app
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)
			statusCode := sw.status
			if statusCode == 0 {
				statusCode = 200
			}

			// Build log message
			logMsg := fmt.Sprintf("%s %s -> %d %s (%dms)",
				r.Method,
				r.URL.Path,
				statusCode,
				http.StatusText(statusCode),
				duration.Milliseconds(),
			)

			// Build log attributes
			logAttrs := []any{
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", statusCode),
				slog.Int64("duration_ms", duration.Milliseconds()),
				slog.String("ip", r.RemoteAddr),
			}

			if len(r.URL.Query()) > 0 {
				logAttrs = append(logAttrs, slog.Any("query", r.URL.Query()))
			}

			// Log based on status code
			logLevel := slog.LevelInfo
			if statusCode >= 500 {
				logLevel = slog.LevelError
			} else if statusCode >= 400 {
				logLevel = slog.LevelWarn
			}
			logger.Log(ctx, logLevel, logMsg, logAttrs...)
		})
	}
}

// ResponseErrorHandler handles errors returned by strict handlers
func ResponseErrorHandler(logger *slog.Logger) func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		// Retrieve Request Id from context
		ctx := r.Context()
		requestID, _ := ctx.Value(requestIDKey).(string)
		if requestID == "" {
			requestID = r.Header.Get("X-Request-ID")
		}

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
			resp.Msg = err.Error()
		}

		logMsg := fmt.Sprintf("%s %s -> %d %s",
			r.Method,
			r.URL.Path,
			statusCode,
			http.StatusText(statusCode),
		)

		logAttrs := []any{
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", statusCode),
			slog.String("error", err.Error()),
			slog.String("ip", r.RemoteAddr),
		}

		// Add user/session context if available in the context
		if user := GetUser(ctx); !user.IsGuest() {
			logAttrs = append(logAttrs, slog.String("user_id", user.Id.String()))
		}
		if session, _ := getSession(ctx); session != nil {
			logAttrs = append(logAttrs, slog.String("session_id", session.Id))
		}

		if cErr != nil {
			resp.Msg = cErr.Message
			logAttrs = append(logAttrs,
				slog.String("operation", cErr.Op),
				slog.String("message", cErr.Message),
			)
			if cErr.Basic != nil {
				logAttrs = append(logAttrs, slog.String("error_type", fmt.Sprintf("%T", cErr.Basic)))
			}
			if cErr.Cause != nil {
				logAttrs = append(logAttrs, slog.String("cause", fmt.Sprintf("%v", cErr.Cause)))
			}
		}

		logLevel := slog.LevelError
		if statusCode < 500 {
			logLevel = slog.LevelWarn
		}
		if statusCode == http.StatusNotFound {
			logLevel = slog.LevelInfo
		}
		logger.Log(ctx, logLevel, logMsg, logAttrs...)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}
}
