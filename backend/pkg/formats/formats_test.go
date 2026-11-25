package formats

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

type summary struct {
	Type, Checker, Interactor, Main             string
	Validators, Generators, Library, Statements []string
	// Groups lists "name/aggregate/tests/samples/points" for the testset and its subgroups.
	Groups []string
}

func summarize(p *spec.Problem) summary {
	s := summary{Type: p.Type + "/" + p.Grading, Main: p.MainSolution}
	name := func(program *spec.Program) string {
		if program == nil {
			return ""
		}
		return program.Name
	}
	s.Checker, s.Interactor = name(p.Checker), name(p.Interactor)
	for _, v := range p.Validators {
		s.Validators = append(s.Validators, v.Name)
	}
	for _, g := range p.Generators {
		s.Generators = append(s.Generators, g.Name)
	}
	for _, f := range p.Library {
		s.Library = append(s.Library, f.Name)
	}
	for _, st := range p.Statements {
		s.Statements = append(s.Statements, st.Language)
	}
	for g := range p.Testset.All() {
		samples, total := 0, points.Value(0)
		for _, t := range g.Tests {
			if t.Sample {
				samples++
			}
			total += t.Points
		}
		s.Groups = append(s.Groups, g.Name+"/"+cmp.Or(g.Aggregate, "sum")+"/"+itoa(len(g.Tests))+"/"+itoa(samples)+"/"+total.String())
	}
	return s
}

// tests lists the tests of the testset, parents' first.
func tests(p *spec.Problem) []spec.Test {
	var result []spec.Test
	for g := range p.Testset.All() {
		result = append(result, g.Tests...)
	}
	return result
}

func itoa(n int) string { return points.Value(n * int(points.Unit)).String() }

