package pg

import (
	"testing"

	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/stretchr/testify/require"
)

func TestMapOutboxEventNormalizesHeaders(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		want map[string]string
	}{
		{"request metadata", []byte(`{"X-Request-ID":"request-123","custom-header":"Привет\n\"quoted\""}`), map[string]string{"X-Request-ID": "request-123", "custom-header": "Привет\n\"quoted\""}},
		{"absent", nil, map[string]string{}},
		{"empty object", []byte(`{}`), map[string]string{}},
		{"null", []byte(`null`), map[string]string{}},
		{"malformed", []byte(`{"key":`), map[string]string{}},
		{"wrong container", []byte(`["value"]`), map[string]string{}},
		{"wrong value type discards partial headers", []byte(`{"valid":"value","invalid":42}`), map[string]string{}},
		{"null value", []byte(`{"key":null}`), map[string]string{"key": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := mapOutboxEvent(sqlc.OutboxEvent{Headers: tc.raw})
			require.Equal(t, tc.want, result.Headers)
		})
	}
}
