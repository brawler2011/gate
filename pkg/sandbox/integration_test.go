// +build integration

package sandbox

import (
	"context"
	"testing"
	"time"
)

// Integration tests require a running go-judge instance
// Run with: go test -tags=integration -v

const (
	testGoJudgeHTTP = "http://localhost:5050"
	testGoJudgeGRPC = "localhost:5051"
)

func TestIntegrationHTTPCompileAndRunCpp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(ClientConfig{
		Protocol: ProtocolHTTP,
		BaseURL:  testGoJudgeHTTP,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	orchestrator := NewOrchestrator(client)

	// Simple C++ program that adds two numbers
	sourceCode := `#include <iostream>
using namespace std;

int main() {
    int a, b;
    cin >> a >> b;
    cout << a + b << endl;
    return 0;
}`

	ctx := context.Background()
	result, err := orchestrator.JudgeSolution(ctx, JudgeSolutionRequest{
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

func TestIntegrationHTTPCompileError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(ClientConfig{
		Protocol: ProtocolHTTP,
		BaseURL:  testGoJudgeHTTP,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	orchestrator := NewOrchestrator(client)

	// Invalid C++ code
	sourceCode := `#include <iostream>
int main() {
    this code will not compile
}`

	ctx := context.Background()
	result, err := orchestrator.JudgeSolution(ctx, JudgeSolutionRequest{
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

func TestIntegrationHTTPWrongAnswer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(ClientConfig{
		Protocol: ProtocolHTTP,
		BaseURL:  testGoJudgeHTTP,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	orchestrator := NewOrchestrator(client)

	// Program that outputs wrong answer
	sourceCode := `#include <iostream>
using namespace std;

int main() {
    int a, b;
    cin >> a >> b;
    cout << a - b << endl;  // Wrong: subtraction instead of addition
    return 0;
}`

	ctx := context.Background()
	result, err := orchestrator.JudgeSolution(ctx, JudgeSolutionRequest{
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

	if result.Verdict != "WA" {
		t.Errorf("Expected verdict WA, got %s", result.Verdict)
	}

	t.Logf("Wrong answer detected correctly")
}

func TestIntegrationCompileChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(ClientConfig{
		Protocol: ProtocolHTTP,
		BaseURL:  testGoJudgeHTTP,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	orchestrator := NewOrchestrator(client)

	// Simple checker that always returns OK
	checkerCode := `#include <iostream>
using namespace std;

int main(int argc, char* argv[]) {
    // Simple checker: always accept
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

	t.Logf("Checker compiled successfully: FileID=%s", binary.FileID)
}

func TestIntegrationGenerator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(ClientConfig{
		Protocol: ProtocolHTTP,
		BaseURL:  testGoJudgeHTTP,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	orchestrator := NewOrchestrator(client)

	// Generator that outputs N random numbers
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
	
	// Compile generator
	binary, err := orchestrator.CompileComponentFromSource(ctx, "generator", generatorCode, "cpp17", nil)
	if err != nil {
		t.Fatalf("Failed to compile generator: %v", err)
	}

	if !binary.Success {
		t.Errorf("Generator compilation failed: %s", binary.Error)
	}

	// Run generator
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

	t.Logf("Test generated successfully: %d bytes", len(generated.Input))
	t.Logf("Generated content: %s", string(generated.Input))
}
