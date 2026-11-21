package formats

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/formats/icpc"
	"github.com/brawler2011/gate/backend/pkg/formats/pcms2"
	"github.com/brawler2011/gate/backend/pkg/formats/polygon"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"gopkg.in/yaml.v3"
)

func DetectFormat(dir string) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", fmt.Errorf("open package: %w", err)
	}
	defer root.Close()
	return detect(root.FS())
}

func detect(fsys fs.FS) (string, error) {
	if _, err := fs.Stat(fsys, "problem.xml"); err == nil {
		data, err := spec.ReadFile(fsys, "problem.xml")
		if err != nil {
			return "", err
		}
		content := string(data)
		if strings.Contains(content, "pcms2.ru") || strings.Contains(content, "-//PCMS2//") || strings.Contains(content, "<problem id=") {
			return "pcms2", nil
		}
		return "polygon", nil
	}
	if _, err := fs.Stat(fsys, "problem.yaml"); err != nil {
		return "", spec.Fail(pkg.ErrPackageLayout, "", "", "unknown problem format: no problem.xml or problem.yaml found")
	}
	data, err := spec.ReadFile(fsys, "problem.yaml")
	if err != nil {
		return "", err
	}
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", spec.Syntax("problem.yaml", err)
	}
	if config == nil {
		return "", spec.Fail(pkg.ErrPackageSyntax, "problem.yaml", "", "problem.yaml must contain a mapping")
	}
	if _, ok := config["format_version"]; ok {
		return "gfmt", nil
	}
	return "icpc", nil
}

// Readers are confined to the package root.
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
