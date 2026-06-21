package formats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var formats = []string{"gfmt", "polygon", "pcms2", "icpc"}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		pkgPath        string
		expectedFormat string
	}{
		{"../testdata/gfmt/a-plus-b", "gfmt"},
		{"../testdata/polygon/a-plus-b", "polygon"},
		{"../testdata/pcms2/a-plus-b", "pcms2"},
		{"../testdata/icpc/a-plus-b", "icpc"},
	}

	for _, tc := range tests {
		t.Run(tc.expectedFormat, func(t *testing.T) {
			path := tc.pkgPath
			detected, err := DetectFormat(path)
			if err != nil {
				t.Fatalf("unexpected error detecting format for %s: %v", path, err)
			}
			if detected != tc.expectedFormat {
				t.Errorf("expected format %q, got %q for path %s", tc.expectedFormat, detected, path)
			}
		})
	}
}

func TestParsers_APlusB(t *testing.T) {
	for _, fmtName := range formats {
		t.Run(fmtName, func(t *testing.T) {
			parser, err := GetParser(fmtName)
			if err != nil {
				t.Fatalf("failed to get parser: %v", err)
			}

			path := filepath.Join("..", "testdata", fmtName, "a-plus-b")
			plan, err := parser.Parse(path)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			prob := plan.Problem

			if prob.FormatVersion != "1.0" {
				t.Errorf("expected FormatVersion 1.0, got %q", prob.FormatVersion)
			}

			if prob.Title != "A+B" {
				t.Errorf("expected Title 'A+B', got %q", prob.Title)
			}

			if prob.Type != "pass-fail" {
				t.Errorf("expected Type 'pass-fail', got %q", prob.Type)
			}

			if prob.Limits.TimeMs != 1000 {
				t.Errorf("expected TimeMs 1000, got %d", prob.Limits.TimeMs)
			}

			if prob.Limits.MemoryMb != 256 {
				t.Errorf("expected MemoryMb 256, got %d", prob.Limits.MemoryMb)
			}

			// Validate subtasks
			samples, ok := prob.Subtasks["samples"]
			if !ok {
				t.Fatalf("missing 'samples' subtask")
			}
			if samples.Points != 0 {
				t.Errorf("expected samples points 0, got %d", samples.Points)
			}
			if samples.Policy != "complete" {
				t.Errorf("expected samples policy 'complete', got %q", samples.Policy)
			}
			if len(samples.Tests) != 3 {
				t.Errorf("expected 3 sample tests, got %d", len(samples.Tests))
			}

			secret, ok := prob.Subtasks["secret"]
			if !ok {
				t.Fatalf("missing 'secret' subtask")
			}
			if secret.Points != 100 {
				t.Errorf("expected secret points 100, got %d", secret.Points)
			}
			if secret.Policy != "each" {
				t.Errorf("expected secret policy 'each', got %q", secret.Policy)
			}
			expectedSecretLen := 9
			if fmtName == "gfmt" {
				expectedSecretLen = 6
			}
			if len(secret.Tests) != expectedSecretLen {
				t.Errorf("expected %d secret tests, got %d", expectedSecretLen, len(secret.Tests))
			}
		})
	}
}

func TestParsers_InteractiveGuess(t *testing.T) {
	for _, fmtName := range formats {
		t.Run(fmtName, func(t *testing.T) {
			parser, err := GetParser(fmtName)
			if err != nil {
				t.Fatalf("failed to get parser: %v", err)
			}

			path := filepath.Join("..", "testdata", fmtName, "interactive-guess")
			plan, err := parser.Parse(path)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			prob := plan.Problem

			if prob.Type != "interactive" {
				t.Errorf("expected Type 'interactive', got %q", prob.Type)
			}

			if prob.Limits.TimeMs != 2000 {
				t.Errorf("expected TimeMs 2000, got %d", prob.Limits.TimeMs)
			}

			// Validate subtasks
			samples, ok := prob.Subtasks["samples"]
			if !ok {
				t.Fatalf("missing 'samples' subtask")
			}
			if len(samples.Tests) != 1 {
				t.Errorf("expected 1 sample test, got %d", len(samples.Tests))
			}

			secret, ok := prob.Subtasks["secret"]
			if !ok {
				t.Fatalf("missing 'secret' subtask")
			}
			if len(secret.Tests) != 1 {
				t.Errorf("expected 1 secret test, got %d", len(secret.Tests))
			}
		})
	}
}

