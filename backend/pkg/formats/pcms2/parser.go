package pcms2

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
)

var sourceDirs = []string{".", "src", "files"}

// PCMS2 names binaries, so sources are resolved by file stem.
func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := spec.ReadFile(fsys, "problem.xml")
	if err != nil {
		return nil, err
	}
	var prob Problem
	if err := spec.UnmarshalXML(data, &prob); err != nil {
		return nil, spec.Syntax("problem.xml", err)
	}
	testset, err := mainTestset(prob.Judging.Testsets)
	if err != nil {
		return nil, err
	}
	p := &spec.Problem{
		Title:         title(prob),
		TimeLimitMs:   testset.TimeLimit,
		MemoryLimitMb: int(testset.MemoryLimit / (1 << 20)),
		InputFile:     strings.TrimSpace(prob.Judging.InputFile),
		OutputFile:    strings.TrimSpace(prob.Judging.OutputFile),
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
	if p.Testset, err = groups(fsys, testset, samples(fsys)); err != nil {
		return nil, err
	}
	p.Type, p.Grading = problemType(run), spec.TestGrading(&p.Testset)
	p.Statements = statements(fsys, p, prob.Statements)
	p.SetExamples(fsys, spec.ExampleDir(fsys, "statements"), func(t spec.Test) (string, string) { return t.Input, t.Answer })
	for _, name := range prob.Names {
		p.SetStatementTitle(spec.LanguageCode(name.Language), name.Value)
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
		return Testset{}, spec.Fail(pkg.ErrPackageLayout, "problem.xml", "", "problem.xml has no testset")
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

func problemType(run Run) string {
	if run.Interactor != nil {
		return "interactive"
	}
	return "standard"
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
	return nil, spec.Fail(pkg.ErrPackageLayout, executable, "", "source of %s not found", executable)
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

// Named groups score together; ungrouped tests stay in the root and score one by one.
func groups(fsys fs.FS, testset Testset, samples int) (spec.Group, error) {
	root := spec.Group{}
	index := map[string]int{}
	for i, t := range testset.Tests {
		number := i + 1
		text := strconv.FormatFloat(t.Points, 'f', -1, 64)
		weight, err := points.Parse(text)
		if err != nil {
			return root, spec.Fail(pkg.ErrFormat, "problem.xml", text, "test %d has invalid points %s: %v", number, text, err)
		}
		test := spec.Test{Input: t.Input, Answer: t.Outcome, Points: weight, Sample: i < samples, Ordinal: number}
		if test.Input == "" {
			test.Input = fmt.Sprintf(testset.InputPathPattern, number)
		}
		if test.Answer == "" {
			test.Answer = fmt.Sprintf(testset.AnswerPathPattern, number)
		}
		if !regular(fsys, test.Answer) {
			test.Answer = ""
		}
		if t.Group == "" {
			root.Tests = append(root.Tests, test)
			continue
		}
		n, ok := index[t.Group]
		if !ok {
			n = len(root.Groups)
			index[t.Group] = n
			root.Groups = append(root.Groups, spec.Group{Name: t.Group, Aggregate: "min"})
		}
		root.Groups[n].Tests = append(root.Groups[n].Tests, test)
	}
	// A group scores the sum of its tests only when all pass: the minimum of tests worth the whole group.
	for i := range root.Groups {
		g := &root.Groups[i]
		total := points.Value(0)
		for _, t := range g.Tests {
			total += t.Points
		}
		for j := range g.Tests {
			g.Tests[j].Points = total
		}
	}
	return root, nil
}

// Packages listing only rendered statements usually still carry statements/<language>/problem.tex;
// otherwise HTML is converted and a PDF is linked from the statement.
func statements(fsys fs.FS, p *spec.Problem, list []Statement) []spec.Statement {
	sources := map[string]string{}
	rendered := map[string]Statement{}
	for _, s := range list {
		language := spec.LanguageCode(s.Language)
		switch s.Type {
		case "application/x-tex":
			sources[language] = s.File
		case "text/html":
			rendered[language] = s
		case "application/pdf":
			if rendered[language].Type != "text/html" {
				rendered[language] = s
			}
		}
	}
	if len(sources) == 0 {
		languages, _ := fs.ReadDir(fsys, "statements")
		for _, language := range languages {
			file := path.Join("statements", language.Name(), "problem.tex")
			if regular(fsys, file) && !strings.HasPrefix(language.Name(), ".") {
				sources[spec.LanguageCode(language.Name())] = file
			}
		}
	}
	for language, s := range rendered {
		if _, ok := sources[language]; !ok && regular(fsys, s.File) {
			sources[language] = s.File
		}
	}
	var result []spec.Statement
	for _, language := range slices.Sorted(maps.Keys(sources)) {
		s := spec.Statement{Language: language, TeX: sources[language]}
		switch r := rendered[language]; {
		case r.File != s.TeX:
		case r.Type == "text/html":
			s.TeX, s.HTML = "", s.TeX
		default:
			s.TeX, s.PDF = "", "statement-"+language+".pdf"
			p.AddAsset(s.PDF, sources[language])
		}
		result = append(result, s)
		dir := path.Dir(sources[language])
		entries, _ := fs.ReadDir(fsys, dir)
		for _, e := range entries {
			if e.Type().IsRegular() {
				p.AddStatementFile(e.Name(), path.Join(dir, e.Name()))
			}
		}
	}
	return result
}
