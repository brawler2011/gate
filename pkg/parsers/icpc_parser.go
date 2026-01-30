package parsers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gate149/core/pkg/problemformat"
	"gopkg.in/yaml.v3"
)

// ICPCProblem represents the structure of ICPC's problem.yaml
type ICPCProblem struct {
	Name   string      `yaml:"name"`
	Author string      `yaml:"author"`
	Source string      `yaml:"source"`
	Rights string      `yaml:"rights"`
	Limits ICPCLimits  `yaml:"limits"`
	Validation ICPCValidation `yaml:"validation"`
}

type ICPCLimits struct {
	TimeMultiplier float64 `yaml:"time_multiplier"`
	TimeSeconds    float64 `yaml:"time"`
	MemoryMB       int     `yaml:"memory"`
	OutputMB       int     `yaml:"output"`
	CodeKB         int     `yaml:"code"`
	CompilationSeconds float64 `yaml:"compilation_time"`
	ValidationSeconds  float64 `yaml:"validation_time"`
	ValidationOutputMB int     `yaml:"validation_output"`
}

type ICPCValidation struct {
	Type string `yaml:"type"` // "default", "custom", "interactive"
}

// ICPCParser implements the ProblemParser interface for ICPC format
type ICPCParser struct{}

func NewICPCParser() *ICPCParser {
	return &ICPCParser{}
}

func (p *ICPCParser) Parse(packageDir string) (*problemformat.ProblemManifest, *problemformat.TestsMetadata, error) {
	// Parse problem.yaml
	yamlPath := filepath.Join(packageDir, "problem.yaml")
	icpcProb, err := parseICPCYaml(yamlPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse problem.yaml: %w", err)
	}

	// Convert to unified format
	manifest, err := convertICPCToManifest(icpcProb, packageDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert manifest: %w", err)
	}

	testsMetadata, err := convertICPCTests(packageDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert tests: %w", err)
	}

	return manifest, testsMetadata, nil
}

func (p *ICPCParser) GetFormat() string {
	return "icpc"
}

func parseICPCYaml(yamlPath string) (*ICPCProblem, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var prob ICPCProblem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &prob, nil
}

func convertICPCToManifest(prob *ICPCProblem, packageDir string) (*problemformat.ProblemManifest, error) {
	// Convert time limit to milliseconds
	timeLimitMs := int(prob.Limits.TimeSeconds * 1000)
	if timeLimitMs == 0 {
		timeLimitMs = 1000 // Default 1 second
	}

	// Memory limit in MB
	memoryLimitMb := prob.Limits.MemoryMB
	if memoryLimitMb == 0 {
		memoryLimitMb = 256 // Default 256 MB
	}

	// Output limit in MB
	stdoutLimitMb := prob.Limits.OutputMB
	if stdoutLimitMb == 0 {
		stdoutLimitMb = 64 // Default 64 MB
	}

	// Code size limit in KB
	codeSizeLimitKb := prob.Limits.CodeKB
	if codeSizeLimitKb == 0 {
		codeSizeLimitKb = 256 // Default 256 KB
	}

	// Determine problem type
	problemType := "pass-fail"
	if prob.Validation.Type == "interactive" {
		problemType = "interactive"
	}

	// Parse statements from statement/ directory
	statements, err := convertICPCStatements(packageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse statements: %w", err)
	}

	// Find validators and checkers
	filesMetadata := findICPCExecutables(packageDir)

	return &problemformat.ProblemManifest{
		LastUpdated:     time.Now(),
		ProblemType:     problemType,
		MaxScore:        nil, // ICPC doesn't use scoring by default
		FilesMetadata:   filesMetadata,
		TimeLimitMs:     timeLimitMs,
		MemoryLimitMb:   memoryLimitMb,
		StdoutLimitMb:   stdoutLimitMb,
		CodeSizeLimitKb: codeSizeLimitKb,
		Statements:      statements,
	}, nil
}

