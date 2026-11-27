// Package icpc reads ICPC packages: legacy ones and those of version 2025-09, which completed the 2023-07 draft.
package icpc

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
	"gopkg.in/yaml.v3"
)

// defaultMemoryMb is the typical system default the spec names for packages without a memory limit.
const defaultMemoryMb = 2048

func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := spec.ReadFile(fsys, "problem.yaml")
	if err != nil {
		return nil, err
	}
	if info, err := fs.Stat(fsys, "data"); err != nil || !info.IsDir() {
		return nil, spec.Fail(pkg.ErrUnsupported, "data", "", "Found problem.yaml, but this is not an ICPC package: there is no data/ directory with tests")
	}
	var version struct {
		Version string `yaml:"problem_format_version"`
	}
	// A problem.yaml that is not a mapping is reported by the legacy reader.
	_ = yaml.Unmarshal(data, &version)
	switch version.Version {
	case "", "legacy", "legacy-icpc":
		return readLegacy(fsys, data)
	case "2023-07-draft", "2025-09":
		return readCurrent(fsys, data)
	}
	return nil, spec.Fail(pkg.ErrUnsupported, "problem.yaml", version.Version, "problem format version %s is not supported", version.Version)
}

// Each directory under data/ becomes a test group.
func readLegacy(fsys fs.FS, data []byte) (*spec.Problem, error) {
	var prob *Problem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, spec.Syntax("problem.yaml", err)
	}
	if prob == nil {
		return nil, spec.Fail(pkg.ErrPackageSyntax, "problem.yaml", "", "problem.yaml must contain a mapping")
	}
	p := &spec.Problem{Type: "standard", Grading: "pass-fail", MemoryLimitMb: defaultMemoryMb, TimeRule: &spec.TimeRule{AcceptedFactor: 5, TLEFactor: 2, ResolutionMs: 1000}}
	validation := strings.Fields(prob.Validation)
	interactive := slices.Contains(validation, "interactive")
	types := strings.Fields(prob.Type)
	scoring := slices.Contains(types, "scoring")
	if scoring {
		p.Grading = spec.GradingPoints
	}
	switch {
	case slices.Contains(validation, "score"):
		return nil, spec.Fail(pkg.ErrUnsupported, "problem.yaml", "score", "output validators that score are not supported")
	case slices.Contains(types, "submit-answer") && interactive:
		return nil, spec.Fail(pkg.ErrUnsupported, "problem.yaml", "submit-answer", "submit-answer problems cannot be interactive")
	case slices.Contains(types, "submit-answer"):
		p.Type = "output-only"
	case interactive:
		p.Type = "interactive"
	}
	custom := len(validation) > 0 && validation[0] == "custom"
	if prob.Limits != nil {
		if prob.Limits.TimeMultiplier > 0 {
			p.TimeRule.AcceptedFactor = prob.Limits.TimeMultiplier
		}
		if prob.Limits.TimeSafetyMargin > 0 {
			p.TimeRule.TLEFactor = prob.Limits.TimeSafetyMargin
		}
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
	if (custom || interactive) && len(output) == 0 {
		return nil, spec.Fail(pkg.ErrPackageLayout, "output_validators", "", "problem.yaml asks for a custom output validator, but output_validators/ is missing or empty")
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
	if p.Solutions, p.MainSolution, err = submissions(fsys, collect, scoring); err != nil {
		return nil, err
	}
	tree := testdata{fsys: fsys, scoring: scoring, legacy: prob.Grading}
	if p.Testset, err = tree.read(); err != nil {
		return nil, err
	}
	if err := hasTests(fsys, &p.Testset); err != nil {
		return nil, err
	}
	p.Statements = statements(fsys, p)
	prob.Name.title(p)
	return p, nil
}

// hasTests explains a data/ without test cases, which generator-based tools leave until they run.
func hasTests(fsys fs.FS, root *spec.Group) error {
	for g := range root.All() {
		if len(g.Tests) > 0 {
			return nil
		}
	}
	for _, generator := range []string{"generators/generators.yaml", "data/generator.sh", "data/generator", "data/generate.sh"} {
		if regular(fsys, generator) {
			return spec.Fail(pkg.ErrUnsupported, generator, "", "data/ has no test cases because %s makes them, and Gate does not run it: generate the tests and import the package with them", generator)
		}
	}
	return spec.Fail(pkg.ErrPackageLayout, "data", "", "data/ has no test cases: add .in files, with .ans answers, under data/sample and data/secret")
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

// expectation is a submissions.yaml rule; the verdicts are AC, WA, TLE and RTE.
type expectation struct {
	Permitted []string `yaml:"permitted"`
	Required  []string `yaml:"required"`
	Score     any      `yaml:"score"`
}

// Solutions in other folders are built only when submissions.yaml sets their verdicts.
var folderRules = map[string]expectation{
	"accepted":            {Permitted: []string{"AC"}},
	"rejected":            {Required: []string{"RTE", "TLE", "WA"}},
	"wrong_answer":        {Permitted: []string{"AC", "WA"}, Required: []string{"WA"}},
	"time_limit_exceeded": {Permitted: []string{"AC", "TLE"}, Required: []string{"TLE"}},
	"run_time_error":      {Permitted: []string{"AC", "RTE"}, Required: []string{"RTE"}},
	"brute_force":         {Permitted: []string{"AC", "RTE", "TLE"}, Required: []string{"RTE", "TLE"}},
}

var verdictClasses = []struct {
	icpc string
	gate []string
}{{"AC", []string{"OK"}}, {"WA", []string{"WA", "PE", "OLE"}}, {"TLE", []string{"TLE"}}, {"RTE", []string{"RE", "MLE"}}}

// verdicts approximates the rules by Gate's expectation: every test in the permitted set, and some failing unless AC is required.
// A rule setting none of these still runs the submission, with every verdict permitted.
func verdicts(rules []expectation) []string {
	permitted := []string{"AC", "WA", "TLE", "RTE"}
	fails, set := false, false
	for _, r := range rules {
		if r.Permitted != nil {
			permitted = slices.DeleteFunc(permitted, func(v string) bool { return !slices.Contains(r.Permitted, v) })
		}
		if r.Required != nil {
			fails = fails || !slices.Contains(r.Required, "AC")
		}
		set = set || r.Permitted != nil || r.Required != nil || r.Score != nil
	}
	if !set {
		return nil
	}
	var result []string
	for _, class := range verdictClasses {
		if slices.Contains(permitted, class.icpc) && (class.icpc != "AC" || !fails) {
			result = append(result, class.gate...)
		}
	}
	return result
}

func submissions(fsys fs.FS, collect func(string) ([]spec.Program, error), scoring bool) ([]spec.Program, string, error) {
	verdictDirs, err := fs.ReadDir(fsys, "submissions")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read submissions: %w", err)
	}
	rules := map[string]expectation{}
	if regular(fsys, "submissions/submissions.yaml") {
		data, err := spec.ReadFile(fsys, "submissions/submissions.yaml")
		if err != nil {
			return nil, "", err
		}
		if err := yaml.Unmarshal(data, &rules); err != nil {
			return nil, "", spec.Syntax("submissions/submissions.yaml", err)
		}
	}
	var result []spec.Program
	main := ""
	for _, dir := range verdictDirs {
		if !dir.IsDir() {
			continue
		}
		list, err := collect(path.Join("submissions", dir.Name()))
		if err != nil {
			return nil, "", err
		}
		for i := range list {
			name := strings.TrimPrefix(list[i].Path, "submissions/")
			var matched []expectation
			if r, ok := folderRules[dir.Name()]; ok {
				matched = append(matched, r)
			}
			for _, pattern := range slices.Sorted(maps.Keys(rules)) {
				if matches(pattern, name) {
					matched = append(matched, rules[pattern])
				}
			}
			list[i].Verdicts = verdicts(matched)
			for _, r := range matched {
				if score, ok := exactScore(r.Score); ok && scoring {
					list[i].Points = &score
				}
			}
		}
		if dir.Name() == "accepted" && len(list) > 0 {
			main = list[0].Path
		}
		result = append(result, list...)
	}
	return result, main, nil
}

// matches reports whether a submissions.yaml glob names the submission or a directory above it.
func matches(pattern, name string) bool {
	for _, p := range expandBraces(pattern) {
		for candidate := name; candidate != "."; candidate = path.Dir(candidate) {
			if ok, _ := path.Match(p, candidate); ok {
				return true
			}
		}
	}
	return false
}

func expandBraces(pattern string) []string {
	open := strings.IndexByte(pattern, '{')
	end := strings.IndexByte(pattern[open+1:], '}')
	if open < 0 || end < 0 {
		return []string{pattern}
	}
	end += open + 1
	var result []string
	for _, alt := range strings.Split(pattern[open+1:end], ",") {
		result = append(result, expandBraces(pattern[:open]+alt+pattern[end+1:])...)
	}
	return result
}

// exactScore reads a score given as one number or a range of one number; Gate expects exact points.
func exactScore(score any) (points.Value, bool) {
	number := func(v any) (float64, bool) {
		switch n := v.(type) {
		case int:
			return float64(n), true
		case float64:
			return n, true
		}
		return 0, false
	}
	if r, ok := score.([]any); ok && len(r) == 2 {
		low, okLow := number(r[0])
		high, okHigh := number(r[1])
		if !okLow || !okHigh || low != high {
			return 0, false
		}
		score = r[0]
	}
	f, ok := number(score)
	if !ok {
		return 0, false
	}
	return points.FromFloat(f)
}

// Legacy packages keep LaTeX in problem_statement/, 2023-07 ones Markdown in statement/; other files there are images.
// Tutorials are solution.<language> files there or in solution/.
func statements(fsys fs.FS, p *spec.Problem) []spec.Statement {
	var result []spec.Statement
	seen := map[string]bool{}
	tutorials, pdfs := map[string]string{}, map[string]string{}
	for _, dir := range []string{"statement", "problem_statement", "solution"} {
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
			tutorial, solution := strings.CutPrefix(base, "solution.")
			if base == "solution" {
				tutorial, solution = "en", true
			}
			if solution && (ext == ".tex" || ext == ".md") && tutorials[spec.LanguageCode(tutorial)] == "" {
				tutorials[spec.LanguageCode(tutorial)] = path.Join(dir, name)
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
			case named && ext == ".pdf" && dir == "statement" && pdfs[language] == "":
				pdfs[language] = name
				p.AddAsset(name, path.Join(dir, name))
			case spec.IsImage(name):
				p.AddAsset(name, path.Join(dir, name))
			}
		}
	}
	// A PDF is the statement only of languages without LaTeX or Markdown.
	for _, language := range slices.Sorted(maps.Keys(pdfs)) {
		if !seen[language] {
			result = append(result, spec.Statement{Language: language, PDF: pdfs[language]})
		}
	}
	for _, language := range slices.Sorted(maps.Keys(tutorials)) {
		spec.AddTutorial(result, language, tutorials[language])
	}
	return result
}

func regular(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
