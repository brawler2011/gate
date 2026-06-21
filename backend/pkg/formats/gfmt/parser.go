package gfmt

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Parser parses a GFMT package directory.
type Parser struct{}

// NewParser creates a new GFMT parser.
func NewParser() *Parser {
	return &Parser{}
}

// GetFormat returns the format name.
func (p *Parser) GetFormat() string {
	return "gfmt"
}

// Parse parses the GFMT problem.yaml configuration and maps all files in the package directory.
func (p *Parser) Parse(packageDir string) (*ImportPlan, error) {
	yamlPath := filepath.Join(packageDir, "problem.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.yaml: %w", err)
	}

	var prob Problem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse GFMT problem.yaml: %w", err)
	}

	var mappings []FileMapping
	err = filepath.WalkDir(packageDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(packageDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "problem.yaml" {
			return nil
		}
		mappings = append(mappings, FileMapping{
			SourcePath: rel,
			TargetPath: rel,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan gfmt directory: %w", err)
	}

	return &ImportPlan{
		Problem:  &prob,
		Mappings: mappings,
	}, nil
}
