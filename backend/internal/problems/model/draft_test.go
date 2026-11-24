package problemsmodel

import (
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var testQuotas = DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 10, Tests: 10, Components: 10, Groups: 10}

func validDraft() Draft {
	gen, sol, sub := "gen", "main", uuid.New()
	return Draft{ProblemID: uuid.New(), Revision: 1, Title: "A", ShortName: "a", Type: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256,
		DraftContent: DraftContent{
			Checker:      &DraftChecker{DraftProgram: DraftProgram{Language: "cpp"}, DraftJury: DraftJury{Protocol: sandbox.ProtocolTestlib}},
			Generators:   []DraftNamedProgram{{Name: gen, DraftProgram: DraftProgram{Language: "python"}}},
			Solutions:    []DraftSolution{{DraftNamedProgram: DraftNamedProgram{Name: sol, DraftProgram: DraftProgram{Language: "cpp"}}, Verdicts: []string{"OK"}}},
			MainSolution: &sol,
			Library:      []DraftLibraryFile{{Name: "testlib.h"}},
			Statements:   []DraftStatement{{Language: "en", Legend: "\\includegraphics[width=3cm]{graph.png}"}},
			Assets:       []DraftAsset{{Name: "graph.png", ObjectID: uuid.New()}},
			Testset: DraftGroup{ID: uuid.New(), Aggregate: "sum", Feedback: "complete",
				Groups: []DraftGroup{{ID: sub, Name: "main", Aggregate: "min", Feedback: "complete"}}},
			Tests: []DraftTest{{ID: uuid.New(), Ordinal: 1, GroupID: sub,
				Input:  DraftInput{Mode: "generator", Generator: &gen},
				Answer: DraftAnswer{Mode: "generated", Solution: &sol}}},
		}}
}

func fieldErrors(t *testing.T, err error) pkg.FieldErrors {
	t.Helper()
	var fields pkg.FieldErrors
	require.ErrorAs(t, err, &fields)
	require.ErrorIs(t, err, pkg.ErrInvalid)
	return fields
}

