package formats

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/formats/icpc"
	"github.com/gate149/gate/backend/pkg/formats/pcms2"
	"github.com/gate149/gate/backend/pkg/formats/polygon"
)

// Parser is the interface that all format parsers must implement.
// It parses a problem package directory into a unified GFMT Problem representation and file mappings.
type Parser interface {
	Parse(packageDir string) (*gfmt.ImportPlan, error)
	GetFormat() string
}

// DetectFormat detects the format of a package directory.
func DetectFormat(packageDir string) (string, error) {
	// Check for XML-based formats (PCMS2 or Polygon)
	xmlPath := filepath.Join(packageDir, "problem.xml")
	if _, err := os.Stat(xmlPath); err == nil {
		data, err := os.ReadFile(xmlPath)
		if err != nil {
			return "", fmt.Errorf("failed to read problem.xml: %w", err)
		}
		content := string(data)
		if strings.Contains(content, "pcms2.ru") || strings.Contains(content, "-//PCMS2//") || strings.Contains(content, "<problem id=") {
			return "pcms2", nil
		}
		return "polygon", nil
	}

	// Check for YAML-based formats (GFMT or ICPC)
	yamlPath := filepath.Join(packageDir, "problem.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			return "", fmt.Errorf("failed to read problem.yaml: %w", err)
		}
		content := string(data)
		if strings.Contains(content, "format_version:") {
			return "gfmt", nil
		}
		return "icpc", nil
	}

	return "", fmt.Errorf("unknown problem format: no problem.xml or problem.yaml found in %s", packageDir)
}

// GetParser returns the appropriate parser for the given format name.
func GetParser(formatName string) (Parser, error) {
	switch formatName {
	case "gfmt":
		return gfmt.NewParser(), nil
	case "polygon":
		return polygon.NewParser(), nil
	case "pcms2":
		return pcms2.NewParser(), nil
	case "icpc":
		return icpc.NewParser(), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", formatName)
	}
}
