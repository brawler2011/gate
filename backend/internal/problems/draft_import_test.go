package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/sandbox"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var importQuotas = model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 1000, Tests: 1000, Components: 100, Subtasks: 100}

func newImportUseCase(t *testing.T) *DraftUseCase {
	return &DraftUseCase{
		TempDir: t.TempDir(), UploadTTL: 1e10, ObjectBytes: 1 << 30, DraftQuotas: importQuotas,
		BuildOptions: model.BuildOptions{
			Limits: model.BuildLimits{DiskBytes: 1 << 30, FileCount: 1000},
			Toolchain: model.Toolchain{Languages: map[string]sandbox.Language{
				"cpp": {}, "python": {}, "go": {}, "java": {},
			}},
		},
	}
}

// Every built-in template must import into a draft that satisfies the draft
// invariants, because new problems are created from them.
func TestBuiltinTemplatesImportIntoValidDrafts(t *testing.T) {
	uc := newImportUseCase(t)
	for _, template := range templates.ListBuiltinTemplates() {
		id := template.ID
		t.Run(id, func(t *testing.T) {
			data, err := templates.GetBuiltinTemplateZip(id)
			require.NoError(t, err)
			problem := uuid.New()
			plan, err := uc.parseImport(context.Background(), bytes.NewReader(data), problem, 1)
			require.NoError(t, err)
			defer plan.Close()
			plan.draft.ProblemID, plan.draft.Revision = problem, 1
			require.NoError(t, plan.draft.Validate(importQuotas, true))
			require.NotEmpty(t, plan.draft.Tests)
			require.NotEmpty(t, plan.draft.Components)
		})
	}
}

// Every author package in pkg/testdata must import into a draft that build
// validation accepts: programs sit in their role folders with the library
// attached, and tests reference existing generators and objects.
func TestAuthorPackagesImportIntoBuildableDrafts(t *testing.T) {
	packages, err := filepath.Glob("../../pkg/testdata/*/*")
	require.NoError(t, err)
	require.NotEmpty(t, packages)
	// PCMS2 records no solution verdicts, so the author picks the main
	// solution an interactive problem needs in the workshop.
	buildErrors := map[string]string{"pcms2/interactive-guess": "solution is required"}
	for _, dir := range packages {
		name := filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			plan, err := importDir(t, dir, nil)
			require.NoError(t, err)
			defer plan.Close()
			d := plan.draft
			require.NoError(t, d.Validate(importQuotas, false))
			if want, ok := buildErrors[name]; ok {
				require.EqualError(t, d.Validate(importQuotas, true), want)
				return
			}
			require.NoError(t, d.Validate(importQuotas, true))
			// Every testdata checker, validator, interactor and generator uses testlib.
			for _, c := range d.Components {
				if c.Role != "solution" {
					require.Contains(t, c.Dependencies, "testlib.h", c.Path)
				}
			}
		})
	}
}

// Polygon packages made on Windows keep well-formed tests with CRLF line
// endings, which testlib rejects on Linux; the importer stores them with LF.
func TestImportConvertsWellFormedPolygonTestsToLF(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/polygon/a-plus-b", nil)
	require.NoError(t, err)
	defer plan.Close()
	source, err := os.ReadFile("../../pkg/testdata/polygon/a-plus-b/tests/01")
	require.NoError(t, err)
	require.Contains(t, string(source), "\r\n")
	input := plan.draft.Tests[0].Input.ObjectID
	for _, o := range plan.objects {
		if o.temporaryID == *input {
			data, err := os.ReadFile(o.path)
			require.NoError(t, err)
			require.Equal(t, strings.ReplaceAll(string(source), "\r\n", "\n"), string(data))
			return
		}
	}
	t.Fatal("input object not found")
}

// Programs are identified by package path: when two solutions share a file
// name, the second is renamed and the tagged main solution stays main.
func TestImportKeepsMainSolutionAcrossNameClash(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"wrong/main.cpp": "int main() { return 1; }\n", "correct/main.cpp": "int main() {}\n", "tests/01": "1\n",
		"problem.xml": `<problem short-name="clash"><judging><testset name="tests">
  <time-limit>1000</time-limit><memory-limit>268435456</memory-limit>
  <input-path-pattern>tests/%02d</input-path-pattern><answer-path-pattern>tests/%02d.a</answer-path-pattern>
  <tests><test method="manual"/></tests></testset></judging>
  <assets><solutions>
    <solution tag="wrong_answer"><source path="wrong/main.cpp"/></solution>
    <solution tag="main"><source path="correct/main.cpp"/></solution>
  </solutions></assets></problem>`,
	}
	plan, err := importDir(t, dir, files)
	require.NoError(t, err)
	defer plan.Close()
	var main model.DraftComponent
	for _, c := range plan.draft.Components {
		if c.ID == *plan.draft.MainSolutionID {
			main = c
		}
	}
	require.Equal(t, "solutions/main-2.cpp", main.Path)
	require.Equal(t, "int main() {}\n", plan.draft.Files[main.Path])
}

