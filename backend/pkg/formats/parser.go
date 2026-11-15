// Package formats detects and reads problem packages of every supported
// author format.
package formats

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/formats/icpc"
	"github.com/brawler2011/gate/backend/pkg/formats/pcms2"
	"github.com/brawler2011/gate/backend/pkg/formats/polygon"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"gopkg.in/yaml.v3"
)

// DetectFormat detects the format of the package in dir.
func DetectFormat(dir string) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("open package: %w", err)
	}
	defer root.Close()
	return detect(root.FS())
}

func detect(fsys fs.FS) (string, error) {
	if data, err := fs.ReadFile(fsys, "problem.xml"); err == nil {
		content := string(data)
		if strings.Contains(content, "pcms2.ru") || strings.Contains(content, "-//PCMS2//") || strings.Contains(content, "<problem id=") {
			return "pcms2", nil
		}
		return "polygon", nil
	}
	data, err := fs.ReadFile(fsys, "problem.yaml")
	if err != nil {
		return "", fmt.Errorf("unknown problem format: no problem.xml or problem.yaml found")
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

// Read detects the format of the package in dir and reads it. Readers see
// the package through an os.Root, so no path a package names can leave it.
func Read(dir string) (*spec.Problem, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open package: %w", err)
	}
	defer root.Close()
	fsys := root.FS()
	format, err := detect(fsys)
	if err != nil {
		return nil, err
	}
	switch format {
	case "gfmt":
		return gfmt.Read(fsys)
	case "polygon":
		return polygon.Read(fsys)
	case "pcms2":
		return pcms2.Read(fsys)
	default:
		return icpc.Read(fsys)
	}
}
