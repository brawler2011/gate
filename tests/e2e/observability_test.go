package e2e_test

import (
	"context"
	"os"
	"testing"

	"github.com/brawler2011/gate/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

var (
	testSpanRecorder *tracetest.SpanRecorder
	testTracerProvider *sdktrace.TracerProvider
)

func TestMain(m *testing.M) {
	// Initialize global in-memory OpenTelemetry tracer provider for test assertions
	testSpanRecorder = tracetest.NewSpanRecorder()
	testTracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(testSpanRecorder),
	)
	otel.SetTracerProvider(testTracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	code := m.Run()

	_ = testTracerProvider.Shutdown(context.Background())
	os.Exit(code)
}

func TestObservabilitySuite_HealthCheck(t *testing.T) {
	repoRoot := helpers.FindRepoRoot()
	require.NotEmpty(t, repoRoot, "Repository root must be resolvable")
	require.True(t, helpers.FileExists(repoRoot+"/Taskfile.yml"), "Taskfile.yml must exist at repo root")
	require.True(t, helpers.DirExists(helpers.GetDeployPath()), "deploy/ directory must exist")
	require.True(t, helpers.DirExists(helpers.GetBackendPath()), "backend/ directory must exist")
	require.True(t, helpers.DirExists(helpers.GetFrontendPath()), "frontend/ directory must exist")
}
