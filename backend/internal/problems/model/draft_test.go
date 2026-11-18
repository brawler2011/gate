package problemsmodel

import (
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var testQuotas = DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 10, Tests: 10, Components: 10, Subtasks: 10}

func validDraft() Draft {
	gen, sol, sub := "gen", "main", uuid.New()
	return Draft{ProblemID: uuid.New(), Revision: 1, Title: "A", ShortName: "a", Type: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256,
		DraftContent: DraftContent{
			Checker:      &DraftChecker{DraftProgram: DraftProgram{Language: "cpp"}},
			Generators:   []DraftNamedProgram{{Name: gen, DraftProgram: DraftProgram{Language: "python"}}},
			Solutions:    []DraftNamedProgram{{Name: sol, DraftProgram: DraftProgram{Language: "cpp"}}},
			MainSolution: &sol,
			Library:      []DraftLibraryFile{{Name: "testlib.h"}},
			Statements:   []DraftStatement{{Language: "en", Legend: "![graph](graph.png)"}},
			Assets:       []DraftAsset{{Name: "graph.png", ObjectID: uuid.New()}},
			Subtasks:     []DraftSubtask{{ID: sub, Ordinal: 1, Name: "main", Policy: "complete"}},
			Tests: []DraftTest{{ID: uuid.New(), Ordinal: 1, SubtaskID: sub,
				Input:  DraftInput{Mode: "generator", Generator: &gen},
				Answer: DraftAnswer{Mode: "generated", Solution: &sol}}},
		}}
}

func fieldErrors(t *testing.T, err error) pkg.FieldErrors {
	t.Helper()
	var fields pkg.FieldErrors
	require.ErrorAs(t, err, &fields)
	require.ErrorIs(t, err, pkg.ErrBadInput)
	return fields
}

// Each violation names the entry the workshop has to show it at.
func TestValidateLocatesViolations(t *testing.T) {
	require.NoError(t, func() error { d := validDraft(); return d.Validate(testQuotas, nil) }())
	for name, tc := range map[string]struct {
		change  func(*Draft)
		pointer string
		message string
	}{
		"test input from a solution": {func(d *Draft) { d.Tests[0].Input.Generator = d.MainSolution }, "/tests/0/input/generator", "generator"},
		"two generators of one name": {func(d *Draft) {
			d.Generators = append(d.Generators, DraftNamedProgram{Name: "gen", DraftProgram: DraftProgram{Language: "cpp"}})
		}, "/generators/1/name", "duplicate name \"gen\""},
		"two library files of one name": {func(d *Draft) {
			d.Library = append(d.Library, DraftLibraryFile{Name: "testlib.h"})
		}, "/library/1/name", "duplicate file name \"testlib.h\""},
		"library name with a folder": {func(d *Draft) { d.Library[0].Name = "lib/testlib.h" }, "/library/0/name", "invalid or duplicate file name"},
		"subtask cycle": {func(d *Draft) {
			other := uuid.New()
			d.Subtasks = append(d.Subtasks, DraftSubtask{ID: other, Ordinal: 2, Policy: "each", DependsOn: []uuid.UUID{d.Subtasks[0].ID}})
			d.Subtasks[0].DependsOn = []uuid.UUID{other}
		}, "/subtasks/1/depends_on/0", "cycle"},
		"auto-updated file with own text": {func(d *Draft) {
			d.Library[0].AutoUpdate, d.Library[0].Content = true, "// patched"
		}, "/library/0/auto_update", "only an empty system library file"},
		"second validator": {func(d *Draft) {
			d.Validators = []DraftNamedProgram{{Name: "a", DraftProgram: DraftProgram{Language: "cpp"}}, {Name: "b", DraftProgram: DraftProgram{Language: "cpp"}}}
		}, "/validators/1", "at most one validator"},
		"image of a missing asset": {func(d *Draft) { d.Statements[0].Notes = "See ![tree](tree.png)." }, "/statements/0/notes", "notes section of the en statement shows missing asset \"tree.png\""},
	} {
		t.Run(name, func(t *testing.T) {
			d := validDraft()
			tc.change(&d)
			fields := fieldErrors(t, d.Validate(testQuotas, nil))
			require.Len(t, fields, 1, "%v", fields)
			require.Equal(t, tc.pointer, fields[0].Pointer)
			require.Contains(t, fields[0].Message, tc.message)
		})
	}
}

// Renaming an asset breaks the sections that show it; the save is accepted
// so that the next save can fix them, while a build is not.
func TestRenamedAssetBlocksBuildUntilStatementFollows(t *testing.T) {
	saved := validDraft()
	renamed := saved
	renamed.Assets = []DraftAsset{{Name: "plot.png", ObjectID: saved.Assets[0].ObjectID}}
	require.NoError(t, renamed.Validate(testQuotas, &saved))
	fields := fieldErrors(t, renamed.ValidateBuild(testQuotas, map[string]sandbox.Language{"cpp": {}, "python": {}}))
	require.Equal(t, pkg.FieldErrors{{Pointer: "/statements/0/legend", Message: `legend section of the en statement shows missing asset "graph.png"`}}, fields)

	typo := renamed
	typo.Statements = []DraftStatement{{Language: "en", Legend: "![graph](plott.png)"}}
	require.Equal(t, "/statements/0/legend", fieldErrors(t, typo.Validate(testQuotas, &renamed))[0].Pointer)

	fixed := renamed
	fixed.Statements = []DraftStatement{{Language: "en", Legend: "![graph](plot.png)"}}
	require.NoError(t, fixed.Validate(testQuotas, &renamed))
}

func TestImageReferencesReadAsTheStatementRenders(t *testing.T) {
	markdown := "![a](./a.png#center) ![b](<my pic.png>) ![c](https://example.com/c.png) ![d](100%25.png)\n" +
		"![e](/rooted.png) ![f](data:image/png;base64,AA) `![g](code.png)` ![h][ref]\n" +
		"![i](a&amp;b.png) ![j](a\\(b\\).png) ![k](&#x433;.png) ![l](%D0%B3.png)\n\n[ref]: h.png\n"
	require.Equal(t, []string{"a.png", "my pic.png", "100%.png", "h.png", "a&b.png", "a(b).png", "г.png", "г.png"}, ImageReferences(markdown))
}
