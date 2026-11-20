package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Importing an exported draft gives the same draft apart from identifiers.
func TestExportRoundTrip(t *testing.T) {
	archives := map[string][]byte{}
	for _, template := range templates.ListBuiltinTemplates() {
		data, err := templates.GetBuiltinTemplateZip(template.ID)
		require.NoError(t, err)
		archives[template.ID] = data
	}
	packages, err := filepath.Glob("../../pkg/testdata/*/*")
	require.NoError(t, err)
	for _, dir := range packages {
		archives[filepath.Base(filepath.Dir(dir))+"/"+filepath.Base(dir)] = zipDir(t, dir)
	}
	uc := newImportUseCase(t)
	for name, data := range archives {
		t.Run(name, func(t *testing.T) {
			first, err := uc.parseImport(context.Background(), bytes.NewReader(data), uuid.New(), 1)
			require.NoError(t, err)
			defer first.Close()
			var exported bytes.Buffer
			_, err = writeDraftArchive(&exported, first.draft, uc.Languages, first.open)
			require.NoError(t, err)
			second, err := uc.parseImport(context.Background(), &exported, uuid.New(), 1)
			require.NoError(t, err)
			defer second.Close()
			require.Equal(t, normalized(first), normalized(second))
		})
	}
}

// Every draft field survives the round trip, including ones no fixture sets.
func TestExportRoundTripKeepsDraftDetails(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/gfmt/subtasks-groups", nil)
	require.NoError(t, err)
	defer plan.Close()
	d := &plan.draft
	d.Library = append(d.Library, model.DraftLibraryFile{Name: "util.h", Content: "#pragma once\n"})
	object := *d.Tests[0].Input.ObjectID
	half := points.Value(50)
	d.Checker = &model.DraftChecker{DraftProgram: model.DraftProgram{Language: "cpp17"}, Standard: "wcmp", Tests: []model.DraftCheckerTest{
		{ID: uuid.New(), Input: object, Output: object, Answer: object, Verdict: "OK"},
		{ID: uuid.New(), Input: object, Output: object, Answer: object, Verdict: "POINTS", Points: &half},
	}}
	d.Generators = append(d.Generators, model.DraftNamedProgram{Name: "gen", DraftProgram: model.DraftProgram{Language: "python3", Source: "print(1)\n"}})
	d.Validators = append(d.Validators, model.DraftValidator{DraftNamedProgram: model.DraftNamedProgram{Name: "second", DraftProgram: model.DraftProgram{Language: "python3", Source: "pass\n"}},
		Subtasks: []uuid.UUID{d.Subtasks[1].ID}, Tests: []model.DraftValidatorTest{{ID: uuid.New(), Input: object, Valid: true, Group: "a b"}, {ID: uuid.New(), Input: object}}})
	gen, other := "gen", d.Solutions[1].Name
	partial := points.Value(5)
	d.Solutions[0].Verdicts = []string{"TLE", "OK"}
	d.Solutions[1].Verdicts, d.Solutions[1].Points = []string{"WA"}, &partial
	d.Solutions = append(d.Solutions, model.DraftSolution{DraftNamedProgram: model.DraftNamedProgram{Name: "slow", DraftProgram: model.DraftProgram{Language: "python3", Source: "pass\n"}}, Verdicts: []string{}})
	// gfmt otherwise treats a subtask named samples as the sample group.
	d.Subtasks[0].Name, d.Subtasks[1].Name = "samples", ""
	d.Subtasks[0].DependsOn = []uuid.UUID{d.Subtasks[1].ID}
	d.Tests = append(d.Tests, model.DraftTest{ID: uuid.New(), Ordinal: 3, SubtaskID: d.Subtasks[0].ID, Points: 5, Sample: true,
		Input:  model.DraftInput{Mode: "generator", Generator: &gen, Args: []string{"", "it's", "a b", "$x", "-n=1"}},
		Answer: model.DraftAnswer{Mode: "generated", Solution: &other}})
	// Interleave the subtasks' tests.
	d.Tests[0].Ordinal, d.Tests[1].Ordinal = 2, 1
	d.Tests[0], d.Tests[1] = d.Tests[1], d.Tests[0]
	require.NoError(t, d.ComputeScores())
	require.NoError(t, d.Validate(importQuotas, nil))

	require.Equal(t, normalized(plan), normalized(exportAndImport(t, plan)))
}

