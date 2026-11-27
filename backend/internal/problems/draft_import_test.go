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
	"github.com/brawler2011/gate/backend/tests/dbfixtures"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/tools/txtar"
)

var importQuotas = model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 1000, Tests: 1000, Components: 100, Groups: 100}

func newImportUseCase(t *testing.T) *DraftUseCase {
	return &DraftUseCase{
		TempDir: t.TempDir(), UploadTTL: 1e10, DraftQuotas: importQuotas,
		BuildOptions: model.BuildOptions{
			Limits:    dbfixtures.BuildLimits(t),
			Languages: testCatalog.Langs,
		},
		Languages: testCatalog,
	}
}

// testCatalog has two configurations for .cpp; the extension suggests the first.
var testCatalog = &sandbox.Config{
	Order: []string{"c", "cpp17", "cpp20", "python3", "go", "java"},
	Langs: map[string]sandbox.Language{
		"c":       {Extensions: []string{"c"}},
		"cpp17":   {Extensions: []string{"cpp", "cc", "cxx"}},
		"cpp20":   {Extensions: []string{"cpp", "cc", "cxx"}},
		"python3": {Extensions: []string{"py"}},
		"go":      {Extensions: []string{"go"}},
		"java":    {Extensions: []string{"java"}},
	},
}

// Built-in templates must import into valid drafts for new problems.
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

// Author fixtures must import into buildable drafts with valid program and object references.
func TestAuthorPackagesImportIntoBuildableDrafts(t *testing.T) {
	packages, err := filepath.Glob("../../pkg/testdata/*/*")
	require.NoError(t, err)
	require.NotEmpty(t, packages)
	// PCMS2 lacks solution verdicts, so interactive imports need an explicit main solution.
	buildErrors := map[string]string{"pcms2/interactive-guess": "/main_solution"}
	languages := newImportUseCase(t).BuildOptions.Languages
	for _, dir := range packages {
		name := filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			plan, err := importDir(t, dir, nil)
			require.NoError(t, err)
			defer plan.Close()
			d := plan.draft
			require.NoError(t, d.Validate(importQuotas))
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
				require.NotEmpty(t, s.Output+s.Interaction, s.Language)
			}
			// testlib programs keep the library file they include.
			if (d.Checker != nil && d.Checker.Protocol == sandbox.ProtocolTestlib) || (d.Interactor != nil && d.Interactor.Protocol == sandbox.ProtocolTestlib) {
				require.NotNil(t, libraryFile(d, "testlib.h"))
			}
		})
	}
}

// Polygon CRLF tests must import with LF so testlib accepts them on Linux.
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

// Polygon checker and validator tests import with their files and expected verdicts.
func TestImportKeepsPolygonCheckerAndValidatorTests(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/polygon/a-plus-b", nil)
	require.NoError(t, err)
	defer plan.Close()
	files := map[uuid.UUID]string{}
	for _, o := range plan.objects {
		data, err := os.ReadFile(o.path)
		require.NoError(t, err)
		files[o.temporaryID] = string(data)
	}
	checker := []string{}
	for _, test := range plan.draft.Checker.Tests {
		checker = append(checker, test.Verdict+" "+strings.TrimSpace(files[test.Output]))
	}
	require.Equal(t, []string{"OK 3", "WA 4", "PE three"}, checker)
	validator := []string{}
	for _, test := range plan.draft.Validators[0].Tests {
		require.False(t, test.Valid)
		validator = append(validator, files[test.Input])
	}
	// Well-formed Polygon tests lose CRLF, as problem tests do.
	require.Equal(t, []string{"101 0\n", "1\n1\n"}, validator)
}

