package pg

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOutboxHeaders_ComplexSerialization stress-tests serialization/deserialization
// with complex header inputs: W3C traceparent/tracestate, UTF-8, special characters,
// empty keys/values, large maps, nil vs empty map.
func TestOutboxHeaders_ComplexSerialization(t *testing.T) {
	testID := uuid.New()
	testAggregateID := uuid.New()
	now := time.Now().UTC()

	tests := []struct {
		name            string
		inputHeaders    map[string]string
		expectedHeaders map[string]string
	}{
		{
			name: "W3C Trace Context headers",
			inputHeaders: map[string]string{
				"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
				"tracestate":  "rojo=1,congo=t61rcWkgMzE=,vendorname=opaqueValue",
			},
			expectedHeaders: map[string]string{
				"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
				"tracestate":  "rojo=1,congo=t61rcWkgMzE=,vendorname=opaqueValue",
			},
		},
		{
			name: "Multi-byte UTF-8 and Unicode characters",
			inputHeaders: map[string]string{
				"x-cyrillic":  "Привет мир",
				"x-japanese":  "こんにちは世界",
				"x-chinese":   "你好世界",
				"x-arabic":    "مرحبا بالعالم",
				"x-emoji":     "🚀🔥🎉✨🌍💻🤖",
				"x-math":      "∀x ∈ ℝ, x² ≥ 0 ∑ ∏ √ ∫",
				"x-accents":   "é à ü ñ ç ø å æ œ",
				"x-combining": "e\u0301a\u0300",
			},
			expectedHeaders: map[string]string{
				"x-cyrillic":  "Привет мир",
				"x-japanese":  "こんにちは世界",
				"x-chinese":   "你好世界",
				"x-arabic":    "مرحبا بالعالم",
				"x-emoji":     "🚀🔥🎉✨🌍💻🤖",
				"x-math":      "∀x ∈ ℝ, x² ≥ 0 ∑ ∏ √ ∫",
				"x-accents":   "é à ü ñ ç ø å æ œ",
				"x-combining": "e\u0301a\u0300",
			},
		},
		{
			name: "Special characters, control characters, JSON injection, and SQL injection strings",
			inputHeaders: map[string]string{
				"x-quotes":         "\"quoted\" and 'single-quoted'",
				"x-backslashes":    "C:\\Users\\Admin\\Documents\\file.txt \\\\ //",
				"x-newlines":       "line1\nline2\r\nline3\tindented",
				"x-json-injection": "{\"nested\":{\"key\":\"value\"},\"array\":[1,2,3]}",
				"x-sql-injection":  "'; DROP TABLE outbox_events; --",
				"x-html-script":    "<script>alert('xss')</script>",
				"x-special-chars":  "~!@#$%^&*()_+-={}|[]\\:\";'<>?,./`",
			},
			expectedHeaders: map[string]string{
				"x-quotes":         "\"quoted\" and 'single-quoted'",
				"x-backslashes":    "C:\\Users\\Admin\\Documents\\file.txt \\\\ //",
				"x-newlines":       "line1\nline2\r\nline3\tindented",
				"x-json-injection": "{\"nested\":{\"key\":\"value\"},\"array\":[1,2,3]}",
				"x-sql-injection":  "'; DROP TABLE outbox_events; --",
				"x-html-script":    "<script>alert('xss')</script>",
				"x-special-chars":  "~!@#$%^&*()_+-={}|[]\\:\";'<>?,./`",
			},
		},
		{
			name: "Empty keys and empty values",
			inputHeaders: map[string]string{
				"":            "empty-key-value",
				"empty-value": "",
				" ":           " ",
			},
			expectedHeaders: map[string]string{
				"":            "empty-key-value",
				"empty-value": "",
				" ":           " ",
			},
		},
		{
			name:            "Empty map",
			inputHeaders:    map[string]string{},
			expectedHeaders: map[string]string{},
		},
		{
			name:            "Nil map",
			inputHeaders:    nil,
			expectedHeaders: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test serialization simulation (what CreateEvent does)
			var headersBytes []byte
			if len(tc.inputHeaders) > 0 {
				var err error
				headersBytes, err = json.Marshal(tc.inputHeaders)
				require.NoError(t, err)
			} else {
				headersBytes = []byte("{}")
			}

			// Validate that headersBytes is valid JSON
			var checkObj any
			err := json.Unmarshal(headersBytes, &checkObj)
			require.NoError(t, err, "Marshaled headers must be valid JSON")

			// Test deserialization via mapOutboxEvent
			sqlcEvent := sqlc.OutboxEvent{
				ID:          testID,
				AggregateID: testAggregateID,
				EventType:   models.OutboxEventSubmissionCreated,
				Payload:     []byte(`{"test":true}`),
				Status:      models.OutboxEventStatusPending,
				CreatedAt:   now,
				Headers:     headersBytes,
			}

			mapped := mapOutboxEvent(sqlcEvent)
			assert.NotNil(t, mapped.Headers, "Headers map must never be nil")
			assert.Equal(t, tc.expectedHeaders, mapped.Headers)
		})
	}
}

