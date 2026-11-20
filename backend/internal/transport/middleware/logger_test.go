package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestCompletion(t *testing.T) {
	for _, status := range []int{0, 200, 204, 401, 404, 500} {
		for _, id := range []string{"", "valid.ID:123_abc-def", strings.Repeat("a", 129), "secret\nvalue", "секрет", "invalid id"} {
			t.Run(http.StatusText(status)+id, func(t *testing.T) {
				var buf bytes.Buffer
				h := RequestLoggerMiddleware(logging.New(&buf), func(*http.Request) string { return "/submissions/{id}" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.True(t, requestid.Valid(requestid.Get(r.Context())))
					assert.Equal(t, w.Header().Get(requestid.Header), requestid.Get(r.Context()))
					if status != 0 {
						w.WriteHeader(status)
						w.WriteHeader(503)
					}
				}))
				req := httptest.NewRequest("GET", "/submissions/path-secret?token=query-secret", nil)
				req.Header.Set(requestid.Header, id)
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, req)
				expected := status
				if expected == 0 {
					expected = 200
				}
				require.Equal(t, expected, rr.Code)
				require.Equal(t, 1, strings.Count(buf.String(), "\n"))
				require.NotContains(t, buf.String(), "path-secret")
				require.NotContains(t, buf.String(), "query-secret")
				var record map[string]interface{}
				require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
				require.Equal(t, "http_request_completed", record["event"])
				require.Equal(t, "/submissions/{id}", record["route"])
				require.InDelta(t, float64(expected), record["status"], 0)
				require.Contains(t, record, "duration_ms")
				if requestid.Valid(id) {
					require.Equal(t, id, record["request_id"])
				} else {
					require.NotEqual(t, id, record["request_id"])
				}
			})
		}
	}
}

func TestErrorResponse(t *testing.T) {
	internalErr := pkg.Wrap(pkg.ErrInternal, errors.New("pg: relation missing"), "unexpected postgres error")
	for _, tc := range []struct {
		name    string
		err     error
		cancel  bool
		status  int
		code    corev1.ErrorCode
		message string
		level   string
	}{
		{"internal error is logged but hidden", internalErr, false, 500, corev1.ErrorCodeInternal, "", "ERROR"},
		{"raw error is logged but hidden", errors.New("open /tmp/x: permission denied"), false, 500, corev1.ErrorCodeInternal, "", "ERROR"},
		{"client error message is public", pkg.Wrap(pkg.ErrBadInput, nil, "empty title"), false, 400, corev1.ErrorCodeInvalid, "empty title", "WARN"},
		{"client disconnect", fmt.Errorf("query: %w", context.Canceled), true, 499, corev1.ErrorCodeInternal, "", "INFO"},
		{"server-side cancellation", fmt.Errorf("query: %w", context.Canceled), false, 500, corev1.ErrorCodeInternal, "", "ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := logging.New(&buf)
			h := RequestLoggerMiddleware(logger, func(*http.Request) string { return "/x" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logger.InfoContext(r.Context(), "inside")
				ctx := r.Context()
				if tc.cancel {
					cctx, cancel := context.WithCancel(ctx)
					cancel()
					r = r.WithContext(cctx)
				}
				ResponseErrorHandler(logger)(r.Context(), w, r, tc.err)
			}))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest("GET", "/x", nil))
			require.Equal(t, tc.status, rr.Code)
			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			require.Len(t, lines, 2)
			var inside, record map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[0]), &inside))
			require.NoError(t, json.Unmarshal([]byte(lines[1]), &record))
			id := rr.Header().Get(requestid.Header)
			require.Equal(t, id, inside["request_id"])
			require.Equal(t, id, record["request_id"])
			require.Equal(t, tc.err.Error(), record["error"])
			require.Equal(t, tc.level, record["level"])
			if tc.status == 499 {
				require.Empty(t, rr.Body.String())
				return
			}
			require.Equal(t, "application/problem+json", rr.Header().Get("Content-Type"))
			var body corev1.Problem
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			require.Equal(t, tc.message, body.Detail.Or(""))
			require.Equal(t, tc.code, body.Code)
			require.Equal(t, tc.status, body.Status)
			require.Equal(t, id, body.RequestID)
		})
	}
}