// Public archives strip checker tests and resources; a package without them imports, without its main solution it does not.
func TestImportSkipsMissingOptionalPolygonFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS("../../pkg/testdata/polygon/a-plus-b")))
	for _, name := range []string{"files/tests/checker-tests/02.o", "files/tests/validator-tests/01", "solutions/257073270_Spider47_ok.cpp"} {
		require.NoError(t, os.Remove(filepath.Join(dir, name)))
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "problem.xml"))
	require.NoError(t, err)
	jar := strings.Replace(string(manifest), "<resources>", `<resources><file path="files/testlib4j.jar" type="jar"/>`, 1)
	plan, err := importDir(t, dir, map[string]string{"problem.xml": jar})
	require.NoError(t, err)
	defer plan.Close()
	require.Len(t, plan.draft.Checker.Tests, 2)
	require.Len(t, plan.draft.Validators[0].Tests, 1)
	require.Len(t, plan.draft.Solutions, 5)
	require.NotNil(t, libraryFile(plan.draft, "testlib.h"))
	require.Nil(t, libraryFile(plan.draft, "testlib4j.jar"))
	require.ElementsMatch(t, []string{"files/testlib4j.jar", "files/tests/checker-tests/02.o", "files/tests/validator-tests/01", "solutions/257073270_Spider47_ok.cpp"}, plan.skipped)

	require.NoError(t, os.Remove(filepath.Join(dir, "solutions/solution_mm.cpp")))
	_, err = importDir(t, dir, nil)
	require.ErrorContains(t, err, "missing package file solutions/solution_mm.cpp")
}

// Known Polygon standard checkers use system references; unknown ones import their source.
func TestImportMapsPolygonStandardChecker(t *testing.T) {
	const pkgDir = "../../pkg/testdata/polygon/a-plus-b"
	plan, err := importDir(t, pkgDir, nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, model.DraftProgram{Language: "cpp17"}, plan.draft.Checker.DraftProgram)
	require.Equal(t, "hcmp", plan.draft.Checker.Standard)
	require.NoError(t, plan.draft.Validate(importQuotas))

	manifest, err := os.ReadFile(pkgDir + "/problem.xml")
	require.NoError(t, err)
	source, err := os.ReadFile(pkgDir + "/files/check.cpp")
	require.NoError(t, err)
	unknown := strings.Replace(string(manifest), `name="std::hcmp.cpp"`, `name="std::nosuch.cpp"`, 1)
	plan, err = importDir(t, pkgDir, map[string]string{"problem.xml": unknown})
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, model.DraftProgram{Language: "cpp17", Source: string(source)}, plan.draft.Checker.DraftProgram)
	require.Empty(t, plan.draft.Checker.Standard)
}

// Polygon tags and gfmt verdicts become expected verdicts; do-not-run and unlisted solutions do not run.
func TestImportKeepsExpectedVerdicts(t *testing.T) {
	const pkgDir = "../../pkg/testdata/polygon/a-plus-b"
	manifest, err := os.ReadFile(pkgDir + "/problem.xml")
	require.NoError(t, err)
	xml := string(manifest)
	for _, tag := range []string{"rejected", "time-limit-exceeded-or-accepted", "do-not-run", "failed"} {
		xml = strings.Replace(xml, `tag="accepted"`, `tag="`+tag+`"`, 1)
	}
	plan, err := importDir(t, pkgDir, map[string]string{"problem.xml": xml})
	require.NoError(t, err)
	defer plan.Close()
	verdicts := [][]string{}
	for _, s := range plan.draft.Solutions {
		verdicts = append(verdicts, s.Verdicts)
	}
	require.Equal(t, [][]string{{"WA", "PE", "TLE", "MLE", "OLE", "RE"}, {"TLE", "OK"}, {}, {"RE"}, {"OK"}, {"OK"}}, verdicts)

	plan, err = importDir(t, "../../pkg/testdata/gfmt/a-plus-b", map[string]string{"solutions/extra.py": "pass\n"})
	require.NoError(t, err)
	defer plan.Close()
	byName := map[string][]string{}
	for _, s := range plan.draft.Solutions {
		byName[s.Name+"."+s.Language] = s.Verdicts
	}
	require.Equal(t, []string{"CE"}, byName["ce.cpp17"])
	require.Equal(t, []string{"WA"}, byName["wa-2.python3"])
	require.Equal(t, []string{}, byName["extra.python3"])
	require.NoError(t, plan.draft.Validate(importQuotas))
}

