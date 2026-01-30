package sandbox

import (
	"context"
	"testing"
)

func TestNormalizeLanguageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"c++11", "cpp11"},
		{"c++17", "cpp17"},
		{"cpp", "cpp17"},
		{"python", "python3"},
		{"go", "golang"},
		{"java", "java17"},
	}

	for _, test := range tests {
		result := NormalizeLanguageName(test.input)
		if result != test.expected {
			t.Errorf("NormalizeLanguageName(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestGetLanguageConfig(t *testing.T) {
	tests := []string{"cpp17", "python3", "golang", "java17", "c11"}

	for _, lang := range tests {
		config, ok := GetLanguageConfig(lang)
		if !ok {
			t.Errorf("GetLanguageConfig(%s) failed, expected success", lang)
		}
		if config.Name == "" {
			t.Errorf("GetLanguageConfig(%s) returned empty name", lang)
		}
	}
}

func TestLanguageConfigCompilation(t *testing.T) {
	tests := []struct {
		language         string
		needsCompilation bool
	}{
		{"cpp17", true},
		{"golang", true},
		{"java17", true},
		{"python3", false},
	}

	for _, test := range tests {
		config, ok := GetLanguageConfig(test.language)
		if !ok {
			t.Errorf("GetLanguageConfig(%s) failed", test.language)
			continue
		}
		if config.NeedsCompilation != test.needsCompilation {
			t.Errorf("Language %s: NeedsCompilation = %v, expected %v",
				test.language, config.NeedsCompilation, test.needsCompilation)
		}
	}
}

func TestCompilerGetLanguageExtension(t *testing.T) {
	// Create a mock client (nil is ok for this test since we're not calling Compile)
	client := &Client{}
	compiler := NewCompiler(client)

	tests := []struct {
		language string
		expected string
	}{
		{"cpp17", ".cpp"},
		{"python3", ".py"},
		{"golang", ".go"},
		{"java17", ".java"},
	}

	for _, test := range tests {
		ext := compiler.GetLanguageExtension(test.language)
		if ext != test.expected {
			t.Errorf("GetLanguageExtension(%s) = %s, expected %s", test.language, ext, test.expected)
		}
	}
}

func TestCompilerNeedsCompilation(t *testing.T) {
	client := &Client{}
	compiler := NewCompiler(client)

	tests := []struct {
		language string
		expected bool
	}{
		{"cpp17", true},
		{"python3", false},
		{"golang", true},
	}

	for _, test := range tests {
		needs := compiler.NeedsCompilation(test.language)
		if needs != test.expected {
			t.Errorf("NeedsCompilation(%s) = %v, expected %v", test.language, needs, test.expected)
		}
	}
}

func TestCompilerGetExecutionLimits(t *testing.T) {
	client := &Client{}
	compiler := NewCompiler(client)

	limits := compiler.GetExecutionLimits("cpp17")
	if limits.CPUTimeMs == 0 {
		t.Error("GetExecutionLimits returned zero CPUTimeMs")
	}
	if limits.MemoryMB == 0 {
		t.Error("GetExecutionLimits returned zero MemoryMB")
	}
}

func TestResourceLimitsConversion(t *testing.T) {
	limits := ResourceLimits{
		CPUTimeMs: 1000, // 1 second
		MemoryMB:  256,  // 256 MB
	}

	nanos := limits.ToNanoseconds()
	expectedNanos := int64(1000 * 1_000_000)
	if nanos != expectedNanos {
		t.Errorf("ToNanoseconds() = %d, expected %d", nanos, expectedNanos)
	}

	bytes := limits.ToBytes()
	expectedBytes := int64(256 * 1024 * 1024)
	if bytes != expectedBytes {
		t.Errorf("ToBytes() = %d, expected %d", bytes, expectedBytes)
	}
}

func TestComputeSHA256(t *testing.T) {
	content := []byte("test content")
	hash := ComputeSHA256(content)
	
	if len(hash) != 64 { // SHA256 is 32 bytes = 64 hex characters
		t.Errorf("ComputeSHA256 returned hash of length %d, expected 64", len(hash))
	}
	
	// Same content should produce same hash
	hash2 := ComputeSHA256(content)
	if hash != hash2 {
		t.Error("ComputeSHA256 returned different hashes for same content")
	}
	
	// Different content should produce different hash
	hash3 := ComputeSHA256([]byte("different content"))
	if hash == hash3 {
		t.Error("ComputeSHA256 returned same hash for different content")
	}
}

func TestBuildCommand(t *testing.T) {
	template := []string{"g++", "-o", "{output}", "{source}"}
	replacements := map[string]string{
		"{source}": "solution.cpp",
		"{output}": "solution",
	}
	
	result := buildCommand(template, replacements)
	expected := []string{"g++", "-o", "solution", "solution.cpp"}
	
	if len(result) != len(expected) {
		t.Errorf("buildCommand returned %d elements, expected %d", len(result), len(expected))
	}
	
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("buildCommand[%d] = %s, expected %s", i, result[i], expected[i])
		}
	}
}

// Mock tests for CompileComponent (without actual go-judge server)
func TestCompileComponentInterpretedLanguage(t *testing.T) {
	// For interpreted languages, compilation should succeed immediately
	client := &Client{protocol: ProtocolHTTP}
	compiler := NewCompiler(client)
	
	req := ComponentCompileRequest{
		Type:       "checker",
		SourceCode: "print('Hello')",
		Language:   "python3",
	}
	
	result, err := compiler.CompileComponent(context.Background(), req)
	if err != nil {
		t.Errorf("CompileComponent failed: %v", err)
	}
	
	if !result.Success {
		t.Error("CompileComponent for Python should succeed without actual compilation")
	}
	
	if result.FileID != "" {
		t.Error("CompileComponent for Python should not return FileID")
	}
}
