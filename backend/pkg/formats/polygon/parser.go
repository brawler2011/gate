package polygon

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/statement"
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
	if p.Testset, err = groups(fsys, testset, generators); err != nil {
		return nil, err
	}
	if err := markPretests(fsys, testset, prob.Judging.Testsets, &p.Testset); err != nil {
		return nil, err
	}
	p.Type, p.Grading = problemType(prob), spec.TestGrading(&p.Testset)
	// Well-formed Polygon tests are text; Windows packages use CRLF.
	for _, property := range prob.Properties {
		if property.Name == "tests-wellformed" && property.Value == "true" {
			p.TextTests = true
		}
	}
	p.Statements = statements(fsys, p, prob.Statements, prob.Tutorials)
	p.SetExamples(fsys, spec.ExampleDir(fsys, "statement-sections"), func(t spec.Test) (string, string) {
		return fmt.Sprintf(testset.InputPathPattern, t.Ordinal), fmt.Sprintf(testset.AnswerPathPattern, t.Ordinal)
	})
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

func problemType(prob Problem) string {
	if prob.Interactive || (prob.Assets != nil && prob.Assets.Interactor != nil) {
		return "interactive"
	}
	return "standard"
}

// Ungrouped tests stay in the root; groups follow their first test, preserving Polygon test numbering.
func groups(fsys fs.FS, testset Testset, generators map[string]string) (spec.Group, error) {
	definitions := map[string]Group{}
	for _, g := range testset.Groups {
		definitions[g.Name] = g
	}
	root := spec.Group{}
	index := map[string]int{}
	for i, t := range testset.Tests {
		test, err := convertTest(fsys, testset, i+1, t, generators)
		if err != nil {
			return root, err
		}
		if t.Group == "" {
			root.Tests = append(root.Tests, test)
			continue
		}
		n, ok := index[t.Group]
		if !ok {
			n = len(root.Groups)
			index[t.Group] = n
			root.Groups = append(root.Groups, newGroup(t.Group, definitions))
		}
		root.Groups[n].Tests = append(root.Groups[n].Tests, test)
	}
	for i := range root.Groups {
		g := &root.Groups[i]
		// A complete group scores its points only when every test passes: the minimum of tests worth the whole group.
		if g.Aggregate == "min" {
			total := points.Value(0)
			for _, t := range g.Tests {
				total += t.Points
			}
			if def := definitions[g.Name]; total == 0 && def.Points > 0 {
				value, err := parsePoints(def.Points)
				if err != nil {
					return root, err
				}
				total = value
			}
			for j := range g.Tests {
				g.Tests[j].Points = total
			}
		}
		if g.Feedback != "" && !slices.Contains(spec.FeedbackPolicies, g.Feedback) {
			return root, spec.Fail(pkg.ErrUnknown, "problem.xml", g.Feedback, "group %s has unknown feedback policy %s", g.Name, g.Feedback)
		}
		for _, dep := range g.Requires {
			if _, ok := index[dep]; !ok {
				return root, spec.Fail(pkg.ErrUnknown, "problem.xml", dep, "group %s depends on unknown group %s", g.Name, dep)
			}
		}
	}
	return root, nil
}

// Named groups score as a whole unless marked each-test.
func newGroup(name string, definitions map[string]Group) spec.Group {
	def, ok := definitions[name]
	g := spec.Group{Name: name, Aggregate: "min", Feedback: def.FeedbackPolicy}
	if ok && def.PointsPolicy == "each-test" {
		g.Aggregate = "sum"
	}
	for _, dep := range def.Dependencies {
		g.Requires = append(g.Requires, dep.Group)
	}
	return g
}