func TestImportKeepsMainSolutionAcrossNameClash(t *testing.T) {
	plan, err := importDir(t, unpack(t, "polygon-solution-name-clash"), nil)
	require.NoError(t, err)
	defer plan.Close()
	main := plan.draft.Solution(*plan.draft.MainSolution)
	require.Equal(t, "main-2", main.Name)
	require.Equal(t, "int main() {}\n", main.Source)
	require.Equal(t, "main", plan.draft.Solutions[0].Name)
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
	require.Equal(t, []model.DraftLibraryFile{{Name: "testlib.h", Content: header}}, plan.draft.Library)
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

// Repeated source references must exhaust draft quotas before their copies accumulate.
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
	require.ErrorContains(t, err, "different files named helper.h: output_validators/check/helper.h, input_validators/validate/helper.h")
}

func libraryFile(d model.Draft, name string) *model.DraftLibraryFile {
	for i := range d.Library {
		if d.Library[i].Name == name {
			return &d.Library[i]
		}
	}
	return nil
}

// Upstream testlib copies become auto-updated references; modified copies retain their text.
func TestImportRecognizesUpstreamTestlib(t *testing.T) {
	const pkgDir = "../../pkg/testdata/polygon/subtasks-groups"
	plan, err := importDir(t, pkgDir, nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{Name: "testlib.h", AutoUpdate: true}}, plan.draft.Library)
	require.NoError(t, plan.draft.Validate(importQuotas))

	original, err := os.ReadFile(pkgDir + "/files/testlib.h")
	require.NoError(t, err)
	patched := string(original) + "// patched\n"
	plan, err = importDir(t, pkgDir, map[string]string{"files/testlib.h": patched})
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{Name: "testlib.h", Content: patched}}, plan.draft.Library)
}

// Unused upstream testlib variants must merge into one auto-updated library file.
func TestImportMergesUpstreamTestlibVersions(t *testing.T) {
	old, err := os.ReadFile("../../pkg/testdata/polygon/max-subarray/files/testlib.h")
	require.NoError(t, err)
	system, _ := syslib.Lookup("testlib.h")
	require.NotEqual(t, system.Content, string(old))
	files := map[string]string{
		"problem.yaml": "name: Versions\nvalidation: custom\n", "data/secret/1.in": "1\n", "data/secret/1.ans": "1\n",
		"output_validators/check/check.cpp": "int main() {}\n", "output_validators/check/testlib.h": string(old),
		"input_validators/validate/validate.cpp": "int main() {}\n", "input_validators/validate/testlib.h": system.Content,
	}
	plan, err := importDir(t, t.TempDir(), files)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{Name: "testlib.h", AutoUpdate: true}}, plan.draft.Library)

	files["input_validators/validate/testlib.h"] = system.Content + "// patched\n"
	_, err = importDir(t, t.TempDir(), files)
	require.ErrorContains(t, err, "different files named testlib.h")
}

// An old upstream testlib shared with a generator still auto-updates: pre-2023 copies do not compile with GCC 13+.
func TestImportUpdatesGeneratorTestlib(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/polygon/max-subarray", nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{Name: "testlib.h", AutoUpdate: true}}, plan.draft.Library)
}

// Templates and GFMT packages must reference system testlib without copying it.
func TestImportSystemLibrary(t *testing.T) {
	plan, err := importAPlusB(t, nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, []model.DraftLibraryFile{{Name: "testlib.h", AutoUpdate: true}}, plan.draft.Library)

	_, err = importAPlusB(t, map[string]string{"lib/testlib.h": "// copy\n"})
	require.ErrorContains(t, err, "system library file")
}

