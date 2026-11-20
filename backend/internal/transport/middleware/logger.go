package middleware

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/felixge/httpsnoop"
	"github.com/google/uuid"
)

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
						if err := writeProblem(sw, newProblem(ctx, errors.New("panic"))); err != nil {
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