// Ungrouped Polygon tests stay in an unnamed subtask, so validators get no --group after a round trip.
func TestExportRoundTripKeepsUngroupedSubtaskUnnamed(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/polygon/a-plus-b", nil)
	require.NoError(t, err)
	defer plan.Close()
	require.Empty(t, plan.draft.Subtasks[0].Name)
	require.Empty(t, exportAndImport(t, plan).draft.Subtasks[0].Name)
}

// Library files survive even when no judge program selects them yet.
func TestExportRoundTripKeepsLibraryWithoutJudgePrograms(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/gfmt/subtasks-groups", nil)
	require.NoError(t, err)
	defer plan.Close()
	plan.draft.Checker, plan.draft.Validators = nil, nil
	plan.draft.Library = append(plan.draft.Library, model.DraftLibraryFile{Name: "util.h", Content: "#pragma once\n"})
	require.NoError(t, plan.draft.Validate(importQuotas, nil))
	require.Equal(t, normalized(plan), normalized(exportAndImport(t, plan)))
}

// A saved draft round-trips although the editor sent sections with edge whitespace.
func TestExportRoundTripKeepsSavedStatements(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/gfmt/a-plus-b", nil)
	require.NoError(t, err)
	defer plan.Close()
	repo := &savingRepo{}
	uc := &DraftUseCase{Repo: repo, Problems: singleProblem{uuid.New()}, Permissions: allowProblemPermissions{}}
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
	plan.draft.Statements[0].Legend = "    indented code\n\nText\n"
	_, err = uc.SaveDraft(ctx, plan.draft.ProblemID, plan.draft.Revision, "key", plan.draft)
	require.NoError(t, err)
	plan.draft = repo.saved[0]
	require.Equal(t, "    indented code\n\nText", plan.draft.Statements[0].Legend)
	require.Equal(t, normalized(plan), normalized(exportAndImport(t, plan)))
}

func exportAndImport(t *testing.T, plan *draftImport) *draftImport {
	t.Helper()
	uc := newImportUseCase(t)
	var exported bytes.Buffer
	_, err := writeDraftArchive(&exported, plan.draft, uc.Languages, plan.open)
	require.NoError(t, err)
	again, err := uc.parseImport(context.Background(), &exported, uuid.New(), 1)
	require.NoError(t, err)
	t.Cleanup(again.Close)
	return again
}

// The export accepts exactly the archives the import can extract.
func TestExportRespectsImportLimits(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/gfmt/a-plus-b", nil)
	require.NoError(t, err)
	defer plan.Close()
	export := func(uc *DraftUseCase) (*os.File, error) {
		f, err := os.CreateTemp(t.TempDir(), "export-")
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })
		w, err := writeDraftArchive(f, plan.draft, uc.Languages, plan.open)
		require.NoError(t, err)
		return f, uc.checkExport(f, w)
	}
	uc := newImportUseCase(t)
	f, err := export(uc)
	require.NoError(t, err)
	archive, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	// Find the smallest file count the import accepts.
	for uc.BuildOptions.Limits.FileCount = 1; ; uc.BuildOptions.Limits.FileCount++ {
		again, err := uc.parseImport(context.Background(), bytes.NewReader(archive), uuid.New(), 1)
		if err == nil {
			again.Close()
			break
		}
	}
	_, err = export(uc)
	require.NoError(t, err)
	uc.BuildOptions.Limits.FileCount--
	_, err = export(uc)
	require.ErrorIs(t, err, pkg.ErrPayloadTooLarge)

	uc = newImportUseCase(t)
	uc.BuildOptions.Limits.DiskBytes = 1024
	_, err = export(uc)
	require.ErrorIs(t, err, pkg.ErrPayloadTooLarge)
}

