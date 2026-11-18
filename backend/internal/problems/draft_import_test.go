package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/brawler2011/gate/backend/pkg/sandbox"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

var importQuotas = model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 1000, Tests: 1000, Components: 100, Subtasks: 100}

func newImportUseCase(t *testing.T) *DraftUseCase {
	return &DraftUseCase{
		TempDir: t.TempDir(), UploadTTL: 1e10, ObjectBytes: 1 << 30, DraftQuotas: importQuotas,
		BuildOptions: model.BuildOptions{
			Limits:    model.BuildLimits{DiskBytes: 1 << 30, FileCount: 1000},
			Languages: testCatalog.Langs,
		},
		Languages: testCatalog,
	}
}

// testCatalog has two configurations for .cpp; the extension suggests the first.
var testCatalog = &sandbox.Config{
	Order: []string{"cpp17", "cpp20", "python3", "go", "java"},
	Langs: map[string]sandbox.Language{
		"cpp17":   {Extensions: []string{"cpp", "cc", "cxx"}},
		"cpp20":   {Extensions: []string{"cpp", "cc", "cxx"}},
		"python3": {Extensions: []string{"py"}},
		"go":      {Extensions: []string{"go"}},
		"java":    {Extensions: []string{"java"}},
	},
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
			require.NoError(t, plan.draft.ValidateBuild(importQuotas, uc.BuildOptions.Languages))
			require.NotEmpty(t, plan.draft.Tests)
			require.NotEmpty(t, plan.draft.Solutions)
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
	buildErrors := map[string]string{"pcms2/interactive-guess": "/main_solution_id"}
	languages := newImportUseCase(t).BuildOptions.Languages
	for _, dir := range packages {
		name := filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			plan, err := importDir(t, dir, nil)
			require.NoError(t, err)
			defer plan.Close()
			d := plan.draft
			require.NoError(t, d.Validate(importQuotas, nil))
			if want, ok := buildErrors[name]; ok {
				var fields pkg.FieldErrors
				require.ErrorAs(t, d.ValidateBuild(importQuotas, languages), &fields)
				require.Equal(t, want, fields[0].Pointer)
				return
			}
			require.NoError(t, d.ValidateBuild(importQuotas, languages))
			require.NotEmpty(t, d.Statements)
			for _, s := range d.Statements {
				require.NotEmpty(t, s.Legend, s.Language)
				require.NotEmpty(t, s.Output, s.Language)
			}
			// Every testdata checker, validator, interactor and generator uses testlib.
			require.NotNil(t, libraryFile(d, "testlib.h"))
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

func TestImportKeepsMainSolutionAcrossNameClash(t *testing.T) {
	plan, err := importDir(t, unpack(t, "polygon-solution-name-clash"), nil)
	require.NoError(t, err)
	defer plan.Close()
	main := plan.draft.Solution(*plan.draft.MainSolutionID)
	require.Equal(t, "main", main.Name)
	require.Equal(t, "int main() {}\n", main.Source)
}

func TestImportRejectsPathsOutsidePackage(t *testing.T) {
	_, err := importDir(t, unpack(t, "polygon-path-outside"), nil)
	require.ErrorContains(t, err, "invalid package path ../outside.tex")
}

// Identical copies of a library file count once against the quotas.
func TestImportChargesSharedLibraryOnce(t *testing.T) {
	header := strings.Repeat("// testlib\n", 20000)
	files := map[string]string{"problem.yaml": "name: Shared\nvalidation: custom\n", "data/secret/1.in": "1\n", "data/secret/1.ans": "1\n"}
	copies := int(importQuotas.TextBytes)/len(header) + 1
	for i := range copies {
		files[fmt.Sprintf("generators/g%d/g.cpp", i)] = "int main() {}\n"
		files[fmt.Sprintf("generators/g%d/testlib.h", i)] = header
	}
	files["output_validators/check.cpp"] = "int main() {}\n"
	plan, err := importDir(t, t.TempDir(), files)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{ID: plan.draft.Library[0].ID, Name: "testlib.h", Content: header}}, plan.draft.Library)
}

// Many statements naming one large file are charged as they are converted.
func TestImportChargesStatements(t *testing.T) {
	statements := ""
	for i := range 20 {
		statements += fmt.Sprintf(`<statement language="%c%c" path="statement.tex" type="application/x-tex"/>`, 'a'+i/26, 'a'+i%26)
	}
	_, err := importDir(t, t.TempDir(), map[string]string{
		"statement.tex": "\\begin{problem}{x}{}{}{}{}" + strings.Repeat("text ", int(importQuotas.FileBytes)/5-10) + "\\end{problem}\n",
		"problem.xml":   `<problem short-name="many"><statements>` + statements + `</statements><judging><testset name="tests"><time-limit>1000</time-limit><memory-limit>268435456</memory-limit></testset></judging></problem>`,
	})
	require.ErrorIs(t, err, pkg.ErrPayloadTooLarge)
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

// A draft has one library file of each name, and judge programs share it.
func TestImportRejectsDifferentLibraryFilesOfOneName(t *testing.T) {
	_, err := importDir(t, unpack(t, "icpc-program-headers"), nil)
	require.ErrorContains(t, err, "different files named helper.h")
}

// libraryFile returns the library file of d named name, or nil.
func libraryFile(d model.Draft, name string) *model.DraftLibraryFile {
	for i := range d.Library {
		if d.Library[i].Name == name {
			return &d.Library[i]
		}
	}
	return nil
}

// A package testlib.h equal to an upstream version becomes the auto-updated
// one; a modified copy stays the author's.
func TestImportRecognizesUpstreamTestlib(t *testing.T) {
	const pkgDir = "../../pkg/testdata/polygon/subtasks-groups"
	plan, err := importDir(t, pkgDir, nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{ID: plan.draft.Library[0].ID, Name: "testlib.h", AutoUpdate: true}}, plan.draft.Library)
	require.NoError(t, plan.draft.Validate(importQuotas, nil))

	original, err := os.ReadFile(pkgDir + "/files/testlib.h")
	require.NoError(t, err)
	patched := string(original) + "// patched\n"
	plan, err = importDir(t, pkgDir, map[string]string{"files/testlib.h": patched})
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{ID: plan.draft.Library[0].ID, Name: "testlib.h", Content: patched}}, plan.draft.Library)
}

