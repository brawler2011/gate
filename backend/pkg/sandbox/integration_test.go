//go:build integration
// +build integration

package sandbox

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultGoJudgeStartupTimeoutSec  = 120
	goJudgeHTTPPort                  = "5050/tcp"
	goJudgeGRPCPort                  = "5051/tcp"
	goJudgeReadinessRetryDelay       = 1500 * time.Millisecond
	goJudgeReadinessProbeCallTimeout = 8 * time.Second
)

var (
	integrationSetupOnce   sync.Once
	integrationSetupErr    error
	integrationContainer   testcontainers.Container
	integrationSandboxConn *Client
)

func TestMain(m *testing.M) {
	code := m.Run()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if integrationSandboxConn != nil {
		_ = integrationSandboxConn.Close()
	}
	if integrationContainer != nil {
		_ = integrationContainer.Terminate(cleanupCtx)
	}

	os.Exit(code)
}

func getIntegrationOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()

	integrationSetupOnce.Do(func() {
		integrationSetupErr = setupIntegrationHarness(context.Background())
	})

	if integrationSetupErr != nil {
		t.Skipf("Skipping integration: unable to start go-judge test container: %v", integrationSetupErr)
	}

	return NewOrchestrator(integrationSandboxConn)
}

func setupIntegrationHarness(ctx context.Context) error {
	startupTimeout := startupTimeoutFromEnv()

	req := testcontainers.ContainerRequest{
		ExposedPorts: []string{goJudgeHTTPPort, goJudgeGRPCPort},
		Env: map[string]string{
			"ES_ENABLE_GRPC": "true",
			"ES_PARALLELISM": "2",
			"ES_PRE_FORK":    "1",
		},
		Privileged:   true,
		WaitingFor: wait.ForLog("Starting http server").
			WithStartupTimeout(startupTimeout),
	}
	if image := os.Getenv("GOJUDGE_TEST_IMAGE"); image != "" {
		req.Image = image
	} else {
		buildContext, dockerfileName, err := testDockerfileContext()
		if err != nil {
			return err
		}
		req.FromDockerfile = testcontainers.FromDockerfile{
			Context:    buildContext,
			Dockerfile: dockerfileName,
			Repo:       "gate-gojudge-integration",
			Tag:        "local",
			KeepImage:  false,
		}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}
	integrationContainer = container

	host, err := container.Host(ctx)
	if err != nil {
		return err
	}

	mappedPort, err := container.MappedPort(ctx, goJudgeGRPCPort)
	if err != nil {
		return err
	}

	grpcAddr := net.JoinHostPort(host, mappedPort.Port())
	client, err := NewClient(ClientConfig{
		Protocol: ProtocolGRPC,
		BaseURL:  grpcAddr,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		return err
	}
	integrationSandboxConn = client

	return waitForJudgeReady(ctx, client, startupTimeout)
}

func startupTimeoutFromEnv() time.Duration {
	raw := os.Getenv("GOJUDGE_TEST_STARTUP_TIMEOUT")
	if raw == "" {
		return time.Duration(defaultGoJudgeStartupTimeoutSec) * time.Second
	}

	if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
		return duration
	}

	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}

	return time.Duration(defaultGoJudgeStartupTimeoutSec) * time.Second
}

func waitForJudgeReady(ctx context.Context, client *Client, startupTimeout time.Duration) error {
	deadline := time.Now().Add(startupTimeout)
	var lastErr error

	for time.Now().Before(deadline) {
		probeCtx, cancel := context.WithTimeout(ctx, goJudgeReadinessProbeCallTimeout)
		result, err := client.Compile(probeCtx, CompileRequest{
			SourceCode:  "int main(){return 0;}",
			Language:    "cpp17",
			SourceFile:  "main.cpp",
			OutputFile:  "main",
			Dependencies: map[string]string{},
			Limits: ResourceLimits{
				CPUTimeMs: 5000,
				MemoryMB:  256,
				ProcLimit: 10,
				StackMB:   64,
			},
		})
		cancel()

		if err == nil && result != nil && result.Success {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("compile probe returned non-success status: %s", result.Stderr)
		}

		time.Sleep(goJudgeReadinessRetryDelay)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("readiness probe timed out")
	}
	return fmt.Errorf("go-judge gRPC readiness probe failed: %w", lastErr)
}

func testDockerfileContext() (string, string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", fmt.Errorf("failed to resolve integration test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata"), "gojudge.Dockerfile", nil
}

func TestIntegrationGRPCCompileAndRunCpp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	orchestrator := getIntegrationOrchestrator(t)

	// Program that outputs wrong answer
	sourceCode := `#include <iostream>
using namespace std;

int main() {
    int a, b;
    cin >> a >> b;
    cout << a + b << endl;
    return 0;
}`

	ctx := context.Background()
	result, err := judgeWithRetry(ctx, orchestrator, JudgeSolutionRequest{
		SolutionCode:     sourceCode,
		SolutionLanguage: "cpp17",
		CheckerFileID:    "", // No checker, simple text comparison
		Input:            []byte("5 3\n"),
		Answer:           []byte("8\n"),
		TimeLimitMs:      1000,
		MemoryLimitMB:    256,
	})

	if err != nil {
		t.Fatalf("JudgeSolution failed: %v", err)
	}

	if result.Verdict != "OK" {
		t.Errorf("Expected verdict OK, got %s: %s", result.Verdict, result.Message)
	}

	t.Logf("Solution ran successfully: Time=%dms, Memory=%dKB",
		result.Time/1000000, result.Memory/1024)
}

