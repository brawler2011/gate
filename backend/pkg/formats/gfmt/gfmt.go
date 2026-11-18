package gfmt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type GateFormat struct {
	Path    string
	Problem *Problem
}

func OpenPackage(dir string) (*GateFormat, error) {
	yamlPath := filepath.Join(dir, "problem.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.yaml: %w", err)
	}

	var prob *Problem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse problem.yaml: %w", err)
	}
	if prob == nil {
		return nil, fmt.Errorf("problem.yaml must contain a mapping")
	}

	return &GateFormat{
		Path:    dir,
		Problem: prob,
	}, nil
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return data, nil
}

func (g *GateFormat) GetSolution(name string) ([]byte, error) {
	if strings.HasPrefix(name, "solutions/") {
		return readFile(filepath.Join(g.Path, name))
	}
	return readFile(filepath.Join(g.Path, "solutions", name))
}

func (g *GateFormat) GetChecker(name string) ([]byte, error) {
	if strings.HasPrefix(name, "checkers/") {
		return readFile(filepath.Join(g.Path, name))
	}
	return readFile(filepath.Join(g.Path, "checkers", name))
}

func (g *GateFormat) GetGenerator(name string) ([]byte, error) {
	if strings.HasPrefix(name, "generators/") {
		return readFile(filepath.Join(g.Path, name))
	}
	return readFile(filepath.Join(g.Path, "generators", name))
}

func (g *GateFormat) GetInteractor(name string) ([]byte, error) {
	if strings.HasPrefix(name, "interactors/") {
		return readFile(filepath.Join(g.Path, name))
	}
	return readFile(filepath.Join(g.Path, "interactors", name))
}

func (g *GateFormat) GetLib(name string) ([]byte, error) {
	if strings.HasPrefix(name, "lib/") {
		return readFile(filepath.Join(g.Path, name))
	}
	return readFile(filepath.Join(g.Path, "lib", name))
}

func (g *GateFormat) GetTestInput(name string) ([]byte, error) {
	candidates := TestInputNames(name)

	for _, cand := range candidates {
		data, err := readFile(filepath.Join(g.Path, "tests", cand))
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("test input file not found for %s", name)
}

func (g *GateFormat) GetTestOutput(name string) ([]byte, error) {
	candidates := TestOutputNames(name)

	for _, cand := range candidates {
		data, err := readFile(filepath.Join(g.Path, "tests", cand))
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("test output file not found for %s", name)
}

func TestOutputNames(name string) []string {
	var candidates []string
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	if ext == ".out" || ext == ".ans" || ext == ".a" || ext == ".output" {
		candidates = append(candidates, name)
	}
	if stem != "" {
		candidates = append(candidates, stem+".out", stem+".ans", stem+".a", stem+".output")
	}
	candidates = append(candidates, name+".out", name+".ans", name+".a", name+".output")

	return candidates
}

func TestInputNames(name string) []string {
	candidates := []string{name}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem != "" && stem != name {
		candidates = append(candidates, stem+".in", stem+".input")
	} else {
		candidates = append(candidates, name+".in", name+".input")
	}

	return candidates
}
