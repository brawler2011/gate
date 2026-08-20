package helpers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	traceparentRegex = regexp.MustCompile(`^([0-9a-f]{2})-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)
	baggageRegex     = regexp.MustCompile(`^[a-zA-Z0-9_-]+=[a-zA-Z0-9_.-]+(,[a-zA-Z0-9_-]+=[a-zA-Z0-9_.-]+)*$`)
)

// W3CContext holds generated W3C trace identifiers.
type W3CContext struct {
	TraceID     string
	SpanID      string
	TraceFlags  string
	Traceparent string
	Baggage     string
}

// GenerateRandomHex generates random lowercase hex string of specified byte length.
func GenerateRandomHex(numBytes int) string {
	b := make([]byte, numBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateW3CContext creates a valid W3C traceparent and baggage tuple.
func GenerateW3CContext() W3CContext {
	traceID := GenerateRandomHex(16) // 32 hex chars
	spanID := GenerateRandomHex(8)   // 16 hex chars
	flags := "01"                    // Sampled
	tp := fmt.Sprintf("00-%s-%s-%s", traceID, spanID, flags)
	baggage := "user_id=usr-test-1,role=participant"

	return W3CContext{
		TraceID:     traceID,
		SpanID:      spanID,
		TraceFlags:  flags,
		Traceparent: tp,
		Baggage:     baggage,
	}
}

// ParseTraceparent validates and extracts components from a W3C traceparent string.
func ParseTraceparent(header string) (version, traceID, parentSpanID, flags string, err error) {
	trimmed := strings.TrimSpace(header)
	matches := traceparentRegex.FindStringSubmatch(trimmed)
	if len(matches) != 5 {
		return "", "", "", "", fmt.Errorf("invalid traceparent format: %s", header)
	}

	version = matches[1]
	traceID = matches[2]
	parentSpanID = matches[3]
	flags = matches[4]

	// W3C TraceContext Level 1 specifies version must be "00"
	if version != "00" {
		return "", "", "", "", fmt.Errorf("unsupported W3C traceparent version: %s (expected '00')", version)
	}

	// Zero trace ID or span ID is invalid according to W3C specification
	if traceID == "00000000000000000000000000000000" {
		return "", "", "", "", fmt.Errorf("all-zero trace ID is invalid")
	}
	if parentSpanID == "0000000000000000" {
		return "", "", "", "", fmt.Errorf("all-zero span ID is invalid")
	}

	return version, traceID, parentSpanID, flags, nil
}

// AssertValidTraceparent asserts that the given traceparent string satisfies W3C specification.
func AssertValidTraceparent(t *testing.T, header string) (traceID, spanID string) {
	t.Helper()
	ver, tid, sid, flags, err := ParseTraceparent(header)
	require.NoError(t, err, "Header '%s' is not a valid W3C traceparent", header)
	require.Equal(t, "00", ver, "Expected W3C version 00")
	require.Len(t, tid, 32, "Trace ID must be 32 hex chars")
	require.Len(t, sid, 16, "Span ID must be 16 hex chars")
	require.Equal(t, "01", flags, "Trace flags expected 01 (sampled)")
	return tid, sid
}

// AssertValidBaggage asserts that the given baggage string conforms to W3C Baggage spec.
func AssertValidBaggage(t *testing.T, baggage string) {
	t.Helper()
	require.True(t, baggageRegex.MatchString(strings.TrimSpace(baggage)),
		"Baggage string '%s' does not conform to W3C Baggage format", baggage)
}

// MapCarrier implements propagation.TextMapCarrier using map[string]string.
type MapCarrier map[string]string

func (m MapCarrier) Get(key string) string {
	return m[strings.ToLower(key)]
}

func (m MapCarrier) Set(key, value string) {
	m[strings.ToLower(key)] = value
}

func (m MapCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// SimulateCarrierRoundtrip tests W3C TraceContext propagator roundtrip across a carrier.
func SimulateCarrierRoundtrip(carrier propagation.TextMapCarrier, traceID, spanID string) (extractedTraceID, extractedSpanID string, err error) {
	propagator := propagation.TraceContext{}

	tid, err := trace.TraceIDFromHex(traceID)
	if err != nil {
		return "", "", fmt.Errorf("invalid trace ID hex: %w", err)
	}
	sid, err := trace.SpanIDFromHex(spanID)
	if err != nil {
		return "", "", fmt.Errorf("invalid span ID hex: %w", err)
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	propagator.Inject(ctx, carrier)

	extractedCtx := propagator.Extract(context.Background(), carrier)
	extractedSC := trace.SpanContextFromContext(extractedCtx)

	if !extractedSC.IsValid() {
		return "", "", fmt.Errorf("extracted span context is invalid")
	}

	return extractedSC.TraceID().String(), extractedSC.SpanID().String(), nil
}

// SimulatedSpan represents a node in a simulated distributed trace tree.
type SimulatedSpan struct {
	Name         string
	ServiceName  string
	TraceID      string
	SpanID       string
	ParentSpanID string
	StartTime    time.Time
	EndTime      time.Time
	Attributes   map[string]string
}

// ValidateTraceDAG validates the directed acyclic graph of a distributed trace and returns error if invalid.
func ValidateTraceDAG(spans []SimulatedSpan, expectedRootName string) error {
	if len(spans) == 0 {
		return fmt.Errorf("span list cannot be empty")
	}

	spanMap := make(map[string]SimulatedSpan)
	var rootSpans []SimulatedSpan
	commonTraceID := spans[0].TraceID

	for _, s := range spans {
		if s.TraceID != commonTraceID {
			return fmt.Errorf("span '%s' has mismatched TraceID (expected %s, got %s)", s.Name, commonTraceID, s.TraceID)
		}
		if _, exists := spanMap[s.SpanID]; exists {
			return fmt.Errorf("duplicate SpanID '%s' found in trace", s.SpanID)
		}
		spanMap[s.SpanID] = s
		if s.ParentSpanID == "" {
			rootSpans = append(rootSpans, s)
		}
	}

	if len(rootSpans) != 1 {
		return fmt.Errorf("trace must have exactly 1 root span (found %d)", len(rootSpans))
	}
	if rootSpans[0].Name != expectedRootName {
		return fmt.Errorf("root span name mismatch: expected '%s', got '%s'", expectedRootName, rootSpans[0].Name)
	}

	// Validate parent linkage, timestamp bounds, and build adjacency list
	childrenMap := make(map[string][]string)
	for _, s := range spans {
		if !s.EndTime.IsZero() && !s.StartTime.IsZero() && s.EndTime.Before(s.StartTime) {
			return fmt.Errorf("span '%s' end time %v is before start time %v", s.Name, s.EndTime, s.StartTime)
		}
		if s.ParentSpanID != "" {
			parent, exists := spanMap[s.ParentSpanID]
			if !exists {
				return fmt.Errorf("span '%s' references non-existent parent span '%s'", s.Name, s.ParentSpanID)
			}
			if s.StartTime.Before(parent.StartTime) {
				return fmt.Errorf("child span '%s' started at %v before parent '%s' at %v", s.Name, s.StartTime, parent.Name, parent.StartTime)
			}
			childrenMap[s.ParentSpanID] = append(childrenMap[s.ParentSpanID], s.SpanID)
		}
	}

	// Detect cycles and verify tree reachability from root span via BFS
	visited := make(map[string]bool)
	queue := []string{rootSpans[0].SpanID}
	visited[rootSpans[0].SpanID] = true

	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]

		for _, childID := range childrenMap[currID] {
			if visited[childID] {
				return fmt.Errorf("cycle detected in trace DAG at span ID '%s'", childID)
			}
			visited[childID] = true
			queue = append(queue, childID)
		}
	}

	if len(visited) != len(spans) {
		return fmt.Errorf("trace contains %d disconnected or unreachable spans", len(spans)-len(visited))
	}

	return nil
}

// VerifyTraceDAG validates the directed acyclic graph of a distributed trace using testing.T.
func VerifyTraceDAG(t *testing.T, spans []SimulatedSpan, expectedRootName string) {
	t.Helper()
	err := ValidateTraceDAG(spans, expectedRootName)
	require.NoError(t, err, "Trace DAG validation failed")
}

// ConvertReadOnlySpans converts OpenTelemetry SDK ReadOnlySpans to SimulatedSpans for DAG verification.
func ConvertReadOnlySpans(spans []sdktrace.ReadOnlySpan) []SimulatedSpan {
	res := make([]SimulatedSpan, len(spans))
	for i, s := range spans {
		var parentID string
		if s.Parent().IsValid() {
			parentID = s.Parent().SpanID().String()
		}
		attrs := make(map[string]string)
		for _, a := range s.Attributes() {
			attrs[string(a.Key)] = a.Value.AsString()
		}

		serviceName := "unknown"
		if s.Resource() != nil {
			for _, a := range s.Resource().Attributes() {
				if a.Key == "service.name" {
					serviceName = a.Value.AsString()
					break
				}
			}
			if serviceName == "unknown" && len(s.Resource().Attributes()) > 0 {
				serviceName = s.Resource().Attributes()[0].Value.AsString()
			}
		}

		res[i] = SimulatedSpan{
			Name:         s.Name(),
			ServiceName:  serviceName,
			TraceID:      s.SpanContext().TraceID().String(),
			SpanID:       s.SpanContext().SpanID().String(),
			ParentSpanID: parentID,
			StartTime:    s.StartTime(),
			EndTime:      s.EndTime(),
			Attributes:   attrs,
		}
	}
	return res
}
