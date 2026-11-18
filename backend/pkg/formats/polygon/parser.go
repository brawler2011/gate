package polygon

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/formats/tex"
	"github.com/brawler2011/gate/backend/pkg/points"
)

func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := spec.ReadFile(fsys, "problem.xml")
	if err != nil {
		return nil, fmt.Errorf("read problem.xml: %w", err)
	}
	var prob Problem
	if err := xml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("parse Polygon problem.xml: %w", err)
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
	assets := prob.Assets
	if assets == nil {
		assets = &Assets{}
	}
	roles := map[string]bool{}
	if assets.Checker != nil {
		if p.Checker, err = program(assets.Checker.Source); err != nil {
			return nil, err
		}
		// Polygon names standard checkers with the std:: prefix.
		if name, ok := strings.CutPrefix(assets.Checker.Name, "std::"); ok {
			p.Checker.Standard = strings.TrimSuffix(name, ".cpp")
		}
		roles[assets.Checker.Source.Path] = true
	}
	switch len(assets.Validators) {
	case 0:
	case 1:
		if p.Validator, err = program(assets.Validators[0].Source); err != nil {
			return nil, err
		}
		roles[assets.Validators[0].Source.Path] = true
	default:
		return nil, fmt.Errorf("package has %d validators; Gate supports one", len(assets.Validators))
	}
	if assets.Interactor != nil {
		if p.Interactor, err = program(assets.Interactor.Source); err != nil {
			return nil, err
		}
		roles[assets.Interactor.Source.Path] = true
	}
	for _, s := range assets.Solutions {
		solution, err := program(s.Source)
		if err != nil {
			return nil, err
		}
		p.Solutions = append(p.Solutions, *solution)
	}
	p.MainSolution = mainSolution(assets.Solutions)
	generators := map[string]string{}
	if prob.Files != nil {
		for _, e := range prob.Files.Executables {
			source := e.Source.Path
			if roles[source] || !spec.IsSourceFile(source) {
				continue
			}
			generator, err := program(e.Source)
			if err != nil {
				return nil, err
			}
			p.Generators = append(p.Generators, *generator)
			generators[spec.Stem(source)] = source
		}
		for _, r := range prob.Files.Resources {
			switch path.Ext(r.Path) {
			case ".sty", ".tex", ".ftl":
			default:
				p.Library = append(p.Library, spec.File{Name: path.Base(r.Path), Path: r.Path})
			}
		}
	}
	if p.Groups, err = groups(fsys, testset, generators); err != nil {
		return nil, err
	}
	p.Type = problemType(prob, p.Groups)
	p.KeepInteractiveInputsPrivate()
	// Well-formed Polygon tests are text; Windows packages use CRLF.
	for _, property := range prob.Properties {
		if property.Name == "tests-wellformed" && property.Value == "true" {
			p.TextTests = true
		}
	}
	p.Statements = statements(prob.Statements)
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
	return prob.ShortName
}

func program(s Source) (*spec.Program, error) {
	language, err := Language(s.Type)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Path, err)
	}
	return &spec.Program{File: spec.File{Name: path.Base(s.Path), Path: s.Path}, Language: language}, nil
}

var (
	// GCC type names encode versions and builds; a bare cpp.g++ specifies no standard.
	gccType  = regexp.MustCompile(`^cpp\.(?:[a-z0-9]+(?:-[a-z0-9]+)*-)?g\+\+(\d\d)$`)
	javaType = regexp.MustCompile(`^java\d*$`)
)

// Compiler versions share a configuration; language standards and implementations must match exactly.
// An unsupported source type is an error; an absent type defers to the file extension.
func Language(sourceType string) (string, error) {
	if m := gccType.FindStringSubmatch(sourceType); m != nil {
		return "cpp" + m[1], nil
	}
	switch {
	case sourceType == "":
		return "", nil
	case strings.HasPrefix(sourceType, "c.gcc"):
		return "c", nil
	case sourceType == "python.3":
		return "python3", nil
	case sourceType == "python.pypy3" || sourceType == "python.pypy3-64":
		return "pypy3", nil
	case javaType.MatchString(sourceType):
		return "java", nil
	case sourceType == "go":
		return "go", nil
	case sourceType == "rust":
		return "rust", nil
	case sourceType == "js.node" || sourceType == "javascript.node":
		return "node", nil
	}
	return "", fmt.Errorf("unsupported Polygon source type %q", sourceType)
}