// Streaming conversion must handle split CRLF pairs and preserve lone CR bytes.
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

func importAPlusB(t *testing.T, extra map[string]string) (*draftImport, error) {
	return importDir(t, "../../pkg/testdata/gfmt/a-plus-b", extra)
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
}

func TestImportKeepsSeveralValidators(t *testing.T) {
	read := func(path string) string {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
	validators := func(plan *draftImport, err error) map[string][]string {
		require.NoError(t, err)
		defer plan.Close()
		require.NoError(t, plan.draft.Validate(importQuotas))
		result := map[string][]string{}
		for _, v := range plan.draft.Validators {
			result[v.Name] = []string{}
			for _, id := range v.Groups {
				result[v.Name] = append(result[v.Name], plan.draft.Group(id).Name)
			}
		}
		return result
	}

	gfmt := read("../../pkg/testdata/gfmt/a-plus-b/problem.yaml") + "\nsources:\n  validators/secret.py:\n    groups: [secret]\n"
	require.Equal(t, map[string][]string{"validator": {}, "secret": {"secret"}},
		validators(importAPlusB(t, map[string]string{"problem.yaml": gfmt, "validators/secret.py": "\n"})))
	_, err := importAPlusB(t, map[string]string{"problem.yaml": strings.Replace(gfmt, "[secret]", "[hidden]", 1), "validators/secret.py": "\n"})
	require.ErrorContains(t, err, "validator validators/secret.py checks unknown test group hidden")

	const polygonDir = "../../pkg/testdata/polygon/a-plus-b"
	xml := strings.Replace(read(polygonDir+"/problem.xml"), "</validators>", `<validator><source path="files/v2.cpp" type="cpp.g++17"/></validator></validators>`, 1)
	require.Equal(t, map[string][]string{"v": {}, "v2": {}},
		validators(importDir(t, polygonDir, map[string]string{"problem.xml": xml, "files/v2.cpp": "int main() {}\n"})))

	require.Equal(t, map[string][]string{"v": {}, "v2": {}},
		validators(importDir(t, "../../pkg/testdata/icpc/a-plus-b", map[string]string{"input_validators/v2.cpp": "int main() {}\n"})))
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

// Statement examples differing from the sample files and tutorials survive the import and the export.
func TestImportKeepsStatementExamplesAndTutorials(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/polygon/interactive-guess", map[string]string{
		"statement-sections/english/example.01":   "? 5\r\n",
		"statement-sections/english/example.01.a": "< 50\n",
	})
	require.NoError(t, err)
	defer plan.Close()
	sample := plan.draft.Tests[0]
	require.True(t, sample.Sample, "an interactive sample stays a sample")
	require.Equal(t, "? 5\n", *sample.StatementInput)
	require.Equal(t, "< 50\n", *sample.StatementOutput)
	require.NotEmpty(t, plan.draft.Statement("en").Tutorial)
	require.Equal(t, normalized(plan), normalized(exportAndImport(t, plan)))

	plan, err = importDir(t, "../../pkg/testdata/pcms2/a-plus-b", map[string]string{"statements/english/example.02.a": "another answer\n"})
	require.NoError(t, err)
	defer plan.Close()
	require.Nil(t, plan.draft.Tests[1].StatementInput, "the example shows the test input")
	require.Equal(t, "another answer\n", *plan.draft.Tests[1].StatementOutput)
	require.Nil(t, plan.draft.Tests[0].StatementOutput)
}

func TestImportLinksPCMS2StatementPDF(t *testing.T) {
	plan, err := importDir(t, unpack(t, "pcms2-pdf-statement"), nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, `\includegraphics{statement-en.pdf}`, plan.draft.Statement("en").Legend)
	require.Equal(t, "statement-en.pdf", plan.draft.Assets[0].Name)
	require.NoError(t, plan.draft.Validate(importQuotas))
}