func zipDir(t *testing.T, dir string) []byte {
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	require.NoError(t, w.AddFS(os.DirFS(dir)))
	require.NoError(t, w.Close())
	return archive.Bytes()
}

func (i *draftImport) open(oid uuid.UUID) (io.ReadCloser, error) {
	for _, o := range i.objects {
		if o.temporaryID == oid {
			f, err := os.Open(o.path)
			if err != nil {
				return nil, fmt.Errorf("open: %w", err)
			}
			return f, nil
		}
	}
	return nil, os.ErrNotExist
}

func clonePointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// normalized replaces identifiers with ordinals and object IDs with content hashes.
func normalized(i *draftImport) model.Draft {
	d := i.draft
	d.ProblemID, d.Revision, d.ShortName = uuid.Nil, 0, ""
	hashes := map[uuid.UUID]uuid.UUID{}
	for _, o := range i.objects {
		hashes[o.temporaryID] = uuid.NewSHA1(uuid.Nil, []byte(o.spec.SHA256))
	}
	subtasks := map[uuid.UUID]uuid.UUID{}
	d.Subtasks = append([]model.DraftSubtask(nil), d.Subtasks...)
	for n := range d.Subtasks {
		subtasks[d.Subtasks[n].ID] = uuid.NewSHA1(uuid.Nil, []byte(strconv.Itoa(d.Subtasks[n].Ordinal)))
	}
	for n := range d.Subtasks {
		s := &d.Subtasks[n]
		s.ID = subtasks[s.ID]
		deps := make([]uuid.UUID, len(s.DependsOn))
		for k, dep := range s.DependsOn {
			deps[k] = subtasks[dep]
		}
		s.DependsOn = deps
	}
	d.Tests = append([]model.DraftTest(nil), d.Tests...)
	for n := range d.Tests {
		t := &d.Tests[n]
		t.ID, t.SubtaskID = uuid.Nil, subtasks[t.SubtaskID]
		t.Input.ObjectID, t.Answer.ObjectID = clonePointer(t.Input.ObjectID), clonePointer(t.Answer.ObjectID)
	}
	d.Assets = slices.Clone(d.Assets)
	if d.Checker != nil {
		checker := *d.Checker
		checker.Tests = slices.Clone(checker.Tests)
		for n := range checker.Tests {
			checker.Tests[n].ID = uuid.Nil
		}
		d.Checker = &checker
	}
	d.Validators = slices.Clone(d.Validators)
	for n := range d.Validators {
		d.Validators[n].Tests = slices.Clone(d.Validators[n].Tests)
		for k := range d.Validators[n].Tests {
			d.Validators[n].Tests[k].ID = uuid.Nil
		}
	}
	d.EachObject(func(_ string, id *uuid.UUID) { *id = hashes[*id] })
	d.Normalize()
	// Re-import lists programs in file name order.
	d.Validators = slices.SortedFunc(slices.Values(d.Validators), func(a, b model.DraftValidator) int { return strings.Compare(a.Name, b.Name) })
	for n := range d.Validators {
		v := &d.Validators[n]
		ids := make([]uuid.UUID, len(v.Subtasks))
		for k, id := range v.Subtasks {
			ids[k] = subtasks[id]
		}
		v.Subtasks = ids
	}
	d.Generators = slices.SortedFunc(slices.Values(d.Generators), func(a, b model.DraftNamedProgram) int { return strings.Compare(a.Name, b.Name) })
	d.Solutions = slices.SortedFunc(slices.Values(d.Solutions), func(a, b model.DraftSolution) int { return strings.Compare(a.Name, b.Name) })
	d.Assets = append([]model.DraftAsset(nil), d.Assets...)
	for n := range d.Assets {
		d.Assets[n].ObjectID = hashes[d.Assets[n].ObjectID]
	}
	return d
}
