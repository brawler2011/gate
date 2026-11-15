package formats

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/stretchr/testify/require"
)

// summary is what a package means, independent of where its files are.
type summary struct {
	Type, Checker, Validator, Interactor, Main string
	Generators, Library, Statements            []string
	// Groups lists "name/policy/tests/samples/points".
	Groups []string
}

func summarize(p *spec.Problem) summary {
	s := summary{Type: p.Type, Main: p.MainSolution}
	name := func(program *spec.Program) string {
		if program == nil {
			return ""
		}
		return program.Name
	}
	s.Checker, s.Validator, s.Interactor = name(p.Checker), name(p.Validator), name(p.Interactor)
	for _, g := range p.Generators {
		s.Generators = append(s.Generators, g.Name)
	}
	for _, f := range p.Library {
		s.Library = append(s.Library, f.Name)
	}
	for _, st := range p.Statements {
		s.Statements = append(s.Statements, st.Language)
	}
	for _, g := range p.Groups {
		samples, total := 0, points.Value(0)
		for _, t := range g.Tests {
			if t.Sample {
				samples++
			}
			total += t.Points
		}
		s.Groups = append(s.Groups, g.Name+"/"+g.Policy+"/"+itoa(len(g.Tests))+"/"+itoa(samples)+"/"+total.String())
	}
	return s
}

func itoa(n int) string { return points.Value(n * int(points.Unit)).String() }

func TestReadTestdataPackages(t *testing.T) {
	testlib := []string{"testlib.h"}
	icpcLib := []string{"testlib.h", "testlib.h", "testlib.h"}
	cases := map[string]summary{
		"gfmt/a-plus-b": {Type: "pass-fail", Checker: "checker.cpp", Validator: "validator.cpp", Main: "solutions/ok.cpp", Generators: []string{"echo.cpp"}, Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"samples/complete/3.00/3.00/0.00", "secret/each/6.00/0.00/0.00"}},
		"gfmt/interactive-guess": {Type: "interactive", Checker: "checker.cpp", Interactor: "interactor.cpp", Main: "solutions/ok.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"samples/complete/1.00/1.00/0.00", "secret/each/1.00/0.00/0.00"}},
		"polygon/a-plus-b": {Type: "pass-fail", Checker: "check.cpp", Validator: "v.cpp", Main: "solutions/solution_mm.cpp", Generators: []string{"echo.cpp"}, Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"tests/each/12.00/3.00/0.00"}},
		"polygon/interactive-guess": {Type: "interactive", Checker: "check.cpp", Validator: "validator.cpp", Interactor: "interactor.cpp", Main: "solutions/accepted.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"tests/each/2.00/0.00/0.00"}},
		"polygon/max-subarray": {Type: "pass-fail", Checker: "check.cpp", Validator: "validator.cpp", Main: "solutions/accepted.cpp", Generators: []string{"generator.cpp"}, Library: testlib, Statements: []string{"en"},
			Groups: []string{"tests/each/4.00/2.00/0.00"}},
		"polygon/subtasks-groups": {Type: "scoring", Checker: "check.cpp", Validator: "validator.cpp", Main: "solutions/accepted.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"1/complete/1.00/1.00/40.00", "2/complete/1.00/0.00/60.00"}},
		"icpc/a-plus-b": {Type: "pass-fail", Checker: "check.cpp", Validator: "v.cpp", Main: "submissions/accepted/accepted.cpp", Generators: []string{"echo.cpp"}, Library: icpcLib, Statements: []string{"en", "ru"},
			Groups: []string{"sample/each/3.00/3.00/0.00", "secret/each/9.00/0.00/0.00"}},
		// An interactive input is what the interactor reads, so none is published.
		"icpc/interactive-guess": {Type: "interactive", Validator: "validator.cpp", Interactor: "interactor.cpp", Main: "submissions/accepted/accepted.cpp", Library: []string{"testlib.h", "testlib.h"}, Statements: []string{"en"},
			Groups: []string{"sample/each/1.00/0.00/0.00", "secret/each/1.00/0.00/0.00"}},
		"pcms2/a-plus-b": {Type: "pass-fail", Checker: "check.cpp", Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"tests/each/12.00/3.00/0.00"}},
		"pcms2/interactive-guess": {Type: "interactive", Checker: "check.cpp", Interactor: "interactor.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"tests/each/2.00/0.00/0.00"}},
		"pcms2/max-subarray": {Type: "pass-fail", Checker: "check.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"tests/each/4.00/2.00/0.00"}},
		"pcms2/subtasks-groups": {Type: "scoring", Checker: "check.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"1/complete/1.00/1.00/40.00", "2/complete/1.00/0.00/60.00"}},
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
	dir := t.TempDir()
	write(t, dir, "tests/01", "1\n")
	write(t, dir, "gen.cpp", "int main() {}\n")
	write(t, dir, "problem.xml", `<problem short-name="groups">
  <judging><testset name="tests">
    <time-limit>1000</time-limit><memory-limit>268435456</memory-limit>
    <input-path-pattern>tests/%02d</input-path-pattern><answer-path-pattern>tests/%02d.a</answer-path-pattern>
    <tests>
      <test method="manual" sample="true" group="samples"/>
      <test method="generated" cmd="gen 'a b' 2" group="main" points="0"/>
      <test method="generated" cmd="gen 3" group="main"/>
    </tests>
    <groups>
      <group name="samples" points-policy="each-test" points="0"/>
      <group name="main" points-policy="complete-group" points="100"><dependencies><dependency group="samples"/></dependencies></group>
    </groups>
  </testset></judging>
  <files><executables><executable><source path="gen.cpp" type="cpp.g++17"/></executable></executables></files>
</problem>`)
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, "scoring", p.Type)
	require.Equal(t, "gen.cpp", p.Generators[0].Path)
	require.Len(t, p.Groups, 2)
	require.Equal(t, spec.Group{Name: "samples", Policy: "each", Tests: []spec.Test{{Input: "tests/01", Sample: true}}}, p.Groups[0])
	// The group's points are earned by passing all of it, as on its last test.
	require.Equal(t, spec.Group{Name: "main", Policy: "complete", DependsOn: []string{"samples"}, Tests: []spec.Test{
		{Generator: "gen.cpp", Args: []string{"a b", "2"}},
		{Generator: "gen.cpp", Args: []string{"3"}, Points: 100 * points.Unit},
	}}, p.Groups[1])
}

// Every directory of tests is a group, named by its path under data/.
func TestICPCGroups(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "problem.yaml", "name: Groups\nvalidation: custom\n")
	write(t, dir, "data/sample/1.in", "1\n")
	write(t, dir, "data/sample/1.ans", "1\n")
	write(t, dir, "data/secret/group1/a.in", "2\n")
	write(t, dir, "data/secret/group1/a.ans", "2\n")
	write(t, dir, "data/secret/group2/nested/c.in", "4\n")
	write(t, dir, "output_validators/checker/checker.cpp", "int main() {}\n")
	write(t, dir, "output_validators/checker/testlib.h", "\n")
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, summary{Type: "pass-fail", Checker: "checker.cpp", Library: []string{"testlib.h"},
		Groups: []string{"sample/each/1.00/1.00/0.00", "secret/group1/each/1.00/0.00/0.00", "secret/group2/nested/each/1.00/0.00/0.00"}}, summarize(p))
	require.Equal(t, "output_validators/checker/checker.cpp", p.Checker.Path)
	require.Empty(t, p.Groups[2].Tests[0].Answer)
}

// The ICPC grader aggregates group scores in ways Gate's per-test points
// cannot express, so a scoring package is rejected rather than rescored.
func TestICPCRejectsScoring(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "problem.yaml", "name: Scoring\ntype: scoring\n")
	_, err := Read(dir)
	require.ErrorContains(t, err, "ICPC scoring problems are not supported")
}