func convertTest(fsys fs.FS, testset Testset, number int, t Test, generators map[string]string) (spec.Test, error) {
	weight, err := parsePoints(t.Points)
	if err != nil {
		return spec.Test{}, err
	}
	test := spec.Test{Points: weight, Sample: t.Sample, Ordinal: number}
	if t.Method == "generated" {
		steps, err := spec.Steps(t.Cmd, generators)
		if err != nil {
			return test, fmt.Errorf("test %d: %w", number, err)
		}
		test.Steps, test.File = steps, t.FromFile
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

// Polygon keeps each section in statement-sections/<language>, with the images it shows; problem.tex is the fallback.
func statements(fsys fs.FS, p *spec.Problem, list, tutorials []Statement) []spec.Statement {
	var result []spec.Statement
	seen := map[string]bool{}
	for _, s := range list {
		language := spec.LanguageCode(s.Language)
		if s.Type != "application/x-tex" || seen[language] {
			continue
		}
		seen[language] = true
		st := spec.Statement{Language: language, Charset: s.Charset, TeX: s.Path}
		dir := path.Join("statement-sections", s.Language)
		entries, _ := fs.ReadDir(fsys, dir)
		for _, e := range entries {
			section, ok := strings.CutSuffix(e.Name(), ".tex")
			if e.Type().IsRegular() && ok && (slices.Contains(statement.Order, section) || section == "tutorial") {
				if st.Sections == nil {
					st.Sections = map[string]string{}
				}
				st.Sections[section] = path.Join(dir, e.Name())
			}
		}
		images(fsys, p, dir)
		// A folder with only the tutorial leaves the statement to problem.tex.
		if slices.ContainsFunc(statement.Order, func(name string) bool { return st.Sections[name] != "" }) {
			st.TeX = ""
		} else {
			images(fsys, p, path.Dir(s.Path))
		}
		result = append(result, st)
	}
	for _, t := range tutorials {
		if t.Type == "application/x-tex" {
			images(fsys, p, path.Dir(t.Path))
			spec.AddTutorial(result, spec.LanguageCode(t.Language), t.Path)
		}
	}
	return result
}

// markPretests marks the main tests a pretests testset repeats, by generator command or input bytes.
func markPretests(fsys fs.FS, main Testset, testsets []Testset, root *spec.Group) error {
	i := slices.IndexFunc(testsets, func(ts Testset) bool { return ts.Name == "pretests" })
	if i < 0 {
		return nil
	}
	pretests := testsets[i]
	// groups() appends tests in testset order, so a group's k-th test is its k-th main test.
	group := map[string]*spec.Group{}
	for g := range root.All() {
		group[g.Name] = g
	}
	seen := map[string]int{}
	tests := make([]*spec.Test, len(main.Tests))
	for n, t := range main.Tests {
		tests[n] = &group[t.Group].Tests[seen[t.Group]]
		seen[t.Group]++
	}
	for n, pre := range pretests.Tests {
		number := n + 1
		match := -1
		if pre.Method == "generated" {
			argv, err := spec.ParseCommand(pre.Cmd)
			if err != nil {
				return fmt.Errorf("pretest %d: %w", number, err)
			}
			match = slices.IndexFunc(main.Tests, func(t Test) bool {
				other, err := spec.ParseCommand(t.Cmd)
				return t.Method == "generated" && err == nil && t.FromFile == pre.FromFile && slices.EqualFunc(argv, other, slices.Equal)
			})
		}
		if name := fmt.Sprintf(pretests.InputPathPattern, number); match < 0 && (pre.Method != "generated" || exists(fsys, name)) {
			input, err := spec.ReadFile(fsys, name)
			if err != nil {
				return err
			}
			for m := range main.Tests {
				if other, err := fs.ReadFile(fsys, fmt.Sprintf(main.InputPathPattern, m+1)); err == nil && bytes.Equal(input, other) {
					match = m
					break
				}
			}
		}
		if match < 0 {
			return spec.Fail(pkg.ErrUnknown, "problem.xml", strconv.Itoa(number), "pretest %d is not one of the tests", number)
		}
		tests[match].Pretest = true
	}
	return nil
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}

func images(fsys fs.FS, p *spec.Problem, dir string) {
	entries, _ := fs.ReadDir(fsys, dir)
	for _, e := range entries {
		if e.Type().IsRegular() {
			p.AddStatementFile(e.Name(), path.Join(dir, e.Name()))
		}
	}
}
