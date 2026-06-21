package gfmt

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GateFormat is a wrapper around a GFMT problem directory that provides
// safe and easy access to problem assets, solutions, and tests.
type GateFormat struct {
	Path    string
	Problem *Problem
}

// OpenPackage parses problem.yaml and returns a GateFormat instance for the package.
func OpenPackage(dir string) (*GateFormat, error) {
	yamlPath := filepath.Join(dir, "problem.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.yaml: %w", err)
	}

	var prob Problem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse problem.yaml: %w", err)
	}

	return &GateFormat{
		Path:    dir,
		Problem: &prob,
	}, nil
}

// GetSolution reads the content of the specified solution file.
func (g *GateFormat) GetSolution(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.Path, "solutions", name))
}

// GetChecker reads the content of the specified checker file.
func (g *GateFormat) GetChecker(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.Path, "checkers", name))
}

// GetGenerator reads the content of the specified generator file.
func (g *GateFormat) GetGenerator(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.Path, "generators", name))
}

// GetInteractor reads the content of the specified interactor file.
func (g *GateFormat) GetInteractor(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.Path, "interactors", name))
}

// GetLib reads the content of the specified library file.
func (g *GateFormat) GetLib(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.Path, "lib", name))
}

// GetTestInput reads the content of the specified test input file.
func (g *GateFormat) GetTestInput(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.Path, "tests", name))
}

// GetTestOutput reads the content of the specified test output file.
func (g *GateFormat) GetTestOutput(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(g.Path, "tests", name))
}