// A PCMS2 program includes the headers of its own folder, so different
// copies of testlib.h in separate folders do not clash.
func TestPCMS2ProgramHeaders(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "problem.xml", `<!DOCTYPE problem PUBLIC "-//PCMS2//DTD Problem Description 1.0//EN" "http://pcms2.ru/dtd/problem-0.2.dtd">
<problem id="headers"><judging><run><verifier type="testlib"><binary executable="check.exe"/></verifier></run>
  <testset name="tests"><time-limit>1000</time-limit><memory-limit>268435456</memory-limit></testset></judging></problem>`)
	write(t, dir, "check.cpp", "\n")
	write(t, dir, "testlib.h", "new\n")
	write(t, dir, "files/testlib.h", "old\n")
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"testlib.h": "testlib.h"}, p.Checker.Dependencies)
}

// Polygon resources are library files, except the statement templates.
func TestPolygonResources(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "problem.xml", `<problem short-name="resources">
  <judging><testset name="tests"><time-limit>1000</time-limit><memory-limit>268435456</memory-limit></testset></judging>
  <files><resources>
    <file path="files/olymp.sty"/><file path="files/problem.tex"/><file path="files/statements.ftl"/>
    <file path="files/testlib.h" type="h.g++"/><file path="files/helper.inc"/>
  </resources></files>
</problem>`)
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, []spec.File{{Name: "testlib.h", Path: "files/testlib.h"}, {Name: "helper.inc", Path: "files/helper.inc"}}, p.Library)
}

func TestICPCProgramHeaders(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "problem.yaml", "name: Headers\nvalidation: custom\n")
	write(t, dir, "output_validators/check/check.cpp", "\n")
	write(t, dir, "output_validators/check/helper.h", "A\n")
	write(t, dir, "input_validators/validate/validate.cpp", "\n")
	write(t, dir, "input_validators/validate/helper.h", "B\n")
	p, err := Read(dir)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"helper.h": "output_validators/check/helper.h"}, p.Checker.Dependencies)
	require.Equal(t, map[string]string{"helper.h": "input_validators/validate/helper.h"}, p.Validator.Dependencies)
}

// Readers see the package through an os.Root: a path in problem.xml cannot
// reach a file outside the package, through ../ or a symlink.
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
		require.Empty(t, p.Groups[0].Tests[0].Answer, pattern)
	}
}

// Gate runs one program per role; a package with more is rejected rather
// than imported with some of them silently dropped.
func TestICPCRejectsSeveralOutputValidators(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "problem.yaml", "name: Two\n")
	write(t, dir, "output_validators/a.cpp", "\n")
	write(t, dir, "output_validators/b.cpp", "\n")
	_, err := Read(dir)
	require.ErrorContains(t, err, "2 output validators")
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

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
