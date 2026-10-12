package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/logging"
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
