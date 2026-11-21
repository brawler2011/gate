package polygon

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/formats/tex"
	"github.com/brawler2011/gate/backend/pkg/points"
)

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
		InputFile:     prob.Judging.InputFile,
		OutputFile:    prob.Judging.OutputFile,
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
		if p.Checker.CheckerTests, err = checkerTests(assets.Checker.Testset); err != nil {
			return nil, err
		}
		roles[assets.Checker.Source.Path] = true
	}
	for _, v := range assets.Validators {
		validator, err := program(v.Source)
		if err != nil {
			return nil, err
		}
		if validator.ValidatorTests, err = validatorTests(v.Testset); err != nil {
			return nil, fmt.Errorf("validator %s: %w", v.Source.Path, err)
		}
		p.Validators = append(p.Validators, *validator)
		roles[v.Source.Path] = true
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
		verdicts, ok := tagVerdicts[strings.ToLower(s.Tag)]
		if !ok {
			return nil, spec.Fail(pkg.ErrUnknown, s.Source.Path, s.Tag, "solution %s has unknown tag %q", s.Source.Path, s.Tag)
		}
		solution.Verdicts = verdicts
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
	for _, name := range prob.Names {
		p.SetStatementTitle(tex.LanguageCode(name.Language), name.Value)
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
	return prob.ShortName
}

func program(s Source) (*spec.Program, error) {
	language, err := Language(s.Type)
	if err != nil {
		return nil, spec.Fail(pkg.ErrUnsupported, s.Path, s.Type, "%s: unsupported Polygon source type %q", s.Path, s.Type)
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
	return "", spec.Fail(pkg.ErrUnsupported, "", sourceType, "unsupported Polygon source type %q", sourceType)
}

var tagVerdicts = map[string][]string{
	"main":                            {"OK"},
	"accepted":                        {"OK"},
	"rejected":                        {"WA", "PE", "TLE", "MLE", "OLE", "RE"},
	"wrong-answer":                    {"WA"},
	"presentation-error":              {"PE"},
	"time-limit-exceeded":             {"TLE"},
	"time-limit-exceeded-or-accepted": {"TLE", "OK"},
	"time-limit-exceeded-or-memory-limit-exceeded": {"TLE", "MLE"},
	"memory-limit-exceeded":                        {"MLE"},
	"failed":                                       {"RE"},
	"do-not-run":                                   {},
}

var checkerVerdicts = map[string]string{"ok": "OK", "wrong-answer": "WA", "presentation-error": "PE", "crashed": "FAIL"}

func checkerTests(ts ProgramTestset) ([]spec.CheckerTest, error) {
	tests := make([]spec.CheckerTest, len(ts.Tests))
	for i, t := range ts.Tests {
		verdict, ok := checkerVerdicts[strings.ToLower(t.Verdict)]
		if !ok {
			return nil, spec.Fail(pkg.ErrUnknown, "problem.xml", t.Verdict, "checker test %d has unknown verdict %q", i+1, t.Verdict)
		}
		tests[i] = spec.CheckerTest{Input: fmt.Sprintf(ts.InputPathPattern, i+1), Output: fmt.Sprintf(ts.OutputPathPattern, i+1), Answer: fmt.Sprintf(ts.AnswerPathPattern, i+1), Verdict: verdict}
	}
	return tests, nil
}

func validatorTests(ts ProgramTestset) ([]spec.ValidatorTest, error) {
	tests := make([]spec.ValidatorTest, len(ts.Tests))
	for i, t := range ts.Tests {
		verdict := strings.ToLower(t.Verdict)
		if verdict != "valid" && verdict != "invalid" {
			return nil, spec.Fail(pkg.ErrUnknown, "problem.xml", t.Verdict, "test %d has unknown verdict %q", i+1, t.Verdict)
		}
		tests[i] = spec.ValidatorTest{Input: fmt.Sprintf(ts.InputPathPattern, i+1), Valid: verdict == "valid", Group: t.Group}
	}
	return tests, nil
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
				return nil, spec.Fail(pkg.ErrUnknown, "problem.xml", dep, "group %s depends on unknown group %s", g.Name, dep)
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
			return test, spec.Fail(pkg.ErrUnknown, "problem.xml", argv[0], "test %d: generator %s not found", number, argv[0])
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
	text := strconv.FormatFloat(value, 'f', -1, 64)
	v, err := points.Parse(text)
	if err != nil {
		return 0, spec.Fail(pkg.ErrFormat, "problem.xml", text, "invalid points %s: %v", text, err)
	}
	return v, nil
}

func statements(list []Statement) []spec.Statement {
	var result []spec.Statement
	seen := map[string]bool{}
	for _, s := range list {
		language := tex.LanguageCode(s.Language)
		if s.Type == "application/x-tex" && !seen[language] {
			seen[language] = true
			result = append(result, spec.Statement{Language: language, Charset: s.Charset, TeX: s.Path})
		}
	}
	return result
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}