func TestReadTestdataPackages(t *testing.T) {
	testlib := []string{"testlib.h"}
	icpcLib := []string{"testlib.h", "testlib.h", "testlib.h"}
	cases := map[string]summary{
		"gfmt/a-plus-b": {Type: "standard/pass-fail", Checker: "checker.cpp", Validators: []string{"validator.cpp"}, Main: "solutions/ok.cpp", Generators: []string{"echo.cpp"}, Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"/sum/0.00/0.00/0.00", "samples/sum/3.00/3.00/0.00", "secret/sum/6.00/0.00/0.00"}},
		"gfmt/interactive-guess": {Type: "interactive/pass-fail", Checker: "checker.cpp", Interactor: "interactor.cpp", Main: "solutions/ok.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/sum/0.00/0.00/0.00", "samples/sum/1.00/1.00/0.00", "secret/sum/1.00/0.00/0.00"}},
		"polygon/a-plus-b": {Type: "standard/pass-fail", Checker: "check.cpp", Validators: []string{"v.cpp"}, Main: "solutions/solution_mm.cpp", Generators: []string{"echo.cpp"}, Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"/sum/12.00/3.00/0.00"}},
		"polygon/interactive-guess": {Type: "interactive/pass-fail", Checker: "check.cpp", Validators: []string{"validator.cpp"}, Interactor: "interactor.cpp", Main: "solutions/accepted.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/sum/2.00/1.00/0.00"}},
		"polygon/max-subarray": {Type: "standard/pass-fail", Checker: "check.cpp", Validators: []string{"validator.cpp"}, Main: "solutions/accepted.cpp", Generators: []string{"generator.cpp"}, Library: testlib, Statements: []string{"en"},
			Groups: []string{"/sum/4.00/2.00/0.00"}},
		"polygon/subtasks-groups": {Type: "standard/points", Checker: "check.cpp", Validators: []string{"validator.cpp"}, Main: "solutions/accepted.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/sum/0.00/0.00/0.00", "1/min/1.00/1.00/40.00", "2/min/1.00/0.00/60.00"}},
		"icpc/a-plus-b": {Type: "standard/pass-fail", Checker: "check.cpp", Validators: []string{"v.cpp"}, Main: "submissions/accepted/accepted.cpp", Generators: []string{"echo.cpp"}, Library: icpcLib, Statements: []string{"en", "ru"},
			Groups: []string{"/sum/0.00/0.00/0.00", "sample/sum/3.00/3.00/0.00", "secret/sum/9.00/0.00/0.00"}},
		// An interactive input is what the interactor reads, so none is published.
		"icpc/interactive-guess": {Type: "interactive/pass-fail", Validators: []string{"validator.cpp"}, Interactor: "interactor.cpp", Main: "submissions/accepted/accepted.cpp", Library: []string{"testlib.h", "testlib.h"}, Statements: []string{"en"},
			Groups: []string{"/sum/0.00/0.00/0.00", "sample/sum/1.00/1.00/0.00", "secret/sum/1.00/0.00/0.00"}},
		// Kattis packages of version 2025-09; the .ctd input validator is not a program Gate runs.
		"icpc/different": {Type: "standard/pass-fail", Checker: "validate.cc", Validators: []string{"validate.py"}, Main: "submissions/accepted/different.c", Library: []string{"validate.h"}, Statements: []string{"en"},
			Groups: []string{"/sum/0.00/0.00/0.00", "sample/sum/1.00/1.00/0.00", "secret/sum/2.00/0.00/0.00"}},
		"icpc/guess": {Type: "interactive/pass-fail", Validators: []string{"validate.py"}, Interactor: "validate.cc", Main: "submissions/accepted/guess.cc", Library: []string{"validate.h"}, Statements: []string{"en"},
			Groups: []string{"/sum/0.00/0.00/0.00", "secret/sum/10.00/0.00/0.00"}},
		"pcms2/a-plus-b": {Type: "standard/pass-fail", Checker: "check.cpp", Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"/sum/12.00/3.00/0.00"}},
		"pcms2/interactive-guess": {Type: "interactive/pass-fail", Checker: "check.cpp", Interactor: "interactor.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/sum/2.00/1.00/0.00"}},
		"pcms2/max-subarray": {Type: "standard/pass-fail", Checker: "check.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/sum/4.00/2.00/0.00"}},
		"pcms2/subtasks-groups": {Type: "standard/points", Checker: "check.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/sum/0.00/0.00/0.00", "1/min/1.00/1.00/40.00", "2/min/1.00/0.00/60.00"}},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := Read(filepath.Join("..", "testdata", name))
			require.NoError(t, err)
			require.Equal(t, want, summarize(p))
		})
	}
}

func TestPolygonGroups(t *testing.T) {
	p, err := Read(unpack(t, "polygon-groups"))
	require.NoError(t, err)
	require.Equal(t, [2]string{"standard", "points"}, [2]string{p.Type, p.Grading})
	require.Equal(t, [2]string{"input.txt", "output.txt"}, [2]string{p.InputFile, p.OutputFile})
	require.Equal(t, "gen.cpp", p.Generators[0].Path)
	require.Equal(t, spec.Group{Groups: []spec.Group{
		{Name: "samples", Aggregate: "sum", Feedback: "complete", Tests: []spec.Test{{Input: "tests/01", Sample: true, Ordinal: 1}}},
		{Name: "main", Aggregate: "min", Feedback: "points", Requires: []string{"samples"}, Tests: []spec.Test{
			{Generator: "gen.cpp", Args: []string{"a b", "2"}, Points: 100 * points.Unit, Ordinal: 2},
			{Generator: "gen.cpp", Args: []string{"3"}, Points: 100 * points.Unit, Ordinal: 3},
		}},
	}}, p.Testset)
}

func TestPolygonPretests(t *testing.T) {
	p, err := Read(filepath.Join("..", "testdata", "polygon", "subtasks-groups"))
	require.NoError(t, err)
	var marks []bool
	for _, test := range tests(p) {
		marks = append(marks, test.Pretest)
	}
	require.Equal(t, []bool{true, false}, marks, "the pretest repeats test 1's input")
}

