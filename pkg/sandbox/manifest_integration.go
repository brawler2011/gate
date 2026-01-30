package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gate149/core/pkg/problemformat"
)

// LoadComponentFromManifest reads component metadata from manifest
func LoadComponentFromManifest(manifest *problemformat.ProblemManifest, componentType string) (*problemformat.FileMetadata, error) {
	for _, meta := range manifest.FilesMetadata {
		if meta.Type == componentType {
			return &meta, nil
		}
	}
	return nil, fmt.Errorf("component of type %s not found in manifest", componentType)
}

// LoadComponentSource reads component source code from problem directory
func LoadComponentSource(problemDir string, metadata *problemformat.FileMetadata) (string, error) {
	sourcePath := filepath.Join(problemDir, metadata.Filename)
	
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read component source file %s: %w", metadata.Filename, err)
	}
	
	return string(data), nil
}

// LoadDependencies reads testlib.h and other dependencies
func LoadDependencies(problemDir string, deps []problemformat.Dependency) (map[string]string, error) {
	dependencies := make(map[string]string)
	
	for _, dep := range deps {
		// Look for dependency in common locations
		possiblePaths := []string{
			filepath.Join(problemDir, dep.Filename),
			filepath.Join(problemDir, "lib", dep.Filename),
			filepath.Join(problemDir, "include", dep.Filename),
		}
		
		var content []byte
		var err error
		found := false
		
		for _, path := range possiblePaths {
			content, err = os.ReadFile(path)
			if err == nil {
				found = true
				break
			}
		}
		
		if !found {
			// If not found in problem dir, try to use system-wide testlib
			// For now, return error
			return nil, fmt.Errorf("dependency %s not found in problem directory", dep.Filename)
		}
		
		dependencies[dep.Filename] = string(content)
	}
	
	return dependencies, nil
}

// LoadAndCompileComponent loads a component from manifest and compiles it
func LoadAndCompileComponent(ctx context.Context, orchestrator *Orchestrator, problemDir string, manifest *problemformat.ProblemManifest, componentType string) (*ComponentBinary, error) {
	// Load component metadata
	metadata, err := LoadComponentFromManifest(manifest, componentType)
	if err != nil {
		return nil, err
	}
	
	// Load source code
	sourceCode, err := LoadComponentSource(problemDir, metadata)
	if err != nil {
		return nil, err
	}
	
	// Load dependencies
	dependencies, err := LoadDependencies(problemDir, metadata.Dependencies)
	if err != nil {
		// Dependencies are optional in some cases
		dependencies = make(map[string]string)
	}
	
	// Compile component
	req := ComponentCompileRequest{
		Type:         componentType,
		SourceCode:   sourceCode,
		Language:     metadata.Compiler,
		Dependencies: dependencies,
	}
	
	return orchestrator.compiler.CompileComponent(ctx, req)
}

// GetComponentExecutableName returns the executable name for a component type
func GetComponentExecutableName(componentType string) string {
	return componentType
}

// GetComponentSourceFileName returns the source filename for a component
func GetComponentSourceFileName(componentType, language string) string {
	langConfig, ok := GetLanguageConfig(NormalizeLanguageName(language))
	if !ok {
		return componentType + ".cpp" // default to .cpp
	}
	return componentType + langConfig.Extension
}

// ExtractLimitsFromManifest extracts resource limits from problem manifest
func ExtractLimitsFromManifest(manifest *problemformat.ProblemManifest) ResourceLimits {
	return ResourceLimits{
		CPUTimeMs: int64(manifest.TimeLimitMs),
		MemoryMB:  int64(manifest.MemoryLimitMb),
		ProcLimit: 1,
		StackMB:   256,
	}
}

// ValidateManifestComponent checks if a component in manifest is valid
func ValidateManifestComponent(metadata *problemformat.FileMetadata) error {
	if metadata.Type == "" {
		return fmt.Errorf("component type is empty")
	}
	
	if metadata.Filename == "" {
		return fmt.Errorf("component filename is empty")
	}
	
	if metadata.Compiler == "" {
		return fmt.Errorf("component compiler is not specified")
	}
	
	// Validate component type
	validTypes := []string{"checker", "validator", "generator", "interactor"}
	valid := false
	for _, t := range validTypes {
		if metadata.Type == t {
			valid = true
			break
		}
	}
	
	if !valid {
		return fmt.Errorf("invalid component type: %s (must be one of: %s)", 
			metadata.Type, strings.Join(validTypes, ", "))
	}
	
	return nil
}

// FindComponentByType finds a component in manifest by type
func FindComponentByType(manifest *problemformat.ProblemManifest, componentType string) *problemformat.FileMetadata {
	for _, meta := range manifest.FilesMetadata {
		if meta.Type == componentType {
			return &meta
		}
	}
	return nil
}

// HasComponent checks if manifest has a specific component type
func HasComponent(manifest *problemformat.ProblemManifest, componentType string) bool {
	return FindComponentByType(manifest, componentType) != nil
}

// ListAllComponents returns all components from manifest
func ListAllComponents(manifest *problemformat.ProblemManifest) []problemformat.FileMetadata {
	return manifest.FilesMetadata
}

// GetCheckerType determines the checker type (standard, custom, etc.)
func GetCheckerType(manifest *problemformat.ProblemManifest) string {
	checker := FindComponentByType(manifest, "checker")
	if checker == nil {
		return "none"
	}
	
	// If filename contains "std::" it's a standard checker
	if strings.HasPrefix(checker.Filename, "std::") {
		return "standard"
	}
	
	return "custom"
}

// IsInteractiveProblem checks if the problem is interactive
func IsInteractiveProblem(manifest *problemformat.ProblemManifest) bool {
	return manifest.ProblemType == "interactive" || HasComponent(manifest, "interactor")
}

// NeedsGenerator checks if any test needs a generator
func NeedsGenerator(testsMetadata *problemformat.TestsMetadata) bool {
	for _, test := range testsMetadata.Tests {
		if test.Method == "generated" && test.Generator != nil {
			return true
		}
	}
	return false
}

// ParseGeneratorCommand parses generator command from TestCase
// Example: "gen_border 1 2 3" -> command="gen_border", args=["1", "2", "3"]
func ParseGeneratorCommand(generatorStr string) (command string, args []string) {
	parts := strings.Fields(generatorStr)
	if len(parts) == 0 {
		return "", []string{}
	}
	return parts[0], parts[1:]
}
