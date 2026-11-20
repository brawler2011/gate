package gfmt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/points"
	"gopkg.in/yaml.v3"
)

// Built is a package the judge can run: every test has its input and answer as files.
type Built struct {
	Path    string
	Problem *Problem
}

// OpenBuilt reads an extracted package, rejecting links out of it.
func OpenBuilt(dir string) (*Built, error) {
	data, err := os.ReadFile(filepath.Join(dir, "problem.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read problem.yaml: %w", err)
	}
	var p *Problem
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse problem.yaml: %w", err)
	}
	if p == nil {
		return nil, fmt.Errorf("problem.yaml must contain a mapping")
	}
	regular := func(path string) error {
		if !fs.ValidPath(path) || path == "." || strings.ContainsAny(path, "\\\x00") {
			return fmt.Errorf("invalid package path %q", path)
		}
		prefix := dir
		for _, part := range strings.Split(path, "/") {
			prefix = filepath.Join(prefix, part)
			info, err := os.Lstat(prefix)
			if err != nil {
				return fmt.Errorf("lstat: %w", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("package symlink %q", path)
			}
		}
		if info, err := os.Stat(prefix); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("package path is not a regular file: %q", path)
		}
		return nil
	}
	library, err := os.ReadDir(filepath.Join(dir, LibraryFolder))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("readdir: %w", err)
	}
	for _, f := range library {
		if err := regular(LibraryFolder + "/" + f.Name()); err != nil {
			return nil, err
		}
	}
	if err := p.CheckBuilt(regular); err != nil {
		return nil, err
	}
	return &Built{Path: dir, Problem: p}, nil
}

// Program finds the source of a checker or interactor and its language.
func (p *Problem) Program(role string) (path, language string, ok bool) {
	for path, source := range p.Sources {
		if strings.HasPrefix(path, RoleFolder(role)+"/") {
			return path, source.Language, true
		}
	}
	return "", "", false
}

// CheckBuilt checks what the judge relies on; regular checks that a package path is a file.
func (p *Problem) CheckBuilt(regular func(string) error) error {
	if p.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported package format version %q", p.FormatVersion)
	}
	if p.Type != "pass-fail" && p.Type != "scoring" && p.Type != "interactive" {
		return fmt.Errorf("invalid package type")
	}
	if p.Limits.TimeMs <= 0 || p.Limits.MemoryMb <= 0 || len(p.Subtasks) == 0 {
		return fmt.Errorf("invalid package limits or subtasks")
	}
	roles := map[string]int{}
	for path, source := range p.Sources {
		role := folderRole(strings.SplitN(path, "/", 2)[0])
		if role != "checker" && role != "interactor" {
			continue
		}
		roles[role]++
		if source.Language == "" {
			return fmt.Errorf("%s has no language", path)
		}
		if err := regular(path); err != nil {
			return err
		}
	}
	for role, required := range map[string]bool{"checker": p.Type != "interactive", "interactor": p.Type == "interactive"} {
		if roles[role] > 1 || required && roles[role] == 0 {
			return fmt.Errorf("package needs exactly one %s", role)
		}
	}
	ordinals := map[int]bool{}
	total := points.Value(0)
	for _, s := range p.Subtasks {
		if s.Policy != "complete" && s.Policy != "each" || s.Points < 0 {
			return fmt.Errorf("invalid package subtask")
		}
		total += s.Points
		for _, t := range s.Tests {
			if t.Ordinal <= 0 || ordinals[t.Ordinal] {
				return fmt.Errorf("invalid test ordinal %d", t.Ordinal)
			}
			ordinals[t.Ordinal] = true
			for _, name := range []string{t.Input, t.Answer} {
				if name == "" {
					return fmt.Errorf("test %d has no input or answer file", t.Ordinal)
				}
				if err := regular(TestFolder + "/" + name); err != nil {
					return err
				}
			}
		}
	}
	if len(ordinals) == 0 {
		return fmt.Errorf("package has no tests")
	}
	if p.Type == "scoring" && (p.MaxScore == nil || *p.MaxScore != total) {
		return fmt.Errorf("invalid package max score")
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("package dependency cycle")
		case 2:
			return nil
		}
		s, ok := p.Subtasks[name]
		if !ok {
			return fmt.Errorf("unknown package subtask dependency")
		}
		state[name] = 1
		for _, dep := range s.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		return nil
	}
	for name := range p.Subtasks {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}