func TestPolygonPretestMatching(t *testing.T) {
	dir := unpack(t, "polygon-pretests")
	p, err := Read(dir)
	require.NoError(t, err)
	var marks []bool
	for _, test := range tests(p) {
		marks = append(marks, test.Pretest)
	}
	require.Equal(t, []bool{true, false, true, true}, marks)

	write(t, dir, "pretests/03", "5\n")
	_, err = Read(dir)
	require.ErrorContains(t, err, "pretest 3 is not one of the tests")
}

func TestPolygonTutorialKeepsProblemTeX(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("..", "testdata", "polygon", "interactive-guess"))))
	for _, name := range []string{"legend", "input", "output", "notes"} {
		require.NoError(t, os.Remove(filepath.Join(dir, "statement-sections", "english", name+".tex")))
	}
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, "statements/english/problem.tex", p.Statements[0].TeX)
	require.Equal(t, "statement-sections/english/tutorial.tex", p.Statements[0].Sections["tutorial"])
}

// A tutorial in a language without a statement still reaches the site.
func TestICPCTutorialWithoutItsStatement(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("..", "testdata", "icpc", "a-plus-b"))))
	write(t, dir, "problem_statement/solution.de.tex", "Lösung\n")
	p, err := Read(dir)
	require.NoError(t, err)
	var tutorials []string
	for _, s := range p.Statements {
		tutorials = append(tutorials, s.Tutorial)
	}
	require.Contains(t, tutorials, "problem_statement/solution.de.tex")
}

func TestPolygonResources(t *testing.T) {
	p, err := Read(unpack(t, "polygon-resources"))
	require.NoError(t, err)
	require.Equal(t, []spec.File{{Name: "testlib.h", Path: "files/testlib.h"}, {Name: "helper.inc", Path: "files/helper.inc"}}, p.Library)
}

func TestICPCGroups(t *testing.T) {
	p, err := Read(unpack(t, "icpc-groups"))
	require.NoError(t, err)
	require.Equal(t, summary{Type: "standard/pass-fail", Checker: "checker.cpp", Library: []string{"testlib.h"},
		Groups: []string{"/sum/0.00/0.00/0.00", "sample/sum/1.00/1.00/0.00", "secret/sum/0.00/0.00/0.00", "secret/group1/sum/1.00/0.00/0.00", "secret/group2/sum/0.00/0.00/0.00", "secret/group2/nested/sum/1.00/0.00/0.00"}}, summarize(p))
	require.Equal(t, "output_validators/checker/checker.cpp", p.Checker.Path)
	require.Empty(t, tests(p)[2].Answer)
}

func TestICPCProtocols(t *testing.T) {
	protocols := func(p *spec.Problem) []string {
		result := []string{p.Checker.Protocol}
		for _, v := range p.Validators {
			result = append(result, v.Protocol)
		}
		return result
	}
	p, err := Read(filepath.Join("..", "testdata", "icpc", "a-plus-b"))
	require.NoError(t, err)
	require.Equal(t, []string{"testlib", "testlib"}, protocols(p))
	p, err = Read(filepath.Join("..", "testdata", "icpc", "icpc-protocol"))
	require.NoError(t, err)
	require.Equal(t, []string{"icpc", "icpc"}, protocols(p))

	dir := filepath.Join(t.TempDir(), "float-default")
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("..", "testdata", "icpc", "float-default"))))
	p, err = Read(dir)
	require.NoError(t, err)
	require.Equal(t, "default_validator.cpp", p.Checker.Name)
	require.Equal(t, []string{"icpc", "float_tolerance", "1e-6"}, append(protocols(p), p.Checker.Args...))
	write(t, dir, "data/secret/testdata.yaml", "output_validator_flags: float_tolerance 1e-3\n")
	p, err = Read(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"float_tolerance", "1e-3"}, p.Testset.Groups[1].CheckerArgs, "group flags follow validator_flags")
	write(t, dir, "data/secret/testdata.yaml", "output_validator_flags: {name: v, flags: x}\n")
	_, err = Read(dir)
	require.ErrorContains(t, err, "output_validator_flags per validator is not supported")
}