func convertICPCStatements(packageDir string) (map[string]problemformat.Statement, error) {
	statements := make(map[string]problemformat.Statement)
	stmtDir := filepath.Join(packageDir, "statement")

	// Check if statement directory exists
	if _, err := os.Stat(stmtDir); os.IsNotExist(err) {
		// No statements, return empty map
		return statements, nil
	}

	// Read all language directories
	entries, err := os.ReadDir(stmtDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read statement directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		lang := entry.Name()
		langDir := filepath.Join(stmtDir, lang)

		// Read problem.tex or problem.md
		var content string
		texPath := filepath.Join(langDir, "problem.tex")
		mdPath := filepath.Join(langDir, "problem.md")

		if data, err := os.ReadFile(texPath); err == nil {
			content = string(data)
		} else if data, err := os.ReadFile(mdPath); err == nil {
			content = string(data)
		} else {
			continue
		}

		// Parse LaTeX/Markdown sections (simplified)
		statements[lang] = problemformat.Statement{
			Title:        "", // Would need to parse from content
			Legend:       content,
			InputFormat:  "",
			OutputFormat: "",
			Notes:        "",
			Interaction:  "",
			Scoring:      "",
		}
	}

	return statements, nil
}

func convertICPCTests(packageDir string) (*problemformat.TestsMetadata, error) {
	dataDir := filepath.Join(packageDir, "data")
	
	// Read sample tests
	sampleDir := filepath.Join(dataDir, "sample")
	samples, err := readICPCTestFiles(sampleDir, true)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read sample tests: %w", err)
	}

	// Read secret tests
	secretDir := filepath.Join(dataDir, "secret")
	secrets, err := readICPCTestFiles(secretDir, false)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read secret tests: %w", err)
	}

	// Combine all tests
	allTests := append(samples, secrets...)

	// Create a single group for all tests (ICPC typically doesn't use groups)
	groups := []problemformat.TestGroup{
		{
			Ordinal:      0,
			Name:         "all",
			Points:       0,
			PointsPolicy: "complete-group",
			DependsOn:    []int{},
			Tests:        [2]int{1, len(allTests)},
		},
	}

	return &problemformat.TestsMetadata{
		Groups: groups,
		Tests:  allTests,
	}, nil
}

func readICPCTestFiles(dir string, isSample bool) ([]problemformat.TestCase, error) {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []problemformat.TestCase{}, nil
	}

	// Read all .in files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	tests := make([]problemformat.TestCase, 0)
	ordinal := 1

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".in") {
			continue
		}

		// Check if corresponding .ans file exists
		ansName := strings.TrimSuffix(name, ".in") + ".ans"
		ansPath := filepath.Join(dir, ansName)
		if _, err := os.Stat(ansPath); os.IsNotExist(err) {
			continue
		}

		tests = append(tests, problemformat.TestCase{
			Ordinal:   ordinal,
			Method:    "manual",
			Generator: nil,
			IsSample:  isSample,
		})
		ordinal++
	}

	return tests, nil
}

func findICPCExecutables(packageDir string) []problemformat.FileMetadata {
	filesMetadata := make([]problemformat.FileMetadata, 0)

	// Look for output validators in output_validators/
	validatorsDir := filepath.Join(packageDir, "output_validators")
	if entries, err := os.ReadDir(validatorsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			if ext == ".cpp" || ext == ".py" || ext == ".java" {
				filesMetadata = append(filesMetadata, problemformat.FileMetadata{
					Type:         "checker",
					Filename:     filepath.Join("output_validators", name),
					Compiler:     detectCompiler(name),
					BinarySha256: nil,
					Dependencies: []problemformat.Dependency{},
				})
			}
		}
	}

	// Look for input validators in input_validators/
	inputValidatorsDir := filepath.Join(packageDir, "input_validators")
	if entries, err := os.ReadDir(inputValidatorsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			if ext == ".cpp" || ext == ".py" || ext == ".java" {
				filesMetadata = append(filesMetadata, problemformat.FileMetadata{
					Type:         "validator",
					Filename:     filepath.Join("input_validators", name),
					Compiler:     detectCompiler(name),
					BinarySha256: nil,
					Dependencies: []problemformat.Dependency{},
				})
			}
		}
	}

	// Look for generators in generators/
	generatorsDir := filepath.Join(packageDir, "generators")
	if entries, err := os.ReadDir(generatorsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			if ext == ".cpp" || ext == ".py" || ext == ".java" {
				filesMetadata = append(filesMetadata, problemformat.FileMetadata{
					Type:         "generator",
					Filename:     filepath.Join("generators", name),
					Compiler:     detectCompiler(name),
					BinarySha256: nil,
					Dependencies: []problemformat.Dependency{},
				})
			}
		}
	}

	return filesMetadata
}