// rnd differs between testlib versions, so a package with generators keeps
// its testlib.h unless it is the current system one; turning auto-update on
// is the author's choice.
func TestImportKeepsGeneratorTestlib(t *testing.T) {
	const pkgDir = "../../pkg/testdata/polygon/max-subarray"
	plan, err := importDir(t, pkgDir, nil)
	require.NoError(t, err)
	defer plan.Close()
	original, err := os.ReadFile(pkgDir + "/files/testlib.h")
	require.NoError(t, err)
	require.Equal(t, []model.DraftLibraryFile{{ID: plan.draft.Library[0].ID, Name: "testlib.h", Content: string(original)}}, plan.draft.Library)

	system, _ := syslib.Lookup("testlib.h")
	plan, err = importDir(t, pkgDir, map[string]string{"files/testlib.h": system.Content})
	require.NoError(t, err)
	defer plan.Close()
	require.True(t, plan.draft.Library[0].AutoUpdate)
}

// Builtin templates and gfmt packages name the system testlib.h instead of
// carrying a copy.
func TestImportSystemLibrary(t *testing.T) {
	plan, err := importAPlusB(t, nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{ID: plan.draft.Library[0].ID, Name: "testlib.h", AutoUpdate: true}}, plan.draft.Library)

	_, err = importAPlusB(t, map[string]string{"lib/testlib.h": "// copy\n"})
	require.ErrorContains(t, err, "system library file")
}

// CRLF conversion streams: a CRLF split across reads is still converted, and
// a lone CR is kept.
func TestCopyLF(t *testing.T) {
	for _, tc := range []struct {
		in, out   string
		converted bool
	}{
		{"a\r\nb\rc\r\n\r", "a\nb\rc\n\r", true},
		{"a\nb\n", "a\nb\n", false},
		{"\r", "\r", false},
	} {
		var out bytes.Buffer
		converted, err := copyLF(&out, iotest.OneByteReader(strings.NewReader(tc.in)))
		require.NoError(t, err)
		require.Equal(t, tc.out, out.String(), tc.in)
		require.Equal(t, tc.converted, converted, tc.in)
	}
}

// importAPlusB imports the gfmt a-plus-b package with extra files added.
func importAPlusB(t *testing.T, extra map[string]string) (*draftImport, error) {
	return importDir(t, "../../pkg/testdata/gfmt/a-plus-b", extra)
}

// unpack writes the package of testdata/<name>.txtar into a temporary
// directory. The comment of each archive describes its case.
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
	require.Equal(t, "pic.png", plan.draft.Assets[0].Name)

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
	require.ErrorContains(t, err, "no language configuration accepts solutions/main.rs")
}

// Statement sections come from olymp.sty commands in a Polygon statement.
func TestImportSplitsPolygonStatementIntoSections(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/polygon/a-plus-b", nil)
	require.NoError(t, err)
	defer plan.Close()
	s := plan.draft.Statement("en")
	require.NotNil(t, s)
	require.Contains(t, s.Legend, "$a$")
	for name, section := range map[string]string{"input": s.Input, "output": s.Output, "notes": s.Notes} {
		require.NotEmpty(t, section, name)
		require.NotContains(t, section, "file", name)
	}
}

func TestImportRejectsUnknownStatementSection(t *testing.T) {
	_, err := importAPlusB(t, map[string]string{"statements/en/story.md": "x\n"})
	require.ErrorContains(t, err, "unknown statement section story in en")
}

func TestImportKeepsPolygonSourceTypes(t *testing.T) {
	plan, err := importDir(t, unpack(t, "polygon-source-types"), nil)
	require.NoError(t, err)
	defer plan.Close()
	languages := map[string]string{}
	for _, s := range plan.draft.Solutions {
		languages[s.Name] = s.Language
	}
	require.Equal(t, map[string]string{"main": "cpp20", "old": "cpp17", "plain": "cpp17", "script": "python3"}, languages)
}

func TestImportRejectsUnsupportedPolygonSourceTypes(t *testing.T) {
	_, err := importDir(t, unpack(t, "polygon-unsupported-standard"), nil)
	require.ErrorContains(t, err, "unsupported language configuration cpp14 of solutions/main.cpp")
	_, err = importDir(t, unpack(t, "polygon-unsupported-type"), nil)
	require.ErrorContains(t, err, `solutions/main.pas: unsupported Polygon source type "pascal.fpc"`)
}