func TestErrorAfterResponseStarted(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf)
	h := RequestLoggerMiddleware(logger, func(*http.Request) string { return "/x" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial"))
		ResponseErrorHandler(logger)(r.Context(), w, r, errors.New("s3 read failed"))
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, "partial", rr.Body.String())
	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	require.InDelta(t, 200, record["status"], 0)
	require.Equal(t, "s3 read failed", record["error"])
}

func TestRequestLoggerStreaming(t *testing.T) {
	var buf bytes.Buffer
	flushed := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	h := RequestLoggerMiddleware(logging.New(&buf), func(*http.Request) string { return "/files/*" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		_, err := w.Write([]byte("first\n"))
		assert.NoError(t, err)
		flusher, ok := w.(http.Flusher)
		if !assert.True(t, ok) {
			return
		}
		flusher.Flush()
		close(flushed)
		<-release
		_, err = w.Write([]byte("last\n"))
		assert.NoError(t, err)
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	<-flushed
	first := make([]byte, 6)
	_, err = io.ReadFull(resp.Body, first)
	require.NoError(t, err)
	require.Equal(t, "first\n", string(first))
	close(release)
	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "last\n", string(rest))
	require.NoError(t, resp.Body.Close())
	<-done
}

func TestRequestLoggerWebSocket(t *testing.T) {
	var buf bytes.Buffer
	done := make(chan struct{})
	upgrader := websocket.Upgrader{}
	h := RequestLoggerMiddleware(logging.New(&buf), func(*http.Request) string { return "/ws/*" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, http.Header{requestid.Header: w.Header().Values(requestid.Header)})
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		kind, data, err := conn.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		assert.NoError(t, conn.WriteMessage(kind, data))
	}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r); close(done) }))
	defer srv.Close()
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/secret?token=secret", nil)
	require.NoError(t, err)
	require.Equal(t, 101, resp.StatusCode)
	require.True(t, requestid.Valid(resp.Header.Get(requestid.Header)))
	require.NoError(t, resp.Body.Close())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("hello")))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))
	require.NoError(t, conn.Close())
	<-done
	require.Equal(t, 1, strings.Count(buf.String(), "\n"))
	require.Contains(t, buf.String(), `"status":101`)
	require.NotContains(t, buf.String(), "secret")
}

func TestRequestIDDetachedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(requestid.With(context.Background(), "request-123"))
	cancel()
	require.Equal(t, "request-123", requestid.Get(context.WithoutCancel(ctx)))
}

func TestPanicAfterFlushAbortsResponse(t *testing.T) {
	var buf bytes.Buffer
	srv := httptest.NewServer(RequestLoggerMiddleware(logging.New(&buf), func(*http.Request) string { return "/x" })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		panic("boom")
	})))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.Error(t, err)
	require.Contains(t, buf.String(), `"level":"ERROR"`)
	require.Contains(t, buf.String(), `"panic":"boom"`)
}

type brokenWriter struct{ *httptest.ResponseRecorder }

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestErrorResponseWriteFailureStaysOnRequestLine(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf)
	h := RequestLoggerMiddleware(logger, func(*http.Request) string { return "/x" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ResponseErrorHandler(logger)(r.Context(), w, r, pkg.Wrap(pkg.ErrBadInput, nil, "empty title"))
	}))
	h.ServeHTTP(brokenWriter{httptest.NewRecorder()}, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, 1, strings.Count(buf.String(), "\n"))
	require.Contains(t, buf.String(), "broken pipe")
}

func TestRequestLoggerPanic(t *testing.T) {
	var buf bytes.Buffer
	h := RequestLoggerMiddleware(logging.New(&buf), func(*http.Request) string { return "unmatched" })(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/path-secret?token=query-secret", nil))
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, 1, strings.Count(buf.String(), "\n"))
	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	require.InDelta(t, http.StatusInternalServerError, record["status"], 0)
	require.Equal(t, "boom", record["panic"])
	require.Contains(t, record["stack"], "logger_test.go")
	require.NotContains(t, buf.String(), "secret")
}
