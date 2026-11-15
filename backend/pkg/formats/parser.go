// Package formats detects and reads problem packages of every supported
// author format.
package formats

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/formats/icpc"
	"github.com/brawler2011/gate/backend/pkg/formats/pcms2"
	"github.com/brawler2011/gate/backend/pkg/formats/polygon"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"gopkg.in/yaml.v3"
)

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
		var config map[string]any
		if err := yaml.Unmarshal(data, &config); err != nil {
			return "", fmt.Errorf("invalid problem.yaml: %w", err)
		}
		if config == nil {
			return "", fmt.Errorf("problem.yaml must contain a mapping")
		}
		if _, ok := config["format_version"]; ok {
			return "gfmt", nil
		}
		return "icpc", nil
	}

	return "", fmt.Errorf("unknown problem format: no problem.xml or problem.yaml found in %s", packageDir)
}

// Read detects the format of the package in dir and reads it.
func Read(dir string) (*spec.Problem, error) {
	format, err := DetectFormat(dir)
	if err != nil {
		return nil, err
	}
	switch format {
	case "gfmt":
		return gfmt.Read(dir)
	case "polygon":
		return polygon.Read(dir)
	case "pcms2":
		return pcms2.Read(dir)
	default:
		return icpc.Read(dir)
	}
}
