package sandbox

import (
	"github.com/gate149/core/pkg/problemformat"
	"testing"
)

func TestFindComponentByType(t *testing.T) {
	manifest := &problemformat.ProblemManifest{
		FilesMetadata: []problemformat.FileMetadata{
			{
				Type:     "checker",
				Filename: "checker.cpp",
				Compiler: "cpp17",
			},
			{
				Type:     "validator",
				Filename: "validator.cpp",
				Compiler: "cpp17",
			},
		},
	}
	
	checker := FindComponentByType(manifest, "checker")
	if checker == nil {
		t.Error("FindComponentByType failed to find checker")
	}
	if checker != nil && checker.Filename != "checker.cpp" {
		t.Errorf("FindComponentByType returned wrong component: %s", checker.Filename)
	}
	
	generator := FindComponentByType(manifest, "generator")
	if generator != nil {
		t.Error("FindComponentByType should return nil for non-existent component")
	}
}

func TestHasComponent(t *testing.T) {
	manifest := &problemformat.ProblemManifest{
		FilesMetadata: []problemformat.FileMetadata{
			{
				Type:     "checker",
				Filename: "checker.cpp",
				Compiler: "cpp17",
			},
		},
	}
	
	if !HasComponent(manifest, "checker") {
		t.Error("HasComponent should return true for checker")
	}
	
	if HasComponent(manifest, "generator") {
		t.Error("HasComponent should return false for generator")
	}
}

func TestValidateManifestComponent(t *testing.T) {
	tests := []struct {
		name      string
		component problemformat.FileMetadata
		wantError bool
	}{
		{
			name: "valid checker",
			component: problemformat.FileMetadata{
				Type:     "checker",
				Filename: "checker.cpp",
				Compiler: "cpp17",
			},
			wantError: false,
		},
		{
			name: "missing type",
			component: problemformat.FileMetadata{
				Type:     "",
				Filename: "checker.cpp",
				Compiler: "cpp17",
			},
			wantError: true,
		},
		{
			name: "missing filename",
			component: problemformat.FileMetadata{
				Type:     "checker",
				Filename: "",
				Compiler: "cpp17",
			},
			wantError: true,
		},
		{
			name: "missing compiler",
			component: problemformat.FileMetadata{
				Type:     "checker",
				Filename: "checker.cpp",
				Compiler: "",
			},
			wantError: true,
		},
		{
			name: "invalid type",
			component: problemformat.FileMetadata{
				Type:     "invalid",
				Filename: "file.cpp",
				Compiler: "cpp17",
			},
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateManifestComponent(&tt.component)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateManifestComponent() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGetCheckerType(t *testing.T) {
	tests := []struct {
		name     string
		manifest *problemformat.ProblemManifest
		expected string
	}{
		{
			name: "no checker",
			manifest: &problemformat.ProblemManifest{
				FilesMetadata: []problemformat.FileMetadata{},
			},
			expected: "none",
		},
		{
			name: "custom checker",
			manifest: &problemformat.ProblemManifest{
				FilesMetadata: []problemformat.FileMetadata{
					{
						Type:     "checker",
						Filename: "checker.cpp",
						Compiler: "cpp17",
					},
				},
			},
			expected: "custom",
		},
		{
			name: "standard checker",
			manifest: &problemformat.ProblemManifest{
				FilesMetadata: []problemformat.FileMetadata{
					{
						Type:     "checker",
						Filename: "std::wcmp",
						Compiler: "cpp17",
					},
				},
			},
			expected: "standard",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCheckerType(tt.manifest)
			if result != tt.expected {
				t.Errorf("GetCheckerType() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestIsInteractiveProblem(t *testing.T) {
	tests := []struct {
		name     string
		manifest *problemformat.ProblemManifest
		expected bool
	}{
		{
			name: "interactive by type",
			manifest: &problemformat.ProblemManifest{
				ProblemType:   "interactive",
				FilesMetadata: []problemformat.FileMetadata{},
			},
			expected: true,
		},
		{
			name: "interactive by interactor",
			manifest: &problemformat.ProblemManifest{
				ProblemType: "pass-fail",
				FilesMetadata: []problemformat.FileMetadata{
					{
						Type:     "interactor",
						Filename: "interactor.cpp",
						Compiler: "cpp17",
					},
				},
			},
			expected: true,
		},
		{
			name: "not interactive",
			manifest: &problemformat.ProblemManifest{
				ProblemType:   "pass-fail",
				FilesMetadata: []problemformat.FileMetadata{},
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInteractiveProblem(tt.manifest)
			if result != tt.expected {
				t.Errorf("IsInteractiveProblem() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestParseGeneratorCommand(t *testing.T) {
	tests := []struct {
		input           string
		expectedCommand string
		expectedArgs    []string
	}{
		{
			input:           "gen_border 1 2 3",
			expectedCommand: "gen_border",
			expectedArgs:    []string{"1", "2", "3"},
		},
		{
			input:           "generator",
			expectedCommand: "generator",
			expectedArgs:    []string{},
		},
		{
			input:           "",
			expectedCommand: "",
			expectedArgs:    []string{},
		},
		{
			input:           "gen --seed 42 --size 1000",
			expectedCommand: "gen",
			expectedArgs:    []string{"--seed", "42", "--size", "1000"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			command, args := ParseGeneratorCommand(tt.input)
			if command != tt.expectedCommand {
				t.Errorf("command = %s, expected %s", command, tt.expectedCommand)
			}
			if len(args) != len(tt.expectedArgs) {
				t.Errorf("len(args) = %d, expected %d", len(args), len(tt.expectedArgs))
			}
			for i := range args {
				if args[i] != tt.expectedArgs[i] {
					t.Errorf("args[%d] = %s, expected %s", i, args[i], tt.expectedArgs[i])
				}
			}
		})
	}
}

func TestExtractLimitsFromManifest(t *testing.T) {
	manifest := &problemformat.ProblemManifest{
		TimeLimitMs:   2000,
		MemoryLimitMb: 512,
	}
	
	limits := ExtractLimitsFromManifest(manifest)
	
	if limits.CPUTimeMs != 2000 {
		t.Errorf("CPUTimeMs = %d, expected 2000", limits.CPUTimeMs)
	}
	
	if limits.MemoryMB != 512 {
		t.Errorf("MemoryMB = %d, expected 512", limits.MemoryMB)
	}
	
	if limits.ProcLimit != 1 {
		t.Errorf("ProcLimit = %d, expected 1", limits.ProcLimit)
	}
}

func TestNeedsGenerator(t *testing.T) {
	tests := []struct {
		name          string
		testsMetadata *problemformat.TestsMetadata
		expected      bool
	}{
		{
			name: "has generated test",
			testsMetadata: &problemformat.TestsMetadata{
				Tests: []problemformat.TestCase{
					{
						Ordinal:   1,
						Method:    "manual",
						Generator: nil,
					},
					{
						Ordinal:   2,
						Method:    "generated",
						Generator: strPtr("gen 1 2 3"),
					},
				},
			},
			expected: true,
		},
		{
			name: "only manual tests",
			testsMetadata: &problemformat.TestsMetadata{
				Tests: []problemformat.TestCase{
					{
						Ordinal:   1,
						Method:    "manual",
						Generator: nil,
					},
				},
			},
			expected: false,
		},
		{
			name: "no tests",
			testsMetadata: &problemformat.TestsMetadata{
				Tests: []problemformat.TestCase{},
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsGenerator(tt.testsMetadata)
			if result != tt.expected {
				t.Errorf("NeedsGenerator() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
