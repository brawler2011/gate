// Package icpc reads problem packages in the legacy ICPC problem package
// format (https://icpc.io/problem-package-format/spec/legacy.html).
//
// Gate runs checkers, validators and interactors with the testlib protocol,
// so a package imports as written only when its programs use testlib.
package icpc

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/formats/tex"
	"gopkg.in/yaml.v3"
)

// Read reads the ICPC package in fsys. Tests are the .in files of data/sample
// and data/secret; each subdirectory of data/secret is a test group.
func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := fs.ReadFile(fsys, "problem.yaml")
	if err != nil {
		return nil, fmt.Errorf("read problem.yaml: %w", err)
	}
	var prob *Problem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("parse ICPC problem.yaml: %w", err)
	}
	if prob == nil {
		return nil, fmt.Errorf("problem.yaml must contain a mapping")
	}
	// The ICPC grader aggregates group scores (sum, min, max, avg, stopping at
	// the first rejection by default), which Gate's per-test points cannot
	// express; importing such a package would silently change its scores.
	if prob.Type == "scoring" {
		return nil, fmt.Errorf("ICPC scoring problems are not supported")
	}
	p := &spec.Problem{Title: prob.Name, Type: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256}
	interactive := slices.Contains(strings.Fields(prob.Validation), "interactive")
	if interactive {
		p.Type = "interactive"
	}
	if prob.Limits != nil {
		if prob.Limits.Time > 0 {
			p.TimeLimitMs = int(prob.Limits.Time * 1000)
		}
		if prob.Limits.Memory > 0 {
			p.MemoryLimitMb = prob.Limits.Memory
		}
	}
	collect := func(folder string) ([]spec.Program, error) {
		programs, headers, err := programs(fsys, folder)
		p.Library = append(p.Library, headers...)
		return programs, err
	}
	output, err := collect("output_validators")
	if err != nil {
		return nil, err
	}
	if len(output) > 1 {
		return nil, fmt.Errorf("package has %d output validators; Gate supports one", len(output))
	}
	if len(output) == 1 {
		// The output validator of an interactive problem is its interactor.
		if interactive {
			p.Interactor = &output[0]
		} else {
			p.Checker = &output[0]
		}
	}
	input, err := collect("input_validators")
	if err != nil {
		return nil, err
	}
	if len(input) > 1 {
		return nil, fmt.Errorf("package has %d input validators; Gate supports one", len(input))
	}
	if len(input) == 1 {
		p.Validator = &input[0]
	}
	if p.Generators, err = collect("generators"); err != nil {
		return nil, err
	}
	if p.Solutions, p.MainSolution, err = submissions(fsys, collect); err != nil {
		return nil, err
	}
	if p.Groups, err = groups(fsys); err != nil {
		return nil, err
	}
	p.KeepInteractiveInputsPrivate()
	p.Statements = statements(fsys)
	return p, nil
}

// programs lists the programs in folder: each source file directly in it, and
// each subdirectory, whose single source file is the program. A program
// includes the headers next to it.
func programs(fsys fs.FS, folder string) ([]spec.Program, []spec.File, error) {
	entries, err := fs.ReadDir(fsys, folder)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", folder, err)
	}
	sources, headers := split(folder, entries)
	var result []spec.Program
	for _, source := range sources {
		result = append(result, program(source, headers))
	}
	library := headers
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := path.Join(folder, e.Name())
		files, err := fs.ReadDir(fsys, dir)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", dir, err)
		}
		sources, headers := split(dir, files)
		if len(sources) != 1 {
			return nil, nil, fmt.Errorf("%s must hold one source file, found %d", dir, len(sources))
		}
		result = append(result, program(sources[0], headers))
		library = append(library, headers...)
	}
	return result, library, nil
}

