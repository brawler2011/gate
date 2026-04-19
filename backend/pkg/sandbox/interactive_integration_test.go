//go:build integration
// +build integration

package sandbox

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestIntegrationInteractiveExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	orchestrator := getIntegrationOrchestrator(t)
	ctx := context.Background()

	verifierInteractorID, err := compileInteractorWithRetry(ctx, orchestrator, verifierInteractorSource())
	if err != nil {
		t.Fatalf("Failed to compile verifier interactor: %v", err)
	}

	t.Run("HappyPath", func(t *testing.T) {
		result, err := judgeInteractiveWithRetry(ctx, orchestrator, JudgeSolutionRequest{
			SolutionCode:     interactiveEchoSolutionSource(),
			SolutionLanguage: "cpp17",
			InteractorFileID: verifierInteractorID,
			Input:            []byte("7\n"),
			Answer:           []byte("7\n"),
			TimeLimitMs:      1000,
			MemoryLimitMB:    256,
		})
		if err != nil {
			t.Fatalf("JudgeSolutionInteractive failed: %v", err)
		}

		if result.Verdict != "OK" {
			t.Fatalf("Expected verdict OK, got %s (%s)", result.Verdict, result.Message)
		}
	})

	t.Run("WrongAnswer", func(t *testing.T) {
		result, err := judgeInteractiveWithRetry(ctx, orchestrator, JudgeSolutionRequest{
			SolutionCode:     interactiveWrongAnswerSolutionSource(),
			SolutionLanguage: "cpp17",
			InteractorFileID: verifierInteractorID,
			Input:            []byte("7\n"),
			Answer:           []byte("7\n"),
			TimeLimitMs:      1000,
			MemoryLimitMB:    256,
		})
		if err != nil {
			t.Fatalf("JudgeSolutionInteractive failed: %v", err)
		}

		if result.Verdict != "WA" {
			t.Fatalf("Expected verdict WA, got %s (%s)", result.Verdict, result.Message)
		}
	})

	t.Run("SolutionTimeLimitExceeded", func(t *testing.T) {
		result, err := judgeInteractiveWithRetry(ctx, orchestrator, JudgeSolutionRequest{
			SolutionCode:     interactiveBusyLoopSolutionSource(),
			SolutionLanguage: "cpp17",
			InteractorFileID: verifierInteractorID,
			Input:            []byte("7\n"),
			Answer:           []byte("7\n"),
			TimeLimitMs:      250,
			MemoryLimitMB:    256,
		})
		if err != nil {
			t.Fatalf("JudgeSolutionInteractive failed: %v", err)
		}

		if result.Verdict != "TLE" {
			t.Fatalf("Expected verdict TLE, got %s (%s)", result.Verdict, result.Message)
		}
	})

	t.Run("InteractorTimeLimitExceeded", func(t *testing.T) {
		interactorID, compileErr := compileInteractorWithRetry(ctx, orchestrator, interactorBusyLoopSource())
		if compileErr != nil {
			t.Fatalf("Failed to compile timeout interactor: %v", compileErr)
		}

		result, err := judgeInteractiveWithRetry(ctx, orchestrator, JudgeSolutionRequest{
			SolutionCode:     interactiveFastExitSolutionSource(),
			SolutionLanguage: "cpp17",
			InteractorFileID: interactorID,
			Input:            []byte("1\n"),
			Answer:           []byte("1\n"),
			TimeLimitMs:      250,
			MemoryLimitMB:    256,
		})
		if err != nil {
			t.Fatalf("JudgeSolutionInteractive failed: %v", err)
		}

		if result.Verdict != "TLE" {
			t.Fatalf("Expected verdict TLE, got %s (%s)", result.Verdict, result.Message)
		}
	})

	t.Run("DeadlockTimeout", func(t *testing.T) {
		interactorID, compileErr := compileInteractorWithRetry(ctx, orchestrator, deadlockInteractorSource())
		if compileErr != nil {
			t.Fatalf("Failed to compile deadlock interactor: %v", compileErr)
		}

		result, err := judgeInteractiveWithRetry(ctx, orchestrator, JudgeSolutionRequest{
			SolutionCode:     deadlockSolutionSource(),
			SolutionLanguage: "cpp17",
			InteractorFileID: interactorID,
			Input:            []byte("1\n"),
			Answer:           []byte("1\n"),
			TimeLimitMs:      250,
			MemoryLimitMB:    256,
		})
		if err != nil {
			t.Fatalf("JudgeSolutionInteractive failed: %v", err)
		}

		if result.Verdict != "TLE" {
			t.Fatalf("Expected verdict TLE, got %s (%s)", result.Verdict, result.Message)
		}
	})
}

