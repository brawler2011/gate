package gfmt

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/points"
)

// OpenBuiltPackage is the shared Builder/Judge contract for published GFMT v2.
// Import parsers may read author formats; published packages contain no recipe
// requiring generators or component compilation during judging.
func OpenBuiltPackage(dir string) (*GateFormat, error) {
	g, err := OpenPackage(dir)
	if err != nil {
		return nil, err
	}
	p := g.Problem
	if p.FormatVersion != "2" || !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(p.ToolchainDigest) || p.Architecture == "" || p.ABI == "" {
		return nil, fmt.Errorf("invalid built package environment")
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
	for id, c := range p.Compiled {
		if id == "" || c.Role == "" {
			return nil, fmt.Errorf("invalid compiled component")
		}
		lang, ok := p.Languages[c.Language]
		if !ok || lang.Execute == "" {
			return nil, fmt.Errorf("missing component language configuration")
		}
		if err := regular(c.Entry); err != nil {
			return nil, err
		}
		for dest, path := range c.Dependencies {
			if !fs.ValidPath(dest) || dest == "." || strings.ContainsAny(dest, "\\\x00") {
				return nil, fmt.Errorf("invalid component dependency destination")
			}
			if err := regular(path); err != nil {
				return nil, err
			}
		}
	}
	selected := func(id, role string, required bool) error {
		if id == "" && !required {
			return nil
		}
		c, ok := p.Compiled[id]
		if !ok || c.Role != role {
			return fmt.Errorf("invalid compiled %s selection", role)
		}
		return nil
	}
	for _, s := range []struct {
		id, role string
		required bool
	}{{p.Checker, "checker", p.Type != "interactive"}, {p.Interactor, "interactor", p.Type == "interactive"}, {p.Validator, "validator", false}} {
		if err := selected(s.id, s.role, s.required); err != nil {
			return nil, err
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
			if p.Type == "interactive" {
				if test.AnswerMode != "interactive" || test.Answer != "" {
					return nil, fmt.Errorf("invalid interactive test")
				}
			} else {
				if test.AnswerMode != "generated" && test.AnswerMode != "provided" {
					return nil, fmt.Errorf("invalid answer mode")
				}
				if err := regular(test.Answer); err != nil {
					return nil, err
				}
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
