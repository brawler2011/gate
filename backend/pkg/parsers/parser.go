package parsers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gate149/gate/backend/pkg/problemformat"
)

// ProblemParser is the interface that all format parsers must implement
type ProblemParser interface {
	Parse(packageDir string) (*problemformat.ProblemManifest, *problemformat.TestsMetadata, error)
	GetFormat() string
}

// DetectFormat attempts to detect the problem package format
func DetectFormat(packageDir string) (string, error) {
	// Check for Polygon format (problem.xml)
	if _, err := os.Stat(filepath.Join(packageDir, "problem.xml")); err == nil {
		return "polygon", nil
	}

	// Check for ICPC format (problem.yaml)
	if _, err := os.Stat(filepath.Join(packageDir, "problem.yaml")); err == nil {
		return "icpc", nil
	}

	return "", fmt.Errorf("unknown problem format: no problem.xml or problem.yaml found")
}

// GetParser returns the appropriate parser for the given format
func GetParser(format string) (ProblemParser, error) {
	switch format {
	case "polygon":
		return NewPolygonParser(), nil
	case "icpc":
		return NewICPCParser(), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// ParsePackage detects the format and parses the problem package
func ParsePackage(packageDir string) (*problemformat.ProblemManifest, *problemformat.TestsMetadata, string, error) {
	format, err := DetectFormat(packageDir)
	if err != nil {
		return nil, nil, "", err
	}

	parser, err := GetParser(format)
	if err != nil {
		return nil, nil, "", err
	}

	manifest, tests, err := parser.Parse(packageDir)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to parse %s package: %w", format, err)
	}

	return manifest, tests, format, nil
}