func TestIntegrationGRPCCompileError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	orchestrator := getIntegrationOrchestrator(t)

	sourceCode := `#include <iostream>
int main() {
    this code will not compile
}`

	ctx := context.Background()
	result, err := judgeWithRetry(ctx, orchestrator, JudgeSolutionRequest{
		SolutionCode:     sourceCode,
		SolutionLanguage: "cpp17",
		CheckerFileID:    "",
		Input:            []byte(""),
		Answer:           []byte(""),
		TimeLimitMs:      1000,
		MemoryLimitMB:    256,
	})
	if err != nil {
		t.Fatalf("JudgeSolution failed: %v", err)
	}

	if result.Verdict != "CE" {
		t.Errorf("Expected verdict CE, got %s", result.Verdict)
	}

	if result.CompileError == "" {
		t.Error("Expected compile error message")
	}

	t.Logf("Compilation error detected correctly: %s", result.CompileError)
}

func TestIntegrationGRPCWrongAnswer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	orchestrator := getIntegrationOrchestrator(t)

	sourceCode := `#include <iostream>
using namespace std;

int main() {
    int a, b;
    cin >> a >> b;
    cout << a - b << endl;
    return 0;
}`

	ctx := context.Background()
	result, err := orchestrator.JudgeSolution(ctx, JudgeSolutionRequest{
		SolutionCode:     sourceCode,
		SolutionLanguage: "cpp17",
		CheckerFileID:    "",
		Input:            []byte("5 3\n"),
		Answer:           []byte("8\n"),
		TimeLimitMs:      1000,
		MemoryLimitMB:    256,
	})

	if err != nil {
		t.Fatalf("JudgeSolution failed: %v", err)
	}

	if result.Verdict != "WA" {
		t.Errorf("Expected verdict WA, got %s", result.Verdict)
	}
}

func TestIntegrationGRPCCompileChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	orchestrator := getIntegrationOrchestrator(t)

	checkerCode := `#include <iostream>
using namespace std;

int main(int argc, char* argv[]) {
    cout << "ok" << endl;
    return 0;
}`

	ctx := context.Background()
	binary, err := orchestrator.CompileComponentFromSource(ctx, "checker", checkerCode, "cpp17", nil)
	if err != nil {
		t.Fatalf("Failed to compile checker: %v", err)
	}

	if !binary.Success {
		t.Errorf("Checker compilation failed: %s", binary.Error)
	}

	if binary.FileID == "" {
		t.Error("Expected FileID for compiled checker")
	}
}

func TestIntegrationGRPCGenerator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	orchestrator := getIntegrationOrchestrator(t)

	generatorCode := `#include <iostream>
using namespace std;

int main(int argc, char* argv[]) {
    int n = 5;
    if (argc > 1) {
        n = atoi(argv[1]);
    }

    cout << n << endl;
    for (int i = 0; i < n; i++) {
        cout << (i + 1) << " ";
    }
    cout << endl;

    return 0;
}`

	ctx := context.Background()
	binary, err := orchestrator.CompileComponentFromSource(ctx, "generator", generatorCode, "cpp17", nil)
	if err != nil {
		t.Fatalf("Failed to compile generator: %v", err)
	}

	if !binary.Success {
		t.Errorf("Generator compilation failed: %s", binary.Error)
	}

	limits := ResourceLimits{
		CPUTimeMs: 5000,
		MemoryMB:  256,
		ProcLimit: 1,
		StackMB:   256,
	}

	generated, err := orchestrator.GenerateTest(ctx, GenerateTestRequest{
		GeneratorFileID: binary.FileID,
		Arguments:       []string{"10"},
		Seed:            42,
		Limits:          limits,
	})
	if err != nil {
		t.Fatalf("Failed to generate test: %v", err)
	}

	if !generated.Success {
		t.Errorf("Test generation failed: %s", generated.Error)
	}

	if len(generated.Input) == 0 {
		t.Error("Generated test is empty")
	}
}

func TestIntegrationGRPCToolchainProbe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	orchestrator := getIntegrationOrchestrator(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		language string
		source   string
	}{
		{
			name:     "cpp17",
			language: "cpp17",
			source: `#include <iostream>
int main() {
    std::cout << 7 << std::endl;
    return 0;
}`,
		},
		{
			name:     "c11",
			language: "c11",
			source: `#include <stdio.h>
int main(void) {
    printf("7\n");
    return 0;
}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
	result, err := judgeWithRetry(ctx, orchestrator, JudgeSolutionRequest{
				SolutionCode:     tt.source,
				SolutionLanguage: tt.language,
				CheckerFileID:    "",
				Input:            []byte(""),
				Answer:           []byte("7\n"),
				TimeLimitMs:      1000,
				MemoryLimitMB:    256,
			})
			if err != nil {
				t.Fatalf("JudgeSolution failed for %s: %v", tt.language, err)
			}
			if result.Verdict != "OK" {
				t.Fatalf("Expected OK for %s toolchain probe, got %s (%s)", tt.language, result.Verdict, result.Message)
			}
		})
	}
}

func judgeWithRetry(ctx context.Context, orchestrator *Orchestrator, req JudgeSolutionRequest) (*JudgeResult, error) {
	var lastResult *JudgeResult
	var lastErr error

	for attempt := 0; attempt < 4; attempt++ {
		result, err := orchestrator.JudgeSolution(ctx, req)
		if err != nil {
			return nil, err
		}
		lastResult = result
		lastErr = err

		if result == nil || result.Verdict != "CE" || !strings.Contains(result.CompileError, "Resource temporarily unavailable") {
			return result, nil
		}

		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}

	return lastResult, lastErr
}
