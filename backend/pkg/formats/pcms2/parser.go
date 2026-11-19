package pcms2

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/formats/tex"
	"github.com/brawler2011/gate/backend/pkg/points"
)

var sourceDirs = []string{".", "src", "files"}

// PCMS2 names binaries, so sources are resolved by file stem.
func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := spec.ReadFile(fsys, "problem.xml")
	if err != nil {
		return nil, fmt.Errorf("read problem.xml: %w", err)
	}
	var prob Problem
	if err := xml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("parse PCMS2 problem.xml: %w", err)
	}
	testset, err := mainTestset(prob.Judging.Testsets)
	if err != nil {
		return nil, err
	}
	p := &spec.Problem{
		Title:         title(prob),
		TimeLimitMs:   testset.TimeLimit,
		MemoryLimitMb: int(testset.MemoryLimit / (1 << 20)),
	}
	run := prob.Judging.Run
	if p.Checker, err = findProgram(fsys, run.Verifier.Binary.Executable); err != nil {
		return nil, err
	}
	if run.Interactor != nil {
		if p.Interactor, err = findProgram(fsys, run.Interactor.Binary.Executable); err != nil {
			return nil, err
		}
	}
	p.Library = headers(fsys)
	if p.Solutions, err = solutions(fsys); err != nil {
		return nil, err
	}
	if p.Groups, err = groups(fsys, testset, samples(fsys)); err != nil {
		return nil, err
	}
	p.Type = problemType(run, p.Groups)
	p.KeepInteractiveInputsPrivate()
	p.Statements = statements(fsys, prob.Statements)
	return p, nil
}

func mainTestset(testsets []Testset) (Testset, error) {
	for _, ts := range testsets {
		if ts.Name == "tests" {
			return ts, nil
		}
	}
	if len(testsets) == 0 {
		return Testset{}, fmt.Errorf("problem.xml has no testset")
	}
	return testsets[0], nil
}

func title(prob Problem) string {
	for _, name := range prob.Names {
		if name.Language == "english" {
			return name.Value
		}
	}
	if len(prob.Names) > 0 {
		return prob.Names[0].Value
	}
	return prob.ID
}

func problemType(run Run, groups []spec.Group) string {
	if run.Interactor != nil {
		return "interactive"
	}
	for _, g := range groups {
		for _, t := range g.Tests {
			if t.Points > 0 {
				return "scoring"
			}
		}
	}
	return "pass-fail"
}

// A named binary without a corresponding source is an error.
func findProgram(fsys fs.FS, executable string) (*spec.Program, error) {
	if executable == "" {
		return nil, nil
	}
	stem := spec.Stem(executable)
	for _, folder := range sourceDirs {
		entries, err := fs.ReadDir(fsys, folder)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Type().IsRegular() && spec.Stem(e.Name()) == stem && spec.IsSourceFile(e.Name()) {
				return program(folder, e.Name(), entries), nil
			}
		}
	}
	return nil, fmt.Errorf("source of %s not found", executable)
}

// Programs include headers from their own source folder.
func headers(fsys fs.FS) []spec.File {
	var result []spec.File
	for _, folder := range append(slices.Clone(sourceDirs), "solutions") {
		entries, err := fs.ReadDir(fsys, folder)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.Type().IsRegular() && spec.IsHeaderFile(e.Name()) {
				result = append(result, spec.File{Name: e.Name(), Path: path.Join(folder, e.Name())})
			}
		}
	}
	return result
}

// PCMS2 records no solution verdicts, so no main solution can be selected.
func solutions(fsys fs.FS) ([]spec.Program, error) {
	entries, err := fs.ReadDir(fsys, "solutions")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read solutions: %w", err)
	}
	var result []spec.Program
	for _, e := range entries {
		if e.Type().IsRegular() && spec.IsSourceFile(e.Name()) {
			result = append(result, *program("solutions", e.Name(), entries))
		}
	}
	return result, nil
}

func program(folder, name string, entries []fs.DirEntry) *spec.Program {
	deps := map[string]string{}
	for _, e := range entries {
		if e.Type().IsRegular() && spec.IsHeaderFile(e.Name()) {
			deps[e.Name()] = path.Join(folder, e.Name())
		}
	}
	return &spec.Program{File: spec.File{Name: name, Path: path.Join(folder, name)}, Dependencies: deps}
}

// PCMS2 displays the first tests as examples; infer their count from files or LaTeX.
func samples(fsys fs.FS) int {
	languages, _ := fs.ReadDir(fsys, "statements")
	files := 0
	for _, language := range languages {
		n := 0
		for language.IsDir() && regular(fsys, path.Join("statements", language.Name(), fmt.Sprintf("example.%02d", n+1))) {
			n++
		}
		files = max(files, n)
	}
	if files > 0 {
		return files
	}
	named := 0
	for _, language := range languages {
		if text, err := spec.ReadFile(fsys, path.Join("statements", language.Name(), "problem.tex")); err == nil {
			named = max(named, examples(string(text)))
		}
	}
	return named
}

func examples(text string) int {
	return strings.Count(text, "\\exmp{") + strings.Count(text, "\\exmpfile{")
}

func regular(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}

// Named groups score together; ungrouped testsets score test by test.
func groups(fsys fs.FS, testset Testset, samples int) ([]spec.Group, error) {
	var result []spec.Group
	index := map[string]int{}
	for i, t := range testset.Tests {
		number := i + 1
		weight, err := points.Parse(strconv.FormatFloat(t.Points, 'f', -1, 64))
		if err != nil {
			return nil, err
		}
		test := spec.Test{Input: t.Input, Answer: t.Outcome, Points: weight, Sample: i < samples}
		if test.Input == "" {
			test.Input = fmt.Sprintf(testset.InputPathPattern, number)
		}
		if test.Answer == "" {
			test.Answer = fmt.Sprintf(testset.AnswerPathPattern, number)
		}
		if !regular(fsys, test.Answer) {
			test.Answer = ""
		}
		name, policy := t.Group, "complete"
		if name == "" {
			policy = "each"
		}
		n, ok := index[name]
		if !ok {
			n = len(result)
			index[name] = n
			result = append(result, spec.Group{Name: name, Policy: policy})
		}
		result[n].Tests = append(result[n].Tests, test)
	}
	return result, nil
}

// Packages listing only rendered statements still carry statements/<language>/problem.tex.
func statements(fsys fs.FS, list []Statement) []spec.Statement {
	sources := map[string]string{}
	for _, s := range list {
		if s.Type == "application/x-tex" {
			sources[tex.LanguageCode(s.Language)] = s.File
		}
	}
	if len(sources) == 0 {
		languages, _ := fs.ReadDir(fsys, "statements")
		for _, language := range languages {
			file := path.Join("statements", language.Name(), "problem.tex")
			if regular(fsys, file) && !strings.HasPrefix(language.Name(), ".") {
				sources[tex.LanguageCode(language.Name())] = file
			}
		}
	}
	var result []spec.Statement
	for _, language := range slices.Sorted(maps.Keys(sources)) {
		result = append(result, spec.Statement{Language: language, TeX: sources[language]})
	}
	return result
}