// Draft quotas apply while the draft is built: a manifest naming one source
// as many solutions is rejected before its copies add up.
func TestImportEnforcesQuotasWhileConverting(t *testing.T) {
	solutions := ""
	for range importQuotas.Components + 1 {
		solutions += `<solution tag="accepted"><source path="main.cpp"/></solution>`
	}
	_, err := importDir(t, t.TempDir(), map[string]string{
		"main.cpp": "int main() {}\n", "tests/01": "1\n",
		"problem.xml": `<problem short-name="many"><judging><testset name="tests">
  <time-limit>1000</time-limit><memory-limit>268435456</memory-limit>
  <input-path-pattern>tests/%02d</input-path-pattern><answer-path-pattern>tests/%02d.a</answer-path-pattern>
  <tests><test method="manual"/></tests></testset></judging>
  <assets><solutions>` + solutions + `</solutions></assets></problem>`,
	})
	require.ErrorIs(t, err, pkg.ErrPayloadTooLarge)
}

// Different headers of one name stay apart, each attached to its program.
func TestImportKeepsProgramHeadersApart(t *testing.T) {
	plan, err := importDir(t, t.TempDir(), map[string]string{
		"problem.yaml":                           "name: Headers\nvalidation: custom\n",
		"data/secret/1.in":                       "1\n",
		"data/secret/1.ans":                      "1\n",
		"output_validators/check/check.cpp":      "int main() {}\n",
		"output_validators/check/helper.h":       "// A\n",
		"input_validators/validate/validate.cpp": "int main() {}\n",
		"input_validators/validate/helper.h":     "// B\n",
	})
	require.NoError(t, err)
	defer plan.Close()
	require.NoError(t, plan.draft.Validate(importQuotas, true))
	require.Equal(t, map[string]string{"helper.h": "lib/helper.h"}, plan.draft.Component("checker").Dependencies)
	require.Equal(t, map[string]string{"helper.h": "lib/helper-2.h"}, plan.draft.Component("validator").Dependencies)
	require.Equal(t, "// B\n", plan.draft.Files["lib/helper-2.h"])
}

// importAPlusB imports the gfmt a-plus-b package with extra files added.
func importAPlusB(t *testing.T, extra map[string]string) (*draftImport, error) {
	return importDir(t, "../../pkg/testdata/gfmt/a-plus-b", extra)
}

// importDir imports the package in source with extra files added.
func importDir(t *testing.T, source string, extra map[string]string) (*draftImport, error) {
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(source)))
	for path, text := range extra {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, path), []byte(text), 0o600))
	}
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	require.NoError(t, w.AddFS(os.DirFS(dir)))
	require.NoError(t, w.Close())
	return newImportUseCase(t).parseImport(context.Background(), &archive, uuid.New(), 1)
}

// Tests name generators by file stem, so echo.cpp and echo.py together are ambiguous.
func TestImportRejectsGeneratorsWithOneStem(t *testing.T) {
	_, err := importAPlusB(t, map[string]string{"generators/echo.py": "print(0, 0)\n"})
	require.ErrorContains(t, err, "several generators named echo")
}

func TestImportKeepsDraftLayout(t *testing.T) {
	plan, err := importAPlusB(t, map[string]string{"assets/pic.png": "png"})
	require.NoError(t, err)
	defer plan.Close()
	require.Len(t, plan.draft.Assets, 1)
	require.Equal(t, "assets/pic.png", plan.draft.Assets[0].Path)

	_, err = importAPlusB(t, map[string]string{"pic.png": "png"})
	require.ErrorContains(t, err, "unexpected file pic.png")
}

func TestImportRejectsSecondSingletonComponent(t *testing.T) {
	_, err := importAPlusB(t, map[string]string{"checkers/other.cpp": "int main() {}\n"})
	require.ErrorContains(t, err, "more than one checker in checkers/")
	_, err = importAPlusB(t, map[string]string{"validators/other.py": "\n"})
	require.ErrorContains(t, err, "more than one validator in validators/")
}

func TestImportRejectsUnknownSourceExtension(t *testing.T) {
	_, err := importAPlusB(t, map[string]string{"solutions/main.rs": "fn main() {}\n"})
	require.ErrorContains(t, err, "unsupported source language: solutions/main.rs")
}
