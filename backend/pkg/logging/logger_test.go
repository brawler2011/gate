package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/stretchr/testify/require"
)

func TestJSONWithSensitiveGroupsAndLogValuer(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(&buf).With(slog.Group("credentials", slog.String("value", "group-secret"))).WithGroup("operation")
	logger.Info("safe message", slog.Group("", slog.String("password", "anonymous-secret")), slog.Any("value", testCustomLogValuer{password: "superSecret123"}))
	for _, secret := range []string{"group-secret", "anonymous-secret", "superSecret123"} {
		require.NotContains(t, buf.String(), secret)
	}
	var record map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	require.Contains(t, record, "time")
	require.Contains(t, record, "operation")
	buf.Reset()
	logging.New(&buf).WithGroup("authorization").With("value", "with-secret").Info("safe message", "other", "record-secret")
	require.NotContains(t, buf.String(), "with-secret")
	require.NotContains(t, buf.String(), "record-secret")
}

func TestRequestIDFromContext(t *testing.T) {
	ctx := requestid.With(context.Background(), "context-id")
	for name, log := range map[string]func(*slog.Logger){
		"context only":     func(l *slog.Logger) { l.InfoContext(ctx, "m") },
		"record attribute": func(l *slog.Logger) { l.InfoContext(ctx, "m", "request_id", "explicit-id") },
		"bound attribute":  func(l *slog.Logger) { l.With("request_id", "explicit-id").InfoContext(ctx, "m") },
		"inline group":     func(l *slog.Logger) { l.InfoContext(ctx, "m", slog.Group("", slog.String("request_id", "explicit-id"))) },
		"bound inline":     func(l *slog.Logger) { l.With(slog.Group("", slog.String("request_id", "explicit-id"))).InfoContext(ctx, "m") },
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			log(logging.New(&buf))
			require.Equal(t, 1, strings.Count(buf.String(), `"request_id"`), buf.String())
		})
	}
}
