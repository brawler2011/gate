package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestErrorResponseUsesContextIDWithoutLoggingErrorObjects(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf)
	h := RequestLoggerMiddleware(logger, func(*http.Request) string { return "unmatched" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ResponseErrorHandler(logger)(r.Context(), w, r, errors.New("query-secret Bearer password-secret"))
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/secret?password=secret", nil))
	var body errorResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, rr.Header().Get(requestid.Header), body.RequestID)
	require.NotContains(t, buf.String(), "password-secret")
	require.NotContains(t, buf.String(), "query-secret")
	require.Equal(t, 1, strings.Count(buf.String(), `"event":"http_request_completed"`))
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
