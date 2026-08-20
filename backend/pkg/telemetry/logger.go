package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

func initLoggerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}

	exporter, err := otlploggrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	return lp, nil
}

func newStdoutHandler(env string) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if env == "dev" || env == "local" {
		opts.Level = slog.LevelDebug
		return slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.NewJSONHandler(os.Stdout, opts)
}

// CompositeHandler fans out slog records to multiple handlers with trace_id/span_id injection and attribute sanitization.
type CompositeHandler struct {
	handlers []slog.Handler
}

// NewCompositeHandler creates a new CompositeHandler wrapping the provided handlers.
func NewCompositeHandler(handlers ...slog.Handler) *CompositeHandler {
	validHandlers := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			validHandlers = append(validHandlers, h)
		}
	}
	return &CompositeHandler{handlers: validHandlers}
}

func (h *CompositeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *CompositeHandler) Handle(ctx context.Context, r slog.Record) error {
	// 1. Inject trace_id (32 hex) and span_id (16 hex) if valid span context is present
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	// 2. Sanitize record attributes
	sanitizedRecord := sanitizeRecord(r)

	// 3. Dispatch to all active handlers
	var errs []error
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, sanitizedRecord.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (h *CompositeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitizedAttrs := SanitizeAttrs(attrs)
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(sanitizedAttrs)
	}
	return &CompositeHandler{handlers: handlers}
}

func (h *CompositeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &CompositeHandler{handlers: handlers}
}

func sanitizeRecord(r slog.Record) slog.Record {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(attr slog.Attr) bool {
		newRecord.AddAttrs(SanitizeAttr(attr))
		return true
	})
	return newRecord
}

// Logger returns the global default slog logger.
func Logger() *slog.Logger {
	return slog.Default()
}
