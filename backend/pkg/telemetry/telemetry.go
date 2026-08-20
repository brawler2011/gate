package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds OpenTelemetry configuration parameters.
type Config struct {
	Endpoint       string
	ServiceName    string
	ServiceVersion string
	Environment    string
	Insecure       bool
	Enabled        bool
}

// ShutdownFunc safely flushes and stops all telemetry providers.
type ShutdownFunc func(ctx context.Context) error

// InitTelemetry initializes OpenTelemetry TracerProvider, MeterProvider, LoggerProvider,
// Go runtime metrics, and sets up global propagators and default slog logger.
func InitTelemetry(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		// Telemetry disabled: configure fallback stdout logger with sanitization
		stdoutHandler := newStdoutHandler(cfg.Environment)
		sanitizedHandler := NewSanitizingHandler(stdoutHandler)
		logger := slog.New(sanitizedHandler)
		slog.SetDefault(logger)
		return func(ctx context.Context) error { return nil }, nil
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	// 1. Initialize TracerProvider
	tp, err := initTracerProvider(ctx, cfg, res)
	if err != nil {
		return nil, fmt.Errorf("init tracer provider: %w", err)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 2. Initialize MeterProvider & Runtime Metrics
	mp, err := initMeterProvider(ctx, cfg, res)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("init meter provider: %w", err)
	}
	otel.SetMeterProvider(mp)
	if err := runtime.Start(
		runtime.WithMeterProvider(mp),
		runtime.WithMinimumReadMemStatsInterval(15*time.Second),
	); err != nil {
		slog.Warn("failed to start runtime metrics collection", slog.String("error", err.Error()))
	}

	if err := initCustomMetrics(mp); err != nil {
		slog.Warn("failed to initialize custom metrics", slog.String("error", err.Error()))
	}

	// 3. Initialize LoggerProvider & Composite slog Logger
	lp, err := initLoggerProvider(ctx, cfg, res)
	if err != nil {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil, fmt.Errorf("init logger provider: %w", err)
	}

	stdoutHandler := newStdoutHandler(cfg.Environment)
	otelHandler := otelslog.NewHandler(cfg.ServiceName, otelslog.WithLoggerProvider(lp))
	compositeHandler := NewCompositeHandler(stdoutHandler, otelHandler)
	logger := slog.New(compositeHandler)
	slog.SetDefault(logger)

	shutdown := func(shutdownCtx context.Context) error {
		var errs []error
		if err := tp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown tracer provider: %w", err))
		}
		if err := mp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown meter provider: %w", err))
		}
		if err := lp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown logger provider: %w", err))
		}
		return errors.Join(errs...)
	}

	return shutdown, nil
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "gate-backend"
	}
	serviceVersion := cfg.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = "0.1.0"
	}

	extraRes, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			attribute.String("deployment.environment.name", cfg.Environment),
		),
		resource.WithHost(),
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessCommandArgs(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithOS(),
		resource.WithContainer(),
	)
	if err != nil && !errors.Is(err, resource.ErrPartialResource) {
		return nil, fmt.Errorf("create telemetry resource attributes: %w", err)
	}

	merged, err := resource.Merge(resource.Default(), extraRes)
	if err != nil {
		return nil, fmt.Errorf("merge telemetry resource: %w", err)
	}

	return merged, nil
}