// The ICPC default grader settings are inherited down data/ and resolved per group.
func TestICPCScoring(t *testing.T) {
	dir := unpack(t, "icpc-scoring")
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, [2]string{"standard", "points"}, [2]string{p.Type, p.Grading})
	test := func(input string, ordinal int, score points.Value, sample bool) spec.Test {
		return spec.Test{Input: input, Answer: strings.TrimSuffix(input, ".in") + ".ans", Points: score * points.Unit, Ordinal: ordinal, Sample: sample}
	}
	require.Equal(t, spec.Group{Aggregate: "sum", Groups: []spec.Group{
		{Name: "sample", Aggregate: "sum", Tests: []spec.Test{test("data/sample/1.in", 1, 0, true)}},
		{Name: "secret", Aggregate: "sum", Tests: []spec.Test{test("data/secret/z.in", 5, 1, false)}, Groups: []spec.Group{
			{Name: "secret/group1", Aggregate: "min", StopOnFailure: true, ValidatorArgs: []string{"--max", "10"},
				Tests: []spec.Test{test("data/secret/group1/a.in", 2, 30, false), test("data/secret/group1/b.in", 3, 30, false)}},
			{Name: "secret/group2", Aggregate: "min", Tests: []spec.Test{test("data/secret/group2/c.in", 4, 70, false)}},
		}},
	}}, p.Testset)
	write(t, dir, "problem.yaml", "name: Scoring\ntype: scoring submit-answer\nvalidation: custom\n")
	p, err = Read(dir)
	require.NoError(t, err)
	require.Equal(t, [2]string{"output-only", "points"}, [2]string{p.Type, p.Grading})
	require.Equal(t, 30*points.Unit, p.Testset.Groups[1].Groups[0].Tests[0].Points, "submit-answer keeps the scoring testset")
	write(t, dir, "problem.yaml", "name: Scoring\ntype: scoring\nvalidation: custom interactive\n")
	p, err = Read(dir)
	require.NoError(t, err)
	require.Equal(t, [2]string{"interactive", "points"}, [2]string{p.Type, p.Grading})
	require.NotNil(t, p.Interactor)
	require.Equal(t, 30*points.Unit, p.Testset.Groups[1].Groups[0].Tests[0].Points, "an interactive problem keeps the scoring testset")
	write(t, dir, "problem.yaml", "name: Scoring\ntype: scoring\nvalidation: custom\ngrading:\n  on_reject: first_error\n")
	p, err = Read(dir)
	require.NoError(t, err)
	for g := range p.Testset.All() {
		require.True(t, g.StopOnFailure, "problem.yaml grading overrides every group as on_reject: break")
	}
	for config, feature := range map[string]string{
		"grading: custom\n":                     "grading \"custom\" is not supported",
		"grader_flags: accept_if_any_accepted\n": "grader flag \"accept_if_any_accepted\" is not supported",
		"reject_score: 1\n":                      "reject_score other than 0 is not supported",
	} {
		write(t, dir, "data/secret/group2/testdata.yaml", config)
		_, err := Read(dir)
		require.ErrorContains(t, err, feature)
	}
}

