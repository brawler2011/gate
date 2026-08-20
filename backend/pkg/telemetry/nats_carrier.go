package telemetry

import (
	"context"
	"strings"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// NATSHeaderCarrier adapts nats.Header to satisfy propagation.TextMapCarrier with case-insensitive lookup.
type NATSHeaderCarrier nats.Header

var _ propagation.TextMapCarrier = NATSHeaderCarrier{}

// Get returns the value associated with the passed key (case-insensitive).
func (c NATSHeaderCarrier) Get(key string) string {
	if c == nil {
		return ""
	}
	if v := c[key]; len(v) > 0 {
		return v[0]
	}
	for k, v := range c {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// Set stores the key-value pair.
func (c NATSHeaderCarrier) Set(key, value string) {
	if c == nil {
		return
	}
	nats.Header(c).Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (c NATSHeaderCarrier) Keys() []string {
	if c == nil {
		return nil
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// NATSMessageCarrier adapts *nats.Msg to satisfy propagation.TextMapCarrier with automatic header initialization.
type NATSMessageCarrier struct {
	msg *nats.Msg
}

var _ propagation.TextMapCarrier = (*NATSMessageCarrier)(nil)

// NewNATSMessageCarrier creates a new carrier wrapping a NATS message.
func NewNATSMessageCarrier(msg *nats.Msg) *NATSMessageCarrier {
	if msg != nil && msg.Header == nil {
		msg.Header = make(nats.Header)
	}
	return &NATSMessageCarrier{msg: msg}
}

// Get returns the value associated with the passed key.
func (c *NATSMessageCarrier) Get(key string) string {
	if c == nil || c.msg == nil || c.msg.Header == nil {
		return ""
	}
	return NATSHeaderCarrier(c.msg.Header).Get(key)
}

// Set stores the key-value pair, auto-initializing msg.Header if nil.
func (c *NATSMessageCarrier) Set(key, value string) {
	if c == nil || c.msg == nil {
		return
	}
	if c.msg.Header == nil {
		c.msg.Header = make(nats.Header)
	}
	NATSHeaderCarrier(c.msg.Header).Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (c *NATSMessageCarrier) Keys() []string {
	if c == nil || c.msg == nil || c.msg.Header == nil {
		return nil
	}
	return NATSHeaderCarrier(c.msg.Header).Keys()
}

// InjectNATSMsg injects current OpenTelemetry trace context from ctx into msg.Header.
func InjectNATSMsg(ctx context.Context, msg *nats.Msg) {
	if msg == nil {
		return
	}
	if msg.Header == nil {
		msg.Header = make(nats.Header)
	}
	otel.GetTextMapPropagator().Inject(ctx, NATSHeaderCarrier(msg.Header))
}

// ExtractNATSMsg extracts OpenTelemetry trace context from msg.Header into a child context.
func ExtractNATSMsg(ctx context.Context, msg *nats.Msg) context.Context {
	if msg == nil || msg.Header == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, NATSHeaderCarrier(msg.Header))
}

// InjectNATSHeader injects OpenTelemetry trace context from ctx into a nats.Header.
func InjectNATSHeader(ctx context.Context, header nats.Header) {
	if header == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, NATSHeaderCarrier(header))
}

// ExtractNATSHeader extracts OpenTelemetry trace context from a nats.Header into a child context.
func ExtractNATSHeader(ctx context.Context, header nats.Header) context.Context {
	if header == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, NATSHeaderCarrier(header))
}

// InjectNATS is an alias for InjectNATSHeader.
func InjectNATS(ctx context.Context, header nats.Header) {
	InjectNATSHeader(ctx, header)
}

// ExtractNATS is an alias for ExtractNATSHeader.
func ExtractNATS(ctx context.Context, header nats.Header) context.Context {
	return ExtractNATSHeader(ctx, header)
}
