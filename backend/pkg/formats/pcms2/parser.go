// Package pcms2 reads PCMS2 problem packages.
package pcms2

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/formats/tex"
	"github.com/brawler2011/gate/backend/pkg/points"
)

// sourceDirs are where PCMS2 packages keep program sources and headers.
var sourceDirs = []string{".", "src", "files"}

// Read reads the PCMS2 package in dir as described by its problem.xml. PCMS2
// names compiled programs only, so their sources are found by file stem.
func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := fs.ReadFile(fsys, "problem.xml")
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
	if p.Statements, err = statements(fsys, prob.Statements); err != nil {
		return nil, err
	}
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

// findProgram finds the source of a compiled program, such as check.cpp for
// check.exe. A program named without a source in the package is an error.
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
				return &spec.Program{File: spec.File{Name: e.Name(), Path: path.Join(folder, e.Name())}}, nil
			}
		}
	}
	return nil, fmt.Errorf("source of %s not found", executable)
}

// headers are the C++ headers next to the program sources, such as testlib.h.
func headers(fsys fs.FS) []spec.File {
	var result []spec.File
	for _, folder := range sourceDirs {
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

// solutions are the sources in solutions/. PCMS2 records no verdicts, so none
// is the main solution.
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
			result = append(result, spec.Program{File: spec.File{Name: e.Name(), Path: "solutions/" + e.Name()}})
		}
	}
	return result, nil
}

// samples counts the examples of the statement, statements/<language>/example.NN;
// PCMS2 shows the first tests as examples.
func samples(fsys fs.FS) int {
	languages, _ := fs.ReadDir(fsys, "statements")
	count := 0
	for _, language := range languages {
		n := 0
		for language.IsDir() {
			if _, err := fs.Stat(fsys, path.Join("statements", language.Name(), fmt.Sprintf("example.%02d", n+1))); err != nil {
				break
			}
			n++
		}
		count = max(count, n)
	}
	return count
}

// groups keeps the test order. Tests of a named group score together; a
// testset without groups is one group scored test by test.
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
		if info, err := fs.Stat(fsys, test.Answer); err != nil || !info.Mode().IsRegular() {
			test.Answer = ""
		}
		name, policy := t.Group, "complete"
		if name == "" {
			name, policy = "tests", "each"
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

// statements converts the LaTeX statements; packages that list only rendered
// statements still carry statements/<language>/problem.tex.
func statements(fsys fs.FS, list []Statement) ([]spec.Statement, error) {
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
			if _, err := fs.Stat(fsys, file); err == nil && !strings.HasPrefix(language.Name(), ".") {
				sources[tex.LanguageCode(language.Name())] = file
			}
		}
	}
	languages := make([]string, 0, len(sources))
	for language := range sources {
		languages = append(languages, language)
	}
	slices.Sort(languages)
	var result []spec.Statement
	for _, language := range languages {
		data, err := fs.ReadFile(fsys, sources[language])
		if err != nil {
			return nil, fmt.Errorf("read statement: %w", err)
		}
		result = append(result, spec.Statement{Language: language, Markdown: tex.ToMarkdown(string(data))})
	}
	return result, nil
}
