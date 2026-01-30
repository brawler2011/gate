package sandbox

import (
	"context"
	"fmt"
	"strings"
)

// Compiler handles compilation of problem components
type Compiler struct {
	client    *Client
	languages map[string]LanguageConfig
}

// NewCompiler creates a new compiler
func NewCompiler(client *Client) *Compiler {
	return &Compiler{
		client:    client,
		languages: DefaultLanguages(),
	}
}

// CompileComponent compiles a problem component (checker/validator/generator/interactor)
func (c *Compiler) CompileComponent(ctx context.Context, req ComponentCompileRequest) (*ComponentBinary, error) {
	// Normalize language name
	language := NormalizeLanguageName(req.Language)
	langConfig, ok := c.languages[language]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", req.Language)
	}

	// Determine source filename and output filename based on component type and language
	sourceFile := fmt.Sprintf("%s%s", req.Type, langConfig.Extension)
	outputFile := req.Type

	// For interpreted languages, we don't need to compile
	if !langConfig.NeedsCompilation {
		// For Python and other interpreted languages, just validate syntax
		// by attempting a "compile" check (if needed in the future)
		return &ComponentBinary{
			FileID:       "", // No FileID for interpreted languages
			BinarySHA256: ComputeSHA256([]byte(req.SourceCode)),
			CompileLog:   "Interpreted language, no compilation needed",
			Success:      true,
		}, nil
	}

	// Compile the component
	compileReq := CompileRequest{
		SourceCode:   req.SourceCode,
		Language:     language,
		SourceFile:   sourceFile,
		OutputFile:   outputFile,
		Dependencies: req.Dependencies,
		Limits:       langConfig.CompileLimits,
	}

	result, err := c.client.Compile(ctx, compileReq)
	if err != nil {
		return &ComponentBinary{
			Success:    false,
			Error:      fmt.Sprintf("compilation failed: %v", err),
			CompileLog: "",
		}, nil
	}

	if !result.Success {
		return &ComponentBinary{
			Success:      false,
			Error:        "compilation failed",
			CompileLog:   fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", result.Stdout, result.Stderr),
			BinarySHA256: "",
		}, nil
	}

	// For compiled languages, we need to compute the SHA256 of the binary
	// In practice, we might need to download the binary from go-judge to compute the hash
	// For now, we'll use a placeholder approach
	binarySHA256 := result.FileID // Use FileID as a proxy for now

	return &ComponentBinary{
		FileID:       result.FileID,
		BinarySHA256: binarySHA256,
		CompileLog:   fmt.Sprintf("Compilation successful\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr),
		Success:      true,
	}, nil
}

// CompileSolution compiles a user solution
func (c *Compiler) CompileSolution(ctx context.Context, sourceCode, language string, limits ResourceLimits) (*ComponentBinary, error) {
	// Normalize language name
	normalizedLang := NormalizeLanguageName(language)
	langConfig, ok := c.languages[normalizedLang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// Determine source filename and output filename
	sourceFile := fmt.Sprintf("solution%s", langConfig.Extension)
	outputFile := "solution"

	// For interpreted languages
	if !langConfig.NeedsCompilation {
		return &ComponentBinary{
			FileID:       "",
			BinarySHA256: ComputeSHA256([]byte(sourceCode)),
			CompileLog:   "Interpreted language, no compilation needed",
			Success:      true,
		}, nil
	}

	// Use provided limits or default compile limits
	if limits.CPUTimeMs == 0 {
		limits = langConfig.CompileLimits
	}

	// Compile the solution
	compileReq := CompileRequest{
		SourceCode:   sourceCode,
		Language:     normalizedLang,
		SourceFile:   sourceFile,
		OutputFile:   outputFile,
		Dependencies: make(map[string]string), // Solutions typically don't have dependencies
		Limits:       limits,
	}

	result, err := c.client.Compile(ctx, compileReq)
	if err != nil {
		return &ComponentBinary{
			Success:    false,
			Error:      fmt.Sprintf("compilation failed: %v", err),
			CompileLog: "",
		}, nil
	}

	if !result.Success {
		return &ComponentBinary{
			Success:      false,
			Error:        "compilation failed",
			CompileLog:   fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", result.Stdout, result.Stderr),
			BinarySHA256: "",
		}, nil
	}

	return &ComponentBinary{
		FileID:       result.FileID,
		BinarySHA256: result.FileID, // Use FileID as proxy
		CompileLog:   fmt.Sprintf("Compilation successful\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr),
		Success:      true,
	}, nil
}

// GetLanguageExtension returns the file extension for a given language
func (c *Compiler) GetLanguageExtension(language string) string {
	normalizedLang := NormalizeLanguageName(language)
	if config, ok := c.languages[normalizedLang]; ok {
		return config.Extension
	}
	return ""
}

// GetExecutionLimits returns the default execution limits for a language
func (c *Compiler) GetExecutionLimits(language string) ResourceLimits {
	normalizedLang := NormalizeLanguageName(language)
	if config, ok := c.languages[normalizedLang]; ok {
		return config.ExecuteLimits
	}
	return ResourceLimits{
		CPUTimeMs: 5000,
		MemoryMB:  256,
		ProcLimit: 1,
		StackMB:   256,
	}
}

// NeedsCompilation returns whether a language needs compilation
func (c *Compiler) NeedsCompilation(language string) bool {
	normalizedLang := NormalizeLanguageName(language)
	if config, ok := c.languages[normalizedLang]; ok {
		return config.NeedsCompilation
	}
	return false
}

// GetExecuteCommand returns the execute command for a language
func (c *Compiler) GetExecuteCommand(language, executableName string) []string {
	normalizedLang := NormalizeLanguageName(language)
	if config, ok := c.languages[normalizedLang]; ok {
		// Replace placeholders in execute command
		cmd := make([]string, len(config.ExecuteCommand))
		for i, part := range config.ExecuteCommand {
			cmd[i] = strings.ReplaceAll(part, "{executable}", executableName)
			cmd[i] = strings.ReplaceAll(cmd[i], "{source}", executableName+config.Extension)
			// Extract class name from source file for Java
			if normalizedLang == "java11" || normalizedLang == "java17" {
				className := strings.TrimSuffix(executableName, config.Extension)
				cmd[i] = strings.ReplaceAll(cmd[i], "{class}", className)
			}
		}
		return cmd
	}
	return []string{"./" + executableName}
}