// split separates the source files and headers among entries of dir.
func split(dir string, entries []fs.DirEntry) ([]string, []spec.File) {
	var sources []string
	var headers []spec.File
	for _, e := range entries {
		switch {
		case !e.Type().IsRegular():
		case spec.IsHeaderFile(e.Name()):
			headers = append(headers, spec.File{Name: e.Name(), Path: path.Join(dir, e.Name())})
		case spec.IsSourceFile(e.Name()):
			sources = append(sources, path.Join(dir, e.Name()))
		}
	}
	return sources, headers
}

func program(source string, headers []spec.File) spec.Program {
	deps := map[string]string{}
	for _, h := range headers {
		deps[h.Name] = h.Path
	}
	return spec.Program{File: spec.File{Name: path.Base(source), Path: source}, Dependencies: deps}
}

// submissions lists the solutions of every verdict directory; the first
// accepted one is the main solution.
func submissions(fsys fs.FS, collect func(string) ([]spec.Program, error)) ([]spec.Program, string, error) {
	verdicts, err := fs.ReadDir(fsys, "submissions")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read submissions: %w", err)
	}
	var result []spec.Program
	main := ""
	for _, verdict := range verdicts {
		if !verdict.IsDir() {
			continue
		}
		list, err := collect(path.Join("submissions", verdict.Name()))
		if err != nil {
			return nil, "", err
		}
		if verdict.Name() == "accepted" && len(list) > 0 {
			main = list[0].Path
		}
		result = append(result, list...)
	}
	return result, main, nil
}

// groups reads data/sample and data/secret. Every directory holding tests,
// at any depth, is a group named by its path under data/.
func groups(fsys fs.FS) ([]spec.Group, error) {
	var result []spec.Group
	for _, top := range []string{"sample", "secret"} {
		err := fs.WalkDir(fsys, path.Join("data", top), func(dir string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && dir == path.Join("data", top) {
				return fs.SkipDir
			}
			if err != nil || !d.IsDir() {
				return err
			}
			tests, err := testFiles(fsys, dir, top == "sample")
			if err != nil || len(tests) == 0 {
				return err
			}
			result = append(result, spec.Group{Name: strings.TrimPrefix(dir, "data/"), Policy: "each", Tests: tests})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("read tests: %w", err)
		}
	}
	return result, nil
}

// testFiles lists the .in files directly in dir with their .ans answers.
func testFiles(fsys fs.FS, dir string, sample bool) ([]spec.Test, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var tests []spec.Test
	for _, e := range entries {
		stem, ok := strings.CutSuffix(e.Name(), ".in")
		if !e.Type().IsRegular() || !ok {
			continue
		}
		test := spec.Test{Input: path.Join(dir, e.Name()), Sample: sample}
		if answer := path.Join(dir, stem+".ans"); regular(fsys, answer) {
			test.Answer = answer
		}
		tests = append(tests, test)
	}
	return tests, nil
}

// statements reads problem.<language>.tex (problem.tex is English) from
// problem_statement/, or from statement/ as in the 2023 format, which may
// also hold problem.<language>.md.
func statements(fsys fs.FS) []spec.Statement {
	for _, dir := range []string{"problem_statement", "statement"} {
		entries, err := fs.ReadDir(fsys, dir)
		if err != nil {
			continue
		}
		var result []spec.Statement
		seen := map[string]bool{}
		for _, e := range entries {
			name := e.Name()
			ext := path.Ext(name)
			base := strings.TrimSuffix(name, ext)
			language, ok := strings.CutPrefix(base, "problem.")
			if base == "problem" {
				language, ok = "en", true
			}
			language = tex.LanguageCode(language)
			if !ok || (ext != ".tex" && ext != ".md") || !e.Type().IsRegular() || seen[language] {
				continue
			}
			seen[language] = true
			result = append(result, spec.Statement{Language: language, Path: path.Join(dir, name), TeX: ext == ".tex"})
		}
		if len(result) > 0 {
			return result
		}
	}
	return nil
}

func regular(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
