package problemformat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTestData_ReadsCanonicalNames(t *testing.T) {
	problemDir := t.TempDir()
	testsDir := filepath.Join(problemDir, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("failed to create tests dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(testsDir, "01.in"), []byte("1 2\n"), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "01.out"), []byte("3\n"), 0o644); err != nil {
		t.Fatalf("failed to write output: %v", err)
	}

	input, output, err := LoadTestData(problemDir, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(input) != "1 2\n" {
		t.Fatalf("unexpected input: %q", string(input))
	}
	if string(output) != "3\n" {
		t.Fatalf("unexpected output: %q", string(output))
	}
}

func TestLoadTestData_RejectsNonPaddedNames(t *testing.T) {
	problemDir := t.TempDir()
	testsDir := filepath.Join(problemDir, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("failed to create tests dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(testsDir, "1.in"), []byte("4 5\n"), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "1.out"), []byte("9\n"), 0o644); err != nil {
		t.Fatalf("failed to write output: %v", err)
	}

	_, _, err := LoadTestData(problemDir, 1)
	if err == nil {
		t.Fatalf("expected error for non-padded test files")
	}
	if !strings.Contains(err.Error(), "01.in") {
		t.Fatalf("expected error to mention canonical filename, got %v", err)
	}
}
