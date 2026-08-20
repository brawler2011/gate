package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

// CapturedLogRecord stores a structured log event recorded in-memory.
type CapturedLogRecord struct {
	Time       time.Time              `json:"time"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Attributes map[string]interface{} `json:"attributes"`
	TraceID    string                 `json:"trace_id,omitempty"`
	SpanID     string                 `json:"span_id,omitempty"`
}

// InMemoryLogHandler implements slog.Handler to capture log events into a thread-safe slice.
type InMemoryLogHandler struct {
	mu           sync.Mutex
	records      []CapturedLogRecord
	level        slog.Level
	sanitize     bool
	injectTraces bool
	attrs        []slog.Attr
	groups       []string
	buffer       *bytes.Buffer
}

// NewInMemoryLogHandler creates an in-memory logger handler.
func NewInMemoryLogHandler(level slog.Level, sanitize, injectTraces bool) (*InMemoryLogHandler, *slog.Logger) {
	buf := new(bytes.Buffer)
	h := &InMemoryLogHandler{
		records:      make([]CapturedLogRecord, 0),
		level:        level,
		sanitize:     sanitize,
		injectTraces: injectTraces,
		buffer:       buf,
	}
	return h, slog.New(h)
}

func (h *InMemoryLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// slogValueToAny converts a slog.Value into a JSON-serializable primitive or map.
func slogValueToAny(v slog.Value) interface{} {
	switch v.Kind() {
	case slog.KindGroup:
		groupMap := make(map[string]interface{})
		for _, ga := range v.Group() {
			groupMap[ga.Key] = slogValueToAny(ga.Value)
		}
		return groupMap
	case slog.KindAny:
		return v.Any()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindString:
		return v.String()
	case slog.KindTime:
		return v.Time()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindLogValuer:
		return slogValueToAny(v.Resolve())
	default:
		return v.Any()
	}
}

func (h *InMemoryLogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	attrsMap := make(map[string]interface{})

	insertAttr := func(target map[string]interface{}, a slog.Attr) {
		attr := a
		if h.sanitize {
			attr = SanitizeSlogAttribute(attr)
		}
		target[attr.Key] = slogValueToAny(attr.Value)
	}

	destMap := attrsMap
	for _, g := range h.groups {
		if g == "" {
			continue
		}
		groupMap := make(map[string]interface{})
		destMap[g] = groupMap
		destMap = groupMap
	}

	// Add handler pre-set attributes
	for _, a := range h.attrs {
		insertAttr(destMap, a)
	}

	// Add record attributes
	r.Attrs(func(a slog.Attr) bool {
		insertAttr(destMap, a)
		return true
	})

	var traceID, spanID string
	if h.injectTraces && ctx != nil {
		sc := trace.SpanContextFromContext(ctx)
		if sc.IsValid() {
			traceID = sc.TraceID().String()
			spanID = sc.SpanID().String()
			attrsMap["trace_id"] = traceID
			attrsMap["span_id"] = spanID
		}
	}

	rec := CapturedLogRecord{
		Time:       r.Time,
		Level:      r.Level.String(),
		Message:    r.Message,
		Attributes: attrsMap,
		TraceID:    traceID,
		SpanID:     spanID,
	}

	h.records = append(h.records, rec)

	// Format line to buffer
	jsonBytes, err := json.Marshal(rec)
	if err == nil {
		h.buffer.Write(jsonBytes)
		h.buffer.WriteString("\n")
	}

	return nil
}

func (h *InMemoryLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	newAttrs := append([]slog.Attr(nil), h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &InMemoryLogHandler{
		records:      h.records,
		level:        h.level,
		sanitize:     h.sanitize,
		injectTraces: h.injectTraces,
		attrs:        newAttrs,
		groups:       h.groups,
		buffer:       h.buffer,
	}
}

func (h *InMemoryLogHandler) WithGroup(name string) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	newGroups := append([]string(nil), h.groups...)
	newGroups = append(newGroups, name)
	return &InMemoryLogHandler{
		records:      h.records,
		level:        h.level,
		sanitize:     h.sanitize,
		injectTraces: h.injectTraces,
		attrs:        h.attrs,
		groups:       newGroups,
		buffer:       h.buffer,
	}
}

// GetRecords returns a copy of captured records.
func (h *InMemoryLogHandler) GetRecords() []CapturedLogRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	res := make([]CapturedLogRecord, len(h.records))
	copy(res, h.records)
	return res
}

// GetLogOutput returns the full string output of the log buffer.
func (h *InMemoryLogHandler) GetLogOutput() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.buffer.String()
}

// Reset clears all captured records and buffer.
func (h *InMemoryLogHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = make([]CapturedLogRecord, 0)
	h.buffer.Reset()
}

// AssertLogContainsTraceContext asserts that at least one captured record has the expected TraceID and SpanID.
func AssertLogContainsTraceContext(t *testing.T, handler *InMemoryLogHandler, expectedTraceID, expectedSpanID string) {
	t.Helper()
	records := handler.GetRecords()
	found := false
	for _, r := range records {
		if r.TraceID == expectedTraceID {
			found = true
			if expectedSpanID != "" {
				require.Equal(t, expectedSpanID, r.SpanID, "SpanID mismatch in log record: %+v", r)
			}
			break
		}
	}
	require.True(t, found, "No log record found with trace_id '%s' (records count: %d)", expectedTraceID, len(records))
}

func checkNestedLogAttributes(t *testing.T, attrs map[string]interface{}, knownSecrets []string) {
	for key, val := range attrs {
		if subMap, ok := val.(map[string]interface{}); ok {
			checkNestedLogAttributes(t, subMap, knownSecrets)
			continue
		}
		valStr := fmt.Sprintf("%v", val)
		for _, secret := range knownSecrets {
			if secret == "" {
				continue
			}
			require.False(t, strings.Contains(valStr, secret),
				"Secret '%s' leaked in log attribute '%s' (value: %s)", secret, key, valStr)
		}
		if IsSensitiveKey(key) {
			require.Equal(t, RedactedPlaceholder, valStr,
				"Sensitive log attribute '%s' was not masked to [REDACTED]", key)
		}
	}
}

// AssertLogSanitized verifies no known secrets appear in any attribute of the captured records.
func AssertLogSanitized(t *testing.T, handler *InMemoryLogHandler, knownSecrets []string) {
	t.Helper()
	records := handler.GetRecords()
	output := handler.GetLogOutput()

	AssertLogOutputSanitization(t, output, knownSecrets)

	for _, r := range records {
		checkNestedLogAttributes(t, r.Attributes, knownSecrets)
	}
}