// 2025-09 scores map onto the group tree: all-or-nothing and min groups give each test the group's score, sum groups average it.
func TestICPCCurrentScoring(t *testing.T) {
	dir := unpack(t, "icpc-current-scoring")
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, [3]string{"standard", "points", "default_validator.cpp"}, [3]string{p.Type, p.Grading, p.Checker.Name})
	test := func(stem string, ordinal int, score points.Value) spec.Test {
		return spec.Test{Input: "data/" + stem + ".in", Answer: "data/" + stem + ".ans", Points: score * points.Unit, Ordinal: ordinal}
	}
	sample := test("sample/1", 1, 0)
	sample.Sample, sample.StatementOutput = true, "data/sample/1.out"
	require.Equal(t, spec.Group{Aggregate: "sum", Groups: []spec.Group{
		{Name: "sample", Aggregate: "sum", Tests: []spec.Test{sample}},
		{Name: "secret", Aggregate: "sum", CheckerArgs: []string{"case_sensitive"}, Groups: []spec.Group{
			{Name: "secret/1-easy", Aggregate: "min", StopOnFailure: true, Requires: []string{"sample"},
				Tests: []spec.Test{test("secret/1-easy/a", 2, 30), test("secret/1-easy/nested/b", 3, 30)}},
			{Name: "secret/2-sum", Aggregate: "avg", Requires: []string{"secret/1-easy"}, CheckerArgs: []string{"float_tolerance", "1e-6"},
				Tests: []spec.Test{test("secret/2-sum/c", 4, 70), test("secret/2-sum/d", 5, 70)}},
		}},
	}}, p.Testset)
	half := 65 * points.Unit
	var solutions []string
	for _, s := range p.Solutions {
		solutions = append(solutions, s.Path+" "+strings.Join(s.Verdicts, ","))
	}
	require.Equal(t, []string{"submissions/accepted/first.cpp OK", "submissions/accepted/ok.cpp OK", "submissions/partial/half.py WA,PE,OLE",
		"submissions/scored/third.cpp OK,WA,PE,OLE,TLE,RE,MLE"}, solutions, "a rule with only a score runs the submission too")
	require.Equal(t, &half, p.Solutions[2].Points)
	require.Equal(t, "submissions/accepted/ok.cpp", p.MainSolution, "the model solution is the main one")
	require.Equal(t, "Scoring", p.Title)
	require.Equal(t, []spec.Statement{{Language: "en", Markdown: "statement/problem.en.md"}, {Language: "ru", Title: "Баллы", PDF: "problem.ru.pdf"}}, p.Statements)
	require.Equal(t, []spec.File{{Name: "tool.py", Path: "attachments/tool.py"}}, p.Attachments)

	write(t, dir, "data/secret/test_group.yaml", "score_aggregation: pass-fail\n")
	p, err = Read(dir)
	require.NoError(t, err)
	secret := p.Testset.Groups[1]
	require.Equal(t, [2]string{"min", "min"}, [2]string{secret.Aggregate, secret.Groups[1].Aggregate}, "all-or-nothing secret makes every group so")
	require.Equal(t, 100*points.Unit, secret.Groups[1].Tests[0].Points)
	write(t, dir, "data/secret/test_group.yaml", "")

	for file, change := range map[string][2]string{
		"problem.yaml":                       {"problem_format_version: 2025-09\nname: X\ntype: [pass-fail, multi-pass]\n", "multi-pass problems are not supported"},
		"data/secret/2-sum/test_group.yaml":  {"max_score: 70\nscore_aggregation: sum\nscore_mode: absolute\n", "score_mode absolute is not supported"},
		"data/secret/1-easy/test_group.yaml": {"score_aggregation: min\n", "unbounded max_score"},
		"data/secret/2-sum/c.yaml":           {"args: [x]\n", "args of a single test case are not supported"},
		"data/secret/2-sum/c.files/f.txt":    {"x", "input files are not supported"},
		"include/cpp/grader.h":               {"", "included files are not supported"},
		"data/secret/1-easy/nested/test_group.yaml": {"", "test data groups cannot nest"},
		"data/sample/test_group.yaml":        {"scoring: {score: 1}\n", "field scoring not found"},
	} {
		t.Run(change[1], func(t *testing.T) {
			dir := unpack(t, "icpc-current-scoring")
			write(t, dir, file, change[0])
			_, err := Read(dir)
			require.ErrorContains(t, err, change[1])
		})
	}
	write(t, dir, "problem.yaml", "problem_format_version: 2026-01\nname: X\n")
	_, err = Read(dir)
	require.ErrorContains(t, err, "problem format version 2026-01 is not supported")
}

