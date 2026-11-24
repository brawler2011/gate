// Package icpc reads legacy ICPC packages.
package icpc

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"gopkg.in/yaml.v3"
)

// Each directory under data/ becomes a test group.
func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := spec.ReadFile(fsys, "problem.yaml")
	if err != nil {
		return nil, err
	}
	var prob *Problem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, spec.Syntax("problem.yaml", err)
	}
	if prob == nil {
		return nil, spec.Fail(pkg.ErrPackageSyntax, "problem.yaml", "", "problem.yaml must contain a mapping")
	}
	p := &spec.Problem{Title: prob.Name, Type: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256}
	validation := strings.Fields(prob.Validation)
	interactive := slices.Contains(validation, "interactive")
	switch {
	case slices.Contains(validation, "score"):
		return nil, spec.Fail(pkg.ErrUnsupported, "problem.yaml", "score", "output validators that score are not supported")
	case interactive && prob.Type == "scoring":
		return nil, spec.Fail(pkg.ErrUnsupported, "problem.yaml", "scoring", "interactive scoring problems are not supported")
	case interactive:
		p.Type = "interactive"
	case prob.Type == "scoring":
		p.Type = "scoring"
	}
	custom := len(validation) > 0 && validation[0] == "custom"
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
		return nil, spec.Fail(pkg.ErrPackageLayout, "output_validators", "", "package has %d output validators; Gate supports one", len(output))
	}
	if !custom && !interactive {
		output = []spec.Program{{File: spec.File{Name: "default_validator.cpp"}, Dependencies: map[string]string{}, Text: defaultValidator}}
	}
	if p.Validators, err = collect("input_validators"); err != nil {
		return nil, err
	}
	for _, programs := range [][]spec.Program{output, p.Validators} {
		for i := range programs {
			if programs[i].Protocol, err = protocol(fsys, programs[i]); err != nil {
				return nil, err
			}
		}
	}
	if len(output) == 1 {
		output[0].Args = strings.Fields(prob.ValidatorFlags)
		// The output validator of an interactive problem is its interactor.
		if interactive {
			p.Interactor = &output[0]
		} else {
			p.Checker = &output[0]
		}
	}
	if p.Generators, err = collect("generators"); err != nil {
		return nil, err
	}
	if p.Solutions, p.MainSolution, err = submissions(fsys, collect); err != nil {
		return nil, err
	}
	tree := testdata{fsys: fsys, scoring: p.Type == "scoring", legacy: prob.Grading}
	if p.Testset, err = tree.read(); err != nil {
		return nil, err
	}
	p.KeepInteractiveInputsPrivate()
	p.Statements = statements(fsys, p)
	return p, nil
}

// The ICPC default output validator, called with the validator_flags of the problem.
//
//go:embed validator/default.cpp
var defaultValidator string

var testlibInclude = regexp.MustCompile(`(?m)^\s*#\s*include\s*["<]testlib\.h[">]`)

// Programs including testlib.h are taken to speak its protocol, as Gate's own fixtures do.
func protocol(fsys fs.FS, p spec.Program) (string, error) {
	text := p.Text
	if text == "" {
		data, err := spec.ReadFile(fsys, p.Path)
		if err != nil {
			return "", err
		}
		text = string(data)
	}
	if testlibInclude.MatchString(text) {
		return "testlib", nil
	}
	return "icpc", nil
}

// A program is one source file with its adjacent headers, optionally in its own directory.
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
			return nil, nil, spec.Fail(pkg.ErrPackageLayout, dir, "", "%s must hold one source file, found %d", dir, len(sources))
		}
		result = append(result, program(sources[0], headers))
		library = append(library, headers...)
	}
	return result, library, nil
}

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

// Solutions in other folders are kept out of the build.
var folderVerdicts = map[string][]string{
	"accepted":            {"OK"},
	"wrong_answer":        {"WA"},
	"time_limit_exceeded": {"TLE"},
	"run_time_error":      {"RE"},
}

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
		for i := range list {
			list[i].Verdicts = folderVerdicts[verdict.Name()]
		}
		if verdict.Name() == "accepted" && len(list) > 0 {
			main = list[0].Path
		}
		result = append(result, list...)
	}
	return result, main, nil
}

// Legacy packages keep LaTeX in problem_statement/, 2023-07 ones Markdown in statement/; other files there are images.
func statements(fsys fs.FS, p *spec.Problem) []spec.Statement {
	var result []spec.Statement
	seen := map[string]bool{}
	for _, dir := range []string{"statement", "problem_statement"} {
		entries, _ := fs.ReadDir(fsys, dir)
		for _, e := range entries {
			name := e.Name()
			if !e.Type().IsRegular() {
				continue
			}
			ext := path.Ext(name)
			base := strings.TrimSuffix(name, ext)
			language, named := strings.CutPrefix(base, "problem.")
			if base == "problem" {
				language, named = "en", true
			}
			language = spec.LanguageCode(language)
			switch {
			case named && (ext == ".tex" || ext == ".md") && !seen[language]:
				seen[language] = true
				s := spec.Statement{Language: language}
				if ext == ".tex" {
					s.TeX = path.Join(dir, name)
				} else {
					s.Markdown = path.Join(dir, name)
				}
				result = append(result, s)
			case spec.IsImage(name):
				p.AddAsset(name, path.Join(dir, name))
			}
		}
	}
	return result
}

func regular(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