func compileInteractorWithRetry(ctx context.Context, orchestrator *Orchestrator, source string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		binary, err := orchestrator.CompileComponentFromSource(ctx, "interactor", source, "cpp17", nil)
		if err != nil {
			lastErr = err
			break
		}
		if binary.Success {
			return binary.FileID, nil
		}
		if strings.Contains(binary.CompileLog, "Resource temporarily unavailable") {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}
		return "", fmt.Errorf("interactor compilation failed: %s", binary.CompileLog)
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("interactor compilation failed after retries")
}

func judgeInteractiveWithRetry(ctx context.Context, orchestrator *Orchestrator, req JudgeSolutionRequest) (*JudgeResult, error) {
	var lastResult *JudgeResult
	for attempt := 0; attempt < 4; attempt++ {
		result, err := orchestrator.JudgeSolutionInteractive(ctx, req)
		if err != nil {
			return nil, err
		}
		lastResult = result

		if result == nil || result.Verdict != "CE" || !strings.Contains(result.CompileError, "Resource temporarily unavailable") {
			return result, nil
		}

		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}

	return lastResult, nil
}

func verifierInteractorSource() string {
	return `#include <fstream>
#include <iostream>

int main(int argc, char* argv[]) {
    if (argc < 2) {
        std::cerr << "missing input file" << std::endl;
        return 3;
    }

    std::ifstream in(argv[1]);
    long long expected = 0;
    if (!(in >> expected)) {
        std::cerr << "bad input" << std::endl;
        return 3;
    }

    std::cout << expected << std::endl;

    long long reply = 0;
    if (!(std::cin >> reply)) {
        std::cerr << "no reply" << std::endl;
        return 1;
    }

    if (reply != expected) {
        std::cerr << "wrong reply" << std::endl;
        return 1;
    }

    return 0;
}
`
}

func interactorBusyLoopSource() string {
	return `#include <cstdint>

int main() {
    volatile std::uint64_t counter = 0;
    while (true) {
        ++counter;
    }
    return 0;
}
`
}

func deadlockInteractorSource() string {
	return `#include <iostream>
#include <string>

int main() {
    std::string token;
    std::cin >> token;
    return 0;
}
`
}

func interactiveEchoSolutionSource() string {
	return `#include <iostream>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);

    long long x = 0;
    if (!(std::cin >> x)) {
        return 0;
    }

    std::cout << x << std::endl;
    return 0;
}
`
}

func interactiveWrongAnswerSolutionSource() string {
	return `#include <iostream>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);

    long long x = 0;
    if (!(std::cin >> x)) {
        return 0;
    }

    std::cout << (x + 1) << std::endl;
    return 0;
}
`
}

func interactiveBusyLoopSolutionSource() string {
	return `#include <cstdint>

int main() {
    volatile std::uint64_t counter = 0;
    while (true) {
        ++counter;
    }
    return 0;
}
`
}

func interactiveFastExitSolutionSource() string {
	return `#include <iostream>

int main() {
    std::cout << 1 << std::endl;
    return 0;
}
`
}

func deadlockSolutionSource() string {
	return `#include <iostream>
#include <string>

int main() {
    std::string token;
    std::cin >> token;
    return 0;
}
`
}