func mainSolution(solutions []Solution) string {
	for _, tag := range []string{"main", "accepted"} {
		for _, s := range solutions {
			if strings.EqualFold(s.Tag, tag) {
				return s.Source.Path
			}
		}
	}
	return ""
}

func problemType(prob Problem, groups []spec.Group) string {
	if prob.Interactive || (prob.Assets != nil && prob.Assets.Interactor != nil) {
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

// Groups follow their first test, preserving Polygon test numbering.
func groups(fsys fs.FS, testset Testset, generators map[string]string) ([]spec.Group, error) {
	definitions := map[string]Group{}
	for _, g := range testset.Groups {
		definitions[g.Name] = g
	}
	var result []spec.Group
	index := map[string]int{}
	for i, t := range testset.Tests {
		test, err := convertTest(fsys, testset, i+1, t, generators)
		if err != nil {
			return nil, err
		}
		name := t.Group
		if name == "" {
			name = "tests"
		}
		n, ok := index[name]
		if !ok {
			n = len(result)
			index[name] = n
			result = append(result, newGroup(name, definitions, t.Group != ""))
		}
		result[n].Tests = append(result[n].Tests, test)
	}
	for i := range result {
		g := &result[i]
		// Complete-group points are awarded on the last test only when the whole group passes.
		total := points.Value(0)
		for _, t := range g.Tests {
			total += t.Points
		}
		if def, ok := definitions[g.Name]; ok && g.Policy == "complete" && total == 0 && def.Points > 0 {
			value, err := parsePoints(def.Points)
			if err != nil {
				return nil, err
			}
			g.Tests[len(g.Tests)-1].Points = value
		}
		for _, dep := range g.DependsOn {
			if _, ok := index[dep]; !ok {
				return nil, fmt.Errorf("group %s depends on unknown group %s", g.Name, dep)
			}
		}
	}
	return result, nil
}

// Named groups score as a whole unless marked each-test; ungrouped tests score individually.
func newGroup(name string, definitions map[string]Group, named bool) spec.Group {
	def, ok := definitions[name]
	if !ok {
		policy := "each"
		if named {
			policy = "complete"
		}
		return spec.Group{Name: name, Policy: policy}
	}
	g := spec.Group{Name: name, Policy: "complete"}
	if def.PointsPolicy == "each-test" {
		g.Policy = "each"
	}
	for _, dep := range def.Dependencies {
		g.DependsOn = append(g.DependsOn, dep.Group)
	}
	return g
}

func convertTest(fsys fs.FS, testset Testset, number int, t Test, generators map[string]string) (spec.Test, error) {
	weight, err := parsePoints(t.Points)
	if err != nil {
		return spec.Test{}, err
	}
	test := spec.Test{Points: weight, Sample: t.Sample}
	if t.Method == "generated" {
		argv, err := spec.ParseCommand(t.Cmd)
		if err != nil {
			return test, fmt.Errorf("test %d: %w", number, err)
		}
		name, ok := generators[argv[0]]
		if !ok {
			return test, fmt.Errorf("test %d: generator %s not found", number, argv[0])
		}
		test.Generator, test.Args = name, argv[1:]
		return test, nil
	}
	test.Input = fmt.Sprintf(testset.InputPathPattern, number)
	if answer := fmt.Sprintf(testset.AnswerPathPattern, number); exists(fsys, answer) {
		test.Answer = answer
	}
	return test, nil
}

func parsePoints(value float64) (points.Value, error) {
	return points.Parse(strconv.FormatFloat(value, 'f', -1, 64))
}

func statements(list []Statement) []spec.Statement {
	var result []spec.Statement
	seen := map[string]bool{}
	for _, s := range list {
		language := tex.LanguageCode(s.Language)
		if s.Type == "application/x-tex" && !seen[language] {
			seen[language] = true
			result = append(result, spec.Statement{Language: language, TeX: s.Path})
		}
	}
	return result
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
