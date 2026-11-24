package formats

import (
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
		"gfmt/a-plus-b": {Type: "pass-fail", Checker: "checker.cpp", Validators: []string{"validator.cpp"}, Main: "solutions/ok.cpp", Generators: []string{"echo.cpp"}, Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"samples/complete/3.00/3.00/0.00", "secret/each/6.00/0.00/0.00"}},
		"gfmt/interactive-guess": {Type: "interactive", Checker: "checker.cpp", Interactor: "interactor.cpp", Main: "solutions/ok.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"samples/complete/1.00/1.00/0.00", "secret/each/1.00/0.00/0.00"}},
		"polygon/a-plus-b": {Type: "pass-fail", Checker: "check.cpp", Validators: []string{"v.cpp"}, Main: "solutions/solution_mm.cpp", Generators: []string{"echo.cpp"}, Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"/each/12.00/3.00/0.00"}},
		"polygon/interactive-guess": {Type: "interactive", Checker: "check.cpp", Validators: []string{"validator.cpp"}, Interactor: "interactor.cpp", Main: "solutions/accepted.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/each/2.00/0.00/0.00"}},
		"polygon/max-subarray": {Type: "pass-fail", Checker: "check.cpp", Validators: []string{"validator.cpp"}, Main: "solutions/accepted.cpp", Generators: []string{"generator.cpp"}, Library: testlib, Statements: []string{"en"},
			Groups: []string{"/each/4.00/2.00/0.00"}},
		"polygon/subtasks-groups": {Type: "scoring", Checker: "check.cpp", Validators: []string{"validator.cpp"}, Main: "solutions/accepted.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"1/complete/1.00/1.00/40.00", "2/complete/1.00/0.00/60.00"}},
		"icpc/a-plus-b": {Type: "pass-fail", Checker: "check.cpp", Validators: []string{"v.cpp"}, Main: "submissions/accepted/accepted.cpp", Generators: []string{"echo.cpp"}, Library: icpcLib, Statements: []string{"en", "ru"},
			Groups: []string{"sample/each/3.00/3.00/0.00", "secret/each/9.00/0.00/0.00"}},
		// An interactive input is what the interactor reads, so none is published.
		"icpc/interactive-guess": {Type: "interactive", Validators: []string{"validator.cpp"}, Interactor: "interactor.cpp", Main: "submissions/accepted/accepted.cpp", Library: []string{"testlib.h", "testlib.h"}, Statements: []string{"en"},
			Groups: []string{"sample/each/1.00/0.00/0.00", "secret/each/1.00/0.00/0.00"}},
		"pcms2/a-plus-b": {Type: "pass-fail", Checker: "check.cpp", Library: testlib, Statements: []string{"en", "ru"},
			Groups: []string{"/each/12.00/3.00/0.00"}},
		"pcms2/interactive-guess": {Type: "interactive", Checker: "check.cpp", Interactor: "interactor.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/each/2.00/0.00/0.00"}},
		"pcms2/max-subarray": {Type: "pass-fail", Checker: "check.cpp", Library: testlib, Statements: []string{"en"},
			Groups: []string{"/each/4.00/2.00/0.00"}},
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
	p, err := Read(unpack(t, "polygon-groups"))
	require.NoError(t, err)
	require.Equal(t, "scoring", p.Type)
	require.Equal(t, [2]string{"input.txt", "output.txt"}, [2]string{p.InputFile, p.OutputFile})
	require.Equal(t, "gen.cpp", p.Generators[0].Path)
	require.Len(t, p.Groups, 2)
	require.Equal(t, spec.Group{Name: "samples", Policy: "each", Feedback: "complete", Tests: []spec.Test{{Input: "tests/01", Sample: true}}}, p.Groups[0])
	require.Equal(t, spec.Group{Name: "main", Policy: "complete", Feedback: "points", DependsOn: []string{"samples"}, Tests: []spec.Test{
		{Generator: "gen.cpp", Args: []string{"a b", "2"}},
		{Generator: "gen.cpp", Args: []string{"3"}, Points: 100 * points.Unit},
	}}, p.Groups[1])
}

func TestPolygonResources(t *testing.T) {
	p, err := Read(unpack(t, "polygon-resources"))
	require.NoError(t, err)
	require.Equal(t, []spec.File{{Name: "testlib.h", Path: "files/testlib.h"}, {Name: "helper.inc", Path: "files/helper.inc"}}, p.Library)
}

func TestICPCGroups(t *testing.T) {
	p, err := Read(unpack(t, "icpc-groups"))
	require.NoError(t, err)
	require.Equal(t, summary{Type: "pass-fail", Checker: "checker.cpp", Library: []string{"testlib.h"},
		Groups: []string{"sample/each/1.00/1.00/0.00", "secret/group1/each/1.00/0.00/0.00", "secret/group2/nested/each/1.00/0.00/0.00"}}, summarize(p))
	require.Equal(t, "output_validators/checker/checker.cpp", p.Checker.Path)
	require.Empty(t, p.Groups[2].Tests[0].Answer)
}

func TestICPCRejectsScoring(t *testing.T) {
	_, err := Read(unpack(t, "icpc-scoring"))
	require.ErrorContains(t, err, "ICPC scoring problems are not supported")
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
		require.Empty(t, p.Groups[0].Tests[0].Answer, pattern)
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
	require.False(t, p.Groups[0].Tests[0].Sample, "an oversized statement is not inspected")
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
