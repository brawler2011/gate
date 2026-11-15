// Package icpc reads problem packages in the ICPC problem package format.
package icpc

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/formats/tex"
	"github.com/brawler2011/gate/backend/pkg/points"
	"gopkg.in/yaml.v3"
)

// Read reads the ICPC package in dir. Tests are the .in files of data/sample
// and data/secret; each subdirectory of data/secret is a test group.
func Read(dir string) (*spec.Problem, error) {
	data, err := os.ReadFile(filepath.Join(dir, "problem.yaml"))
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
	p := &spec.Problem{Title: prob.Name, Type: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256}
	if p.Title == "" {
		p.Title = filepath.Base(dir)
	}
	interactive := prob.Validation != nil && strings.Contains(prob.Validation.Type, "interactive")
	switch {
	case interactive:
		p.Type = "interactive"
	case prob.Type == "scoring":
		p.Type = "scoring"
	}
	if prob.Limits != nil {
		if prob.Limits.Time > 0 {
			p.TimeLimitMs = int(prob.Limits.Time * 1000)
		}
		if prob.Limits.Memory > 0 {
			p.MemoryLimitMb = prob.Limits.Memory
		}
	}
	// Programs of each kind live in their directory, one file or one
	// subdirectory per program; headers there are shared library files.
	var library []spec.File
	collect := func(folder string) ([]spec.Program, error) {
		programs, headers, err := programs(dir, folder)
		library = append(library, headers...)
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
	if p.Solutions, p.MainSolution, err = submissions(dir, collect); err != nil {
		return nil, err
	}
	p.Library = library
	if p.Groups, err = groups(dir, p.Type == "scoring"); err != nil {
		return nil, err
	}
	if p.Statements, err = statements(dir); err != nil {
		return nil, err
	}
	return p, nil
}

// programs lists the programs in folder: each source file directly in it,
// and the single source file of each subdirectory.
func programs(dir, folder string) ([]spec.Program, []spec.File, error) {
	entries, err := os.ReadDir(filepath.Join(dir, folder))
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", folder, err)
	}
	var result []spec.Program
	var headers []spec.File
	for _, e := range entries {
		rel := path.Join(folder, e.Name())
		if !e.IsDir() {
			switch {
			case spec.IsHeaderFile(e.Name()):
				headers = append(headers, spec.File{Name: e.Name(), Path: rel})
			case spec.IsSourceFile(e.Name()):
				result = append(result, spec.Program{File: spec.File{Name: e.Name(), Path: rel}})
			}
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, folder, e.Name()))
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", rel, err)
		}
		var sources []string
		for _, f := range files {
			switch {
			case f.IsDir():
			case spec.IsHeaderFile(f.Name()):
				headers = append(headers, spec.File{Name: f.Name(), Path: path.Join(rel, f.Name())})
			case spec.IsSourceFile(f.Name()):
				sources = append(sources, f.Name())
			}
		}
		if len(sources) != 1 {
			return nil, nil, fmt.Errorf("%s must hold one source file, found %d", rel, len(sources))
		}
		result = append(result, spec.Program{File: spec.File{Name: sources[0], Path: path.Join(rel, sources[0])}})
	}
	return result, headers, nil
}

// submissions lists the solutions of every verdict directory; the first
// accepted one is the main solution.
func submissions(dir string, collect func(string) ([]spec.Program, error)) ([]spec.Program, string, error) {
	verdicts, err := os.ReadDir(filepath.Join(dir, "submissions"))
	if os.IsNotExist(err) {
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
			main = list[0].Name
		}
		result = append(result, list...)
	}
	return result, main, nil
}

// groups reads data/sample and data/secret. In a scoring problem every test
// is worth its group's accept_score, 1 by default.
func groups(dir string, scoring bool) ([]spec.Group, error) {
	var result []spec.Group
	add := func(rel, name string, sample bool) error {
		weight := points.Value(0)
		if scoring {
			score, err := acceptScore(dir, rel)
			if err != nil {
				return err
			}
			weight = score
		}
		tests, err := testFiles(dir, rel, weight, sample)
		if err != nil || len(tests) == 0 {
			return err
		}
		result = append(result, spec.Group{Name: name, Policy: "each", Tests: tests})
		return nil
	}
	if err := add("data/sample", "sample", true); err != nil {
		return nil, err
	}
	if err := add("data/secret", "secret", false); err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "data", "secret"))
	for _, e := range entries {
		if e.IsDir() {
			if err := add(path.Join("data/secret", e.Name()), e.Name(), false); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func acceptScore(dir, rel string) (points.Value, error) {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel), "testdata.yaml"))
	if os.IsNotExist(err) {
		return points.Unit, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read testdata.yaml: %w", err)
	}
	var td Testdata
	if err := yaml.Unmarshal(data, &td); err != nil {
		return 0, fmt.Errorf("parse %s/testdata.yaml: %w", rel, err)
	}
	if td.AcceptScore == 0 {
		return points.Unit, nil
	}
	return points.Parse(strconv.FormatFloat(td.AcceptScore, 'f', -1, 64))
}

// testFiles lists the .in files directly in rel with their .ans answers.
func testFiles(dir, rel string, weight points.Value, sample bool) ([]spec.Test, error) {
	entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	var tests []spec.Test
	for _, e := range entries {
		stem, ok := strings.CutSuffix(e.Name(), ".in")
		if e.IsDir() || !ok {
			continue
		}
		test := spec.Test{Input: path.Join(rel, e.Name()), Points: weight, Sample: sample}
		if answer := path.Join(rel, stem+".ans"); fileExists(dir, answer) {
			test.Answer = answer
		}
		tests = append(tests, test)
	}
	return tests, nil
}

// statements converts statement/<language>/problem.tex (statements/ in some
// packages) for each language.
func statements(dir string) ([]spec.Statement, error) {
	var result []spec.Statement
	seen := map[string]bool{}
	for _, folder := range []string{"statement", "statements"} {
		languages, _ := os.ReadDir(filepath.Join(dir, folder))
		for _, language := range languages {
			file := filepath.Join(dir, folder, language.Name(), "problem.tex")
			code := tex.LanguageCode(language.Name())
			if !language.IsDir() || seen[code] || !fileExists(dir, path.Join(folder, language.Name(), "problem.tex")) {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("read statement: %w", err)
			}
			seen[code] = true
			result = append(result, spec.Statement{Language: code, Markdown: tex.ToMarkdown(string(data))})
		}
	}
	slices.SortFunc(result, func(a, b spec.Statement) int { return strings.Compare(a.Language, b.Language) })
	return result, nil
}

func fileExists(dir, rel string) bool {
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}