func TestICPCRejectsSeveralOutputValidators(t *testing.T) {
	_, err := Read(unpack(t, "icpc-two-output-validators"))
	require.ErrorContains(t, err, "2 output validators")
}

func TestPCMS2ProgramHeaders(t *testing.T) {
	p, err := Read(unpack(t, "pcms2-program-headers"))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"testlib.h": "testlib.h"}, p.Checker.Dependencies)
	require.Equal(t, [2]string{"", "output.txt"}, [2]string{p.InputFile, p.OutputFile})
}

// Package paths cannot escape os.Root through traversal or symlinks.
func TestReadersStayInsidePackage(t *testing.T) {
	parent := t.TempDir()
	write(t, parent, "01.a", "outside\n")
	dir := filepath.Join(parent, "package")
	write(t, dir, "tests/01", "1\n")
	require.NoError(t, os.Symlink(filepath.Join(parent, "01.a"), filepath.Join(dir, "tests", "01.a")))
	for _, pattern := range []string{"../%02d.a", "tests/%02d.a"} {
		write(t, dir, "problem.xml", `<problem short-name="escape"><judging><testset name="tests">
  <time-limit>1000</time-limit><memory-limit>268435456</memory-limit>
  <input-path-pattern>tests/%02d</input-path-pattern><answer-path-pattern>`+pattern+`</answer-path-pattern>
  <tests><test method="manual"/></tests></testset></judging></problem>`)
		p, err := Read(dir)
		require.NoError(t, err)
		require.Empty(t, tests(p)[0].Answer, pattern)
	}
}

// Readers parse metadata of bounded size only.
func TestReadersBoundMetadata(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "problem.xml", `<!DOCTYPE problem PUBLIC "-//PCMS2//DTD Problem Description 1.0//EN" "http://pcms2.ru/dtd/problem-0.2.dtd">
<problem id="big"><judging><testset name="tests"><time-limit>1000</time-limit><memory-limit>268435456</memory-limit>
  <input-path-pattern>tests/%02d</input-path-pattern><tests><test/></tests></testset></judging></problem>`)
	write(t, dir, "tests/01", "1\n")
	write(t, dir, "statements/english/problem.tex", "\\exmp{1}{1}"+strings.Repeat(" ", spec.MaxMetadataBytes))
	p, err := Read(dir)
	require.NoError(t, err)
	require.False(t, tests(p)[0].Sample, "an oversized statement is not inspected")
	write(t, dir, "problem.xml", "<problem>"+strings.Repeat(" ", spec.MaxMetadataBytes)+"</problem>")
	_, err = Read(dir)
	require.ErrorContains(t, err, "larger than")
}

func TestYAMLFormatsRejectNullAndNonMappingConfiguration(t *testing.T) {
	for _, content := range []string{"null\n", "", "[]\n", "subtasks: ["} {
		t.Run(content, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "problem.yaml", content)
			_, err := Read(dir)
			require.Error(t, err)
		})
	}
}

func TestParseCommand(t *testing.T) {
	argv, err := spec.ParseCommand(`gen "a b" c\ d 'e"f' > $`)
	require.NoError(t, err)
	require.Equal(t, []string{"gen", "a b", "c d", `e"f`}, argv)
	for _, bad := range []string{"", "gen | cat", "gen > out", "gen 'open"} {
		_, err := spec.ParseCommand(bad)
		require.Error(t, err, bad)
	}
}

func unpack(t *testing.T, name string) string {
	t.Helper()
	archive, err := txtar.ParseFile(filepath.Join("testdata", name+".txtar"))
	require.NoError(t, err)
	fsys, err := txtar.FS(archive)
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, fsys))
	return dir
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