func TestImportKeepsStatementTitles(t *testing.T) {
	xml, err := os.ReadFile("../../pkg/testdata/polygon/a-plus-b/problem.xml")
	require.NoError(t, err)
	renamed := strings.Replace(string(xml), `<name language="russian" value="A+B"/>`, `<name language="russian" value="Сумма"/>`, 1)
	require.NotEqual(t, string(xml), renamed)
	plan, err := importDir(t, "../../pkg/testdata/polygon/a-plus-b", map[string]string{"problem.xml": renamed})
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, "A+B", plan.draft.Title)
	require.Empty(t, plan.draft.Statement("en").Title)
	require.Equal(t, "Сумма", plan.draft.Statement("ru").Title)
	require.Equal(t, normalized(plan), normalized(exportAndImport(t, plan)))
}

// Old Russian packages use windows-1251: Polygon declares it, PCMS2 sources leave it implicit.
func TestImportConvertsWindows1251Text(t *testing.T) {
	cp1251 := charmap.Windows1251.NewEncoder()
	encode := func(s string) string {
		out, err := cp1251.String(s)
		require.NoError(t, err)
		return out
	}
	read := func(path string) string {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
	const polygonDir = "../../pkg/testdata/polygon/a-plus-b"
	xml := strings.Replace(read(polygonDir+"/problem.xml"), `charset="UTF-8" language="russian"`, `charset="windows-1251" language="russian"`, 1)
	input := "7 8 " + encode("Привет") + "\n"
	files := map[string]string{"problem.xml": xml, "tests/01": input}
	var sections []string
	for _, name := range []string{"legend", "input", "output", "notes"} {
		path := "statement-sections/russian/" + name + ".tex"
		files[path] = encode(read(polygonDir + "/" + path))
		sections = append(sections, path)
	}
	plan, err := importDir(t, polygonDir, files)
	require.NoError(t, err)
	defer plan.Close()
	require.Contains(t, plan.draft.Statement("ru").Legend, "Вам заданы два целых числа")
	require.ElementsMatch(t, sections, plan.converted)
	data, err := os.ReadFile(objectPath(t, plan, *plan.draft.Tests[0].Input.ObjectID))
	require.NoError(t, err)
	require.Equal(t, input, string(data))

	const pcms2Dir = "../../pkg/testdata/pcms2/a-plus-b"
	problem := strings.Replace(read(pcms2Dir+"/problem.xml"), `encoding="utf-8"`, `encoding="windows-1251"`, 1)
	checker := "// Сравнение чисел\n" + read(pcms2Dir+"/check.cpp")
	plan, err = importDir(t, pcms2Dir, map[string]string{"problem.xml": encode(problem), "check.cpp": encode(checker)})
	require.NoError(t, err)
	defer plan.Close()
	require.Equal(t, checker, plan.draft.Checker.Source)
	require.Equal(t, []string{"check.cpp"}, plan.converted)
	require.NoError(t, plan.draft.ValidateBuild(importQuotas, newImportUseCase(t).BuildOptions.Languages))
}

func objectPath(t *testing.T, plan *draftImport, id uuid.UUID) string {
	t.Helper()
	for _, o := range plan.objects {
		if o.temporaryID == id {
			return o.path
		}
	}
	t.Fatal("object not found")
	return ""
}

func TestImportRejectsUnknownStatementSection(t *testing.T) {
	_, err := importAPlusB(t, map[string]string{"statements/en/story.tex": "x\n"})
	require.ErrorContains(t, err, "unknown statement section story in en")
}

// The client sees only the outermost pkg.Error, so the defect must be its public message.
func TestImportShowsWhyAPackageIsInvalid(t *testing.T) {
	_, err := importAPlusB(t, map[string]string{"junk.txt": "x\n"})
	var e *pkg.Error
	require.ErrorAs(t, err, &e)
	require.Equal(t, pkg.ErrPackageLayout.Code, e.Code)
	require.Contains(t, e.Message, "unexpected file junk.txt")
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