// TestOutboxHeaders_LargePayloads verifies serialization with large number of keys and large values.
func TestOutboxHeaders_LargePayloads(t *testing.T) {
	testID := uuid.New()
	testAggregateID := uuid.New()
	now := time.Now().UTC()

	t.Run("1000 key-value pairs", func(t *testing.T) {
		largeMap := make(map[string]string, 1000)
		for i := 0; i < 1000; i++ {
			n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
			largeMap[fmt.Sprintf("x-custom-header-key-%04d", i)] = fmt.Sprintf("value-%04d-payload-%d", i, n.Int64())
		}

		rawJSON, err := json.Marshal(largeMap)
		require.NoError(t, err)

		sqlcEvent := sqlc.OutboxEvent{
			ID:          testID,
			AggregateID: testAggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{}`),
			Status:      models.OutboxEventStatusPending,
			CreatedAt:   now,
			Headers:     rawJSON,
		}

		mapped := mapOutboxEvent(sqlcEvent)
		assert.NotNil(t, mapped.Headers)
		assert.Len(t, mapped.Headers, 1000)
		assert.Equal(t, largeMap, mapped.Headers)
	})

	t.Run("Large header value (64KB string)", func(t *testing.T) {
		largeVal := strings.Repeat("A-Z-0-9-utf8-привет-", 3200) // ~64KB
		headersMap := map[string]string{
			"x-large-header": largeVal,
		}

		rawJSON, err := json.Marshal(headersMap)
		require.NoError(t, err)

		sqlcEvent := sqlc.OutboxEvent{
			ID:          testID,
			AggregateID: testAggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{}`),
			Status:      models.OutboxEventStatusPending,
			CreatedAt:   now,
			Headers:     rawJSON,
		}

		mapped := mapOutboxEvent(sqlcEvent)
		assert.NotNil(t, mapped.Headers)
		assert.Equal(t, largeVal, mapped.Headers["x-large-header"])
	})
}

// TestOutboxHeaders_ErrorResilience tests resilience against unexpected DB values,
// malformed JSON, wrong data types, and corrupted byte sequences.
func TestOutboxHeaders_ErrorResilience(t *testing.T) {
	testID := uuid.New()
	testAggregateID := uuid.New()
	now := time.Now().UTC()

	corruptedCases := []struct {
		name       string
		rawHeaders []byte
		expected   map[string]string
	}{
		{"nil slice", nil, map[string]string{}},
		{"empty slice", []byte{}, map[string]string{}},
		{"whitespace only", []byte("   \n\t  "), map[string]string{}},
		{"null literal", []byte("null"), map[string]string{}},
		{"boolean true", []byte("true"), map[string]string{}},
		{"boolean false", []byte("false"), map[string]string{}},
		{"number integer", []byte("12345"), map[string]string{}},
		{"number float", []byte("3.14159"), map[string]string{}},
		{"JSON string literal", []byte(`"just a string"`), map[string]string{}},
		{"JSON array of strings", []byte(`["a", "b", "c"]`), map[string]string{}},
		{"JSON array of objects", []byte(`[{"key":"val"}]`), map[string]string{}},
		{"JSON object with integer values", []byte(`{"trace_id": 123456789}`), map[string]string{}},
		{"JSON object with nested objects", []byte(`{"trace": {"parent": "abc"}}`), map[string]string{}},
		{"JSON object with array values", []byte(`{"tags": ["tag1", "tag2"]}`), map[string]string{}},
		{"JSON object with boolean values", []byte(`{"enabled": true}`), map[string]string{}},
		{"JSON object with null value", []byte(`{"key": null}`), map[string]string{"key": ""}},
		{"truncated JSON opening brace", []byte(`{`), map[string]string{}},
		{"truncated JSON unclosed string", []byte(`{"key": "val`), map[string]string{}},
		{"malformed JSON missing colon", []byte(`{"key" "val"}`), map[string]string{}},
		{"malformed JSON trailing comma", []byte(`{"key": "val",}`), map[string]string{}},
		{"malformed JSON garbage text", []byte(`not json at all`), map[string]string{}},
		{"binary garbage bytes", []byte{0x00, 0xFF, 0xFE, 0xFD, 0x80, 0x81, 0x1F, 0x20}, map[string]string{}},
		{"HTML content", []byte(`<html><body>502 Bad Gateway</body></html>`), map[string]string{}},
		{"SQL error string", []byte(`ERROR: relation "outbox_events" does not exist`), map[string]string{}},
	}

	for _, tc := range corruptedCases {
		t.Run(tc.name, func(t *testing.T) {
			sqlcEvent := sqlc.OutboxEvent{
				ID:          testID,
				AggregateID: testAggregateID,
				EventType:   models.OutboxEventSubmissionCreated,
				Payload:     []byte(`{}`),
				Status:      models.OutboxEventStatusPending,
				CreatedAt:   now,
				Headers:     tc.rawHeaders,
			}

			// Must not panic, must return non-nil map with expected contents
			require.NotPanics(t, func() {
				mapped := mapOutboxEvent(sqlcEvent)
				assert.NotNil(t, mapped.Headers, "Headers must never be nil even on corrupted DB input")
				assert.Equal(t, tc.expected, mapped.Headers)
				assert.Equal(t, testID, mapped.Id)
				assert.Equal(t, testAggregateID, mapped.AggregateID)
			})
		})
	}
}

// TestOutboxHeaders_Fuzzing sends 10,000 randomized byte sequences to mapOutboxEvent
// to ensure zero panics and memory safety.
func TestOutboxHeaders_Fuzzing(t *testing.T) {
	testID := uuid.New()
	testAggregateID := uuid.New()
	now := time.Now().UTC()

	for i := 0; i < 10000; i++ {
		lenBig, _ := rand.Int(rand.Reader, big.NewInt(256))
		length := int(lenBig.Int64())
		buf := make([]byte, length)
		_, _ = rand.Read(buf)

		sqlcEvent := sqlc.OutboxEvent{
			ID:          testID,
			AggregateID: testAggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{}`),
			Status:      models.OutboxEventStatusPending,
			CreatedAt:   now,
			Headers:     buf,
		}

		assert.NotPanics(t, func() {
			mapped := mapOutboxEvent(sqlcEvent)
			assert.NotNil(t, mapped.Headers, "Headers must not be nil during fuzz iteration %d", i)
		})
	}
}
