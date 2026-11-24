package gfmt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
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
	for _, folder := range []string{LibraryFolder, GraderFolder} {
		files, err := os.ReadDir(filepath.Join(dir, folder))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("readdir: %w", err)
		}
		for _, f := range files {
			if err := regular(folder + "/" + f.Name()); err != nil {
				return nil, err
			}
		}
	}
	if err := p.CheckBuilt(regular); err != nil {
		return nil, err
	}
	return &Built{Path: dir, Problem: p}, nil
}

// Program finds the source of a checker or interactor.
func (p *Problem) Program(role string) (string, Source, bool) {
	for path, source := range p.Sources {
		if strings.HasPrefix(path, RoleFolder(role)+"/") {
			return path, source, true
		}
	}
	return "", Source{}, false
}

// CheckBuilt checks what the judge relies on; regular checks that a package path is a file.
func (p *Problem) CheckBuilt(regular func(string) error) error {
	if p.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported package format version %q", p.FormatVersion)
	}
	if !slices.Contains(spec.ProblemTypes, p.Type) {
		return fmt.Errorf("invalid package type")
	}
	if p.Limits.TimeMs <= 0 || p.Limits.MemoryMb <= 0 || p.Testset.Name != "" || len(p.Testset.Requires) > 0 {
		return fmt.Errorf("invalid package limits or testset")
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
	if (p.Type == "communication") != (p.Processes > 0) || p.Processes < 0 || p.Processes > spec.MaxProcesses {
		return fmt.Errorf("invalid package processes")
	}
	for role, required := range map[string]bool{"checker": !spec.Interactive(p.Type), "interactor": spec.Interactive(p.Type)} {
		if roles[role] > 1 || required && roles[role] == 0 {
			return fmt.Errorf("package needs exactly one %s", role)
		}
	}
	ordinals := map[int]bool{}
	groups := map[string]*Group{}
	for g := range p.Testset.All() {
		if g != &p.Testset && (g.Name == "" || groups[g.Name] != nil) {
			return fmt.Errorf("package group names must be unique and non-empty")
		}
		groups[g.Name] = g
		if g.Aggregate != "" && !slices.Contains(spec.Aggregations, g.Aggregate) || g.Feedback != "" && !slices.Contains(spec.FeedbackPolicies, g.Feedback) {
			return fmt.Errorf("invalid package group %q", g.Name)
		}
		for _, t := range g.Tests {
			if t.Ordinal <= 0 || ordinals[t.Ordinal] || t.Points < 0 {
				return fmt.Errorf("invalid test ordinal %d or points", t.Ordinal)
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
	state := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("package group requirements form a cycle")
		case 2:
			return nil
		}
		g, ok := groups[name]
		if !ok || name == "" {
			return fmt.Errorf("package group requires unknown group %q", name)
		}
		state[name] = 1
		for _, required := range g.Requires {
			if err := visit(required); err != nil {
				return err
			}
		}
		state[name] = 2
		return nil
	}
	for name := range groups {
		if name != "" {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	if max, _ := p.Grade(func(int) bool { return true }); spec.Scored(p.Type) && (p.MaxScore == nil || *p.MaxScore != max) {
		return fmt.Errorf("invalid package max score")
	}
	return nil
}
