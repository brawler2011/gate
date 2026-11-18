package gfmt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/points"
)

// OpenBuiltPackage is the shared Builder/Judge contract for published GFMT v2.
// Import parsers may read author formats; published packages contain no recipe
// requiring generators, and only their checker or interactor is compiled, by the
// judge with its own image.
func OpenBuiltPackage(dir string) (*GateFormat, error) {
	g, err := OpenPackage(dir)
	if err != nil {
		return nil, err
	}
	p := g.Problem
	if p.FormatVersion != "2" {
		return nil, fmt.Errorf("invalid built package format version")
	}
	if p.Type != "pass-fail" && p.Type != "scoring" && p.Type != "interactive" {
		return nil, fmt.Errorf("invalid package type")
	}
	if p.Limits.TimeMs <= 0 || p.Limits.MemoryMb <= 0 || len(p.Subtasks) == 0 {
		return nil, fmt.Errorf("invalid package limits or subtasks")
	}
	regular := func(path string) error {
		if !fs.ValidPath(path) || path == "." || strings.ContainsAny(path, "\\\x00") {
			return fmt.Errorf("invalid package path %q", path)
		}
		parts := strings.Split(path, "/")
		prefix := dir
		for _, part := range parts {
			prefix = filepath.Join(prefix, part)
			info, err := os.Lstat(prefix)
			if err != nil {
				return fmt.Errorf("lstat: %w", err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("package symlink %q", path)
			}
		}
		info, err := os.Stat(prefix)
		if err != nil {
			return fmt.Errorf("stat: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package path is not regular: %q", path)
		}
		return nil
	}
	for role, c := range p.Programs {
		if role != "checker" && role != "interactor" {
			return nil, fmt.Errorf("invalid program role %q", role)
		}
		if c.Language == "" {
			return nil, fmt.Errorf("program %s has no language", role)
		}
		if err := regular(c.Source); err != nil {
			return nil, err
		}
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
	for role, required := range map[string]bool{"checker": p.Type != "interactive", "interactor": p.Type == "interactive"} {
		if _, ok := p.Programs[role]; required && !ok {
			return nil, fmt.Errorf("built package needs a %s", role)
		}
	}
	ordinals := map[int]bool{}
	groups := map[int]bool{}
	totalPoints := points.Value(0)
	for name, s := range p.Subtasks {
		if name == "" || s.Ordinal <= 0 || groups[s.Ordinal] || s.Points < 0 || (s.Policy != "complete" && s.Policy != "each") {
			return nil, fmt.Errorf("invalid package subtask")
		}
		groups[s.Ordinal] = true
		totalPoints += s.Points
		for _, test := range s.Tests {
			if test.Ordinal <= 0 || ordinals[test.Ordinal] || test.Generate != "" {
				return nil, fmt.Errorf("invalid materialized test order or recipe")
			}
			ordinals[test.Ordinal] = true
			if err := regular(test.Manual); err != nil {
				return nil, err
			}
			interactive := test.AnswerMode == "interactive"
			if interactive != (p.Type == "interactive") || !interactive && test.AnswerMode != "generated" && test.AnswerMode != "provided" {
				return nil, fmt.Errorf("invalid answer mode")
			}
			if err := regular(test.Answer); err != nil {
				return nil, err
			}
		}
	}
	if len(ordinals) == 0 {
		return nil, fmt.Errorf("package has no tests")
	}
	if p.Type == "scoring" && (p.MaxScore == nil || *p.MaxScore != totalPoints) {
		return nil, fmt.Errorf("invalid package max score")
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("package dependency cycle")
		}
		if state[name] == 2 {
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
			return nil, err
		}
	}
	return g, nil
}