// Each violation names the entry the workshop has to show it at.
func TestValidateLocatesViolations(t *testing.T) {
	require.NoError(t, func() error { d := validDraft(); return d.Validate(testQuotas, nil) }())
	for name, tc := range map[string]struct {
		change  func(*Draft)
		pointer string
		code    pkg.Code
	}{
		"test input from a solution": {func(d *Draft) { d.Tests[0].Input.Generator = d.MainSolution }, "/tests/0/input/generator", pkg.ErrUnknown.Code},
		"two generators of one name": {func(d *Draft) {
			d.Generators = append(d.Generators, DraftNamedProgram{Name: "gen", DraftProgram: DraftProgram{Language: "cpp"}})
		}, "/generators/1/name", pkg.ErrDuplicate.Code},
		"two library files of one name": {func(d *Draft) {
			d.Library = append(d.Library, DraftLibraryFile{Name: "testlib.h"})
		}, "/library/1/name", pkg.ErrDuplicate.Code},
		"library name with a folder": {func(d *Draft) { d.Library[0].Name = "lib/testlib.h" }, "/library/0/name", pkg.ErrFormat.Code},
		"requirement cycle": {func(d *Draft) {
			other := uuid.New()
			d.Testset.Groups = append(d.Testset.Groups, DraftGroup{ID: other, Name: "other", Aggregate: "sum", Feedback: "complete", Requires: []uuid.UUID{d.Testset.Groups[0].ID}})
			d.Testset.Groups[0].Requires = []uuid.UUID{other}
		}, "/testset/groups/1/requires/0", pkg.ErrCycle.Code},
		"requirement of a subgroup": {func(d *Draft) {
			inner := uuid.New()
			d.Testset.Groups[0].Groups = []DraftGroup{{ID: inner, Name: "inner", Aggregate: "sum", Feedback: "complete"}}
			d.Testset.Groups[0].Requires = []uuid.UUID{inner}
		}, "/testset/groups/0/requires/0", pkg.ErrInconsistent.Code},
		"requirement of an ancestor": {func(d *Draft) {
			d.Testset.Groups[0].Groups = []DraftGroup{{ID: uuid.New(), Name: "inner", Aggregate: "sum", Feedback: "complete", Requires: []uuid.UUID{d.Testset.Groups[0].ID}}}
		}, "/testset/groups/0/groups/0/requires/0", pkg.ErrInconsistent.Code},
		"testset with requirements": {func(d *Draft) {
			d.Testset.Requires = []uuid.UUID{d.Testset.Groups[0].ID}
		}, "/testset/requires/0", pkg.ErrInconsistent.Code},
		"auto-updated file with own text": {func(d *Draft) {
			d.Library[0].AutoUpdate, d.Library[0].Content = true, "// patched"
		}, "/library/0/auto_update", pkg.ErrInconsistent.Code},
		"unnamed subgroup": {func(d *Draft) { d.Testset.Groups[0].Name = "" }, "/testset/groups/0/name", pkg.ErrRequired.Code},
		"two groups of one name": {func(d *Draft) {
			d.Testset.Groups[0].Groups = []DraftGroup{{ID: uuid.New(), Name: "main", Aggregate: "sum", Feedback: "complete"}}
		}, "/testset/groups/0/groups/0/name", pkg.ErrDuplicate.Code},
		"validator of an unknown group": {func(d *Draft) {
			d.Validators = []DraftValidator{{DraftNamedProgram: DraftNamedProgram{Name: "a", DraftProgram: DraftProgram{Language: "cpp"}}, DraftJury: DraftJury{Protocol: sandbox.ProtocolTestlib}, Groups: []uuid.UUID{uuid.New()}}}
		}, "/validators/0/groups/0", pkg.ErrUnknown.Code},
		"validator of the testset": {func(d *Draft) {
			d.Validators = []DraftValidator{{DraftNamedProgram: DraftNamedProgram{Name: "a", DraftProgram: DraftProgram{Language: "cpp"}}, DraftJury: DraftJury{Protocol: sandbox.ProtocolTestlib}, Groups: []uuid.UUID{d.Testset.ID}}}
		}, "/validators/0/groups/0", pkg.ErrInconsistent.Code},
		"group arguments of a testlib checker": {func(d *Draft) { d.Testset.Groups[0].CheckerArgs = []string{"case_sensitive"} }, "/testset/groups/0/checker_args", pkg.ErrInconsistent.Code},
		"arguments of a testlib checker": {func(d *Draft) { d.Checker.Args = []string{"case_sensitive"} }, "/checker/args", pkg.ErrInconsistent.Code},
		"image of a missing asset": {func(d *Draft) { d.Statements[0].Notes = "See \\includegraphics{tree.png}." }, "/statements/0/notes", pkg.ErrUnknown.Code},
	} {
		t.Run(name, func(t *testing.T) {
			d := validDraft()
			tc.change(&d)
			fields := fieldErrors(t, d.Validate(testQuotas, nil))
			require.Len(t, fields, 1, "%v", fields)
			require.Equal(t, tc.pointer, fields[0].Pointer)
			require.Equal(t, tc.code, fields[0].Code)
		})
	}
}

// Asset renames may save with stale image references, but builds must reject them.
func TestRenamedAssetBlocksBuildUntilStatementFollows(t *testing.T) {
	saved := validDraft()
	renamed := saved
	renamed.Assets = []DraftAsset{{Name: "plot.png", ObjectID: saved.Assets[0].ObjectID}}
	require.NoError(t, renamed.Validate(testQuotas, &saved))
	fields := fieldErrors(t, renamed.ValidateBuild(testQuotas, map[string]sandbox.Language{"cpp": {}, "python": {}}))
	require.Equal(t, pkg.FieldErrors{{Pointer: "/statements/0/legend", Code: pkg.ErrUnknown.Code, Params: map[string]any{"value": "graph.png"}, Message: `legend section of the en statement shows missing asset "graph.png"`}}, fields)

	typo := renamed
	typo.Statements = []DraftStatement{{Language: "en", Legend: "\\includegraphics{plott.png}"}}
	require.Equal(t, "/statements/0/legend", fieldErrors(t, typo.Validate(testQuotas, &renamed))[0].Pointer)

	fixed := renamed
	fixed.Statements = []DraftStatement{{Language: "en", Legend: "\\includegraphics{plot.png}"}}
	require.NoError(t, fixed.Validate(testQuotas, &renamed))

	// graphicx finds plot.png for a name without an extension.
	bare := renamed
	bare.Statements = []DraftStatement{{Language: "en", Legend: "\\includegraphics[width=2cm]{plot}"}}
	require.NoError(t, bare.Validate(testQuotas, &renamed))
}
