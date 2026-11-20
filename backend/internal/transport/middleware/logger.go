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
	"runtime/debug"
	"strconv"
	"time"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/felixge/httpsnoop"
	"github.com/google/uuid"
	"github.com/ogen-go/ogen/ogenerrors"
)

type errorResponse struct {
	Err       string          `json:"error"`
	Msg       string          `json:"message"`
	RequestID string          `json:"request_id,omitempty"`
	Fields    pkg.FieldErrors `json:"fields,omitempty"`
}

// nginx's code for a client that closed the connection before the response.
const statusClientClosedRequest = 499

// requestLog lets the error handler attach its error to the request's single log line.
type requestLog struct {
	err     error
	written bool
}

type requestLogKey struct{}

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
			state := &requestLog{}
			ctx := context.WithValue(requestid.With(r.Context(), id), requestLogKey{}, state)
			r = r.WithContext(ctx)
			status := http.StatusOK
			sw := httpsnoop.Wrap(w, httpsnoop.Hooks{
				WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
					return func(code int) {
						if !state.written && (code == 101 || code >= 200) {
							status = code
							state.written = true
						}
						next(code)
					}
				},
				Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
					return func(p []byte) (int, error) { state.written = true; return next(p) }
				},
				ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
					return func(r io.Reader) (int64, error) { state.written = true; return next(r) }
				},
				Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
					return func() { state.written = true; next() }
				},
				Hijack: func(next httpsnoop.HijackFunc) httpsnoop.HijackFunc {
					return func() (net.Conn, *bufio.ReadWriter, error) {
						conn, rw, err := next()
						if err == nil {
							status = http.StatusSwitchingProtocols
							state.written = true
						}
						return conn, rw, err
					}
				},
			})
			defer func() {
				attrs := []slog.Attr{
					slog.String("event", "http_request_completed"),
					slog.String("method", r.Method),
					slog.String("route", route(r)),
				}
				recovered := recover()
				recoveredErr, _ := recovered.(error)
				abort := errors.Is(recoveredErr, http.ErrAbortHandler)
				committed := state.written
				if recovered != nil {
					attrs = append(attrs, slog.String("panic", fmt.Sprint(recovered)), slog.String("stack", string(debug.Stack())))
					if !committed && !abort {
						if err := writeError(sw, http.StatusInternalServerError, internalMessage, id, nil); err != nil {
							state.err = errors.Join(state.err, err)
						}
					}
				}
				if state.err != nil {
					attrs = append(attrs, slog.String("error", state.err.Error()))
				}
				attrs = append(attrs, slog.Int("status", status), slog.Int64("duration_ms", time.Since(start).Milliseconds()))
				level := statusLevel(status)
				if recovered != nil && !abort {
					level = slog.LevelError
				}
				logger.LogAttrs(ctx, level, "http_request_completed", attrs...)
				switch {
				case abort:
					panic(recovered)
				case recovered != nil && committed:
					// Without an abort the server would end a truncated response as if it were complete.
					panic(http.ErrAbortHandler)
				}
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

func statusLevel(status int) slog.Level {
	switch {
	case status == statusClientClosedRequest:
		return slog.LevelInfo
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

const internalMessage = "internal server error"

func ResponseErrorHandler(logger *slog.Logger) func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		if ctx == nil {
			ctx = r.Context()
		}
		var statusCode int
		msg := ""

		var cErr *pkg.CustomError
		var sizeErr *http.MaxBytesError
		var rateErr *pkg.RateLimitError
		switch {
		case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
			statusCode = statusClientClosedRequest
		case errors.As(err, &rateErr):
			statusCode = http.StatusTooManyRequests
			msg = rateErr.Error()
			seconds := max(1, int64((rateErr.RetryAfter+time.Second-1)/time.Second))
			w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
		case errors.As(err, &sizeErr):
			statusCode = http.StatusRequestEntityTooLarge
			msg = "request body exceeds maximum allowed size"
		case errors.As(err, &cErr):
			statusCode = pkg.ToREST(err)
			msg = cErr.Message
		default:
			statusCode = pkg.ToREST(err)
			if statusCode == http.StatusInternalServerError {
				if code := ogenerrors.ErrorCode(err); code != http.StatusInternalServerError {
					statusCode = code
				}
			}
			msg = err.Error()
		}
		if statusCode >= 500 {
			msg = internalMessage
		}

		state, _ := ctx.Value(requestLogKey{}).(*requestLog)
		if state == nil {
			logger.LogAttrs(ctx, statusLevel(statusCode), "http_request_error", slog.Int("status", statusCode), slog.String("error", err.Error()))
		} else {
			state.err = err
			if state.written {
				return
			}
		}

		var fields pkg.FieldErrors
		if !errors.As(err, &fields) {
			fields = nil
		}
		if writeErr := writeError(w, statusCode, msg, requestid.Get(ctx), fields); writeErr != nil {
			if state != nil {
				state.err = errors.Join(state.err, writeErr)
			} else {
				logger.ErrorContext(ctx, "failed to write error response", "error", writeErr)
			}
		}
	}
}

func writeError(w http.ResponseWriter, status int, msg, requestID string, fields pkg.FieldErrors) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status == statusClientClosedRequest {
		return nil
	}
	if err := json.NewEncoder(w).Encode(errorResponse{Err: http.StatusText(status), Msg: msg, RequestID: requestID, Fields: fields}); err != nil {
		return fmt.Errorf("write error response: %w", err)
	}
	return nil
}