func TestParsers_MaxSubarray(t *testing.T) {
	for _, fmtName := range formats {
		t.Run(fmtName, func(t *testing.T) {
			parser, err := GetParser(fmtName)
			if err != nil {
				t.Fatalf("failed to get parser: %v", err)
			}

			path := filepath.Join("..", "testdata", fmtName, "max-subarray")
			plan, err := parser.Parse(path)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			prob := plan.Problem

			if prob.Type != "pass-fail" {
				t.Errorf("expected Type 'pass-fail', got %q", prob.Type)
			}

			// Validate subtasks
			samples, ok := prob.Subtasks["samples"]
			if !ok {
				t.Fatalf("missing 'samples' subtask")
			}
			if len(samples.Tests) != 2 {
				t.Errorf("expected 2 sample tests, got %d", len(samples.Tests))
			}

			secret, ok := prob.Subtasks["secret"]
			if !ok {
				t.Fatalf("missing 'secret' subtask")
			}
			if len(secret.Tests) == 0 {
				t.Errorf("expected secret tests, got 0")
			}
		})
	}
}

func TestParsers_SubtasksGroups(t *testing.T) {
	for _, fmtName := range formats {
		t.Run(fmtName, func(t *testing.T) {
			parser, err := GetParser(fmtName)
			if err != nil {
				t.Fatalf("failed to get parser: %v", err)
			}

			path := filepath.Join("..", "testdata", fmtName, "subtasks-groups")
			plan, err := parser.Parse(path)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			prob := plan.Problem

			if prob.Type != "scoring" {
				t.Errorf("expected Type 'scoring', got %q", prob.Type)
			}

			// Validate subtasks
			even, ok := prob.Subtasks["even_subtask"]
			if !ok {
				t.Fatalf("missing 'even_subtask' subtask")
			}
			if even.Points != 40 {
				t.Errorf("expected even_subtask points 40, got %d", even.Points)
			}
			if len(even.Tests) != 1 {
				t.Errorf("expected 1 test in even_subtask, got %d", len(even.Tests))
			}

			odd, ok := prob.Subtasks["odd_subtask"]
			if !ok {
				t.Fatalf("missing 'odd_subtask' subtask")
			}
			if odd.Points != 60 {
				t.Errorf("expected odd_subtask points 60, got %d", odd.Points)
			}
			if len(odd.Tests) != 1 {
				t.Errorf("expected 1 test in odd_subtask, got %d", len(odd.Tests))
			}
		})
	}
}

func TestImport(t *testing.T) {
	parser, err := GetParser("polygon")
	if err != nil {
		t.Fatalf("failed to get parser: %v", err)
	}

	tempDst, err := os.MkdirTemp("", "gfmt-import-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dst: %v", err)
	}
	defer os.RemoveAll(tempDst)

	srcPath := filepath.Join("..", "testdata", "polygon", "a-plus-b")
	
	// Clean up any generated markdown files from the source directory afterwards
	defer func() {
		os.Remove(filepath.Join(srcPath, "statements", "en.md"))
		os.Remove(filepath.Join(srcPath, "statements", "ru.md"))
	}()

	err = Import(srcPath, tempDst, parser)
	if err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	gfmtYamlPath := filepath.Join(tempDst, "problem.yaml")
	if _, err := os.Stat(gfmtYamlPath); err != nil {
		t.Errorf("problem.yaml not found in dst: %v", err)
	}

	checkerPath := filepath.Join(tempDst, "checkers", "checker.cpp")
	if _, err := os.Stat(checkerPath); err != nil {
		t.Errorf("checker.cpp not found in checkers/: %v", err)
	}

	testlibPath := filepath.Join(tempDst, "lib", "testlib.h")
	if _, err := os.Stat(testlibPath); err != nil {
		t.Errorf("testlib.h not found in lib/: %v", err)
	}

	t1InPath := filepath.Join(tempDst, "tests", "01.in")
	if _, err := os.Stat(t1InPath); err != nil {
		t.Errorf("01.in not found in tests/: %v", err)
	}

	// Verify English statement
	enPath := filepath.Join(tempDst, "statements", "en.md")
	if _, err := os.Stat(enPath); err != nil {
		t.Errorf("en.md not found in statements/: %v", err)
	} else {
		content, err := os.ReadFile(enPath)
		if err != nil {
			t.Fatalf("failed to read en.md: %v", err)
		}
		if !strings.Contains(string(content), "<!--legend -->") {
			t.Errorf("en.md missing legend header: %s", string(content))
		}
		if !strings.Contains(string(content), "Print $a+b$") {
			t.Errorf("en.md content incorrect: %s", string(content))
		}
	}

	// Verify Russian statement
	ruPath := filepath.Join(tempDst, "statements", "ru.md")
	if _, err := os.Stat(ruPath); err != nil {
		t.Errorf("ru.md not found in statements/: %v", err)
	} else {
		content, err := os.ReadFile(ruPath)
		if err != nil {
			t.Fatalf("failed to read ru.md: %v", err)
		}
		if !strings.Contains(string(content), "<!--legend -->") {
			t.Errorf("ru.md missing legend header: %s", string(content))
		}
		if !strings.Contains(string(content), "Вам заданы два целых числа") {
			t.Errorf("ru.md content incorrect: %s", string(content))
		}
	}
}
