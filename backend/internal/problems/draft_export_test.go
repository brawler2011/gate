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

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
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
	uc := newImportUseCase(t)
	plan, err := importDir(t, "../../pkg/testdata/gfmt/subtasks-groups", nil)
	require.NoError(t, err)
	defer plan.Close()
	d := &plan.draft
	d.Library = append(d.Library, model.DraftLibraryFile{Name: "util.h", Content: "#pragma once\n"})
	d.Checker = &model.DraftChecker{DraftProgram: model.DraftProgram{Language: "cpp17"}, Standard: "wcmp"}
	d.Generators = append(d.Generators, model.DraftNamedProgram{Name: "gen", DraftProgram: model.DraftProgram{Language: "python3", Source: "print(1)\n"}})
	gen, other := "gen", d.Solutions[1].Name
	// Interleave subtasks, give the second subtask the first name, and use arguments that need quoting.
	d.Subtasks[1].Name = d.Subtasks[0].Name
	d.Subtasks[0].DependsOn = []uuid.UUID{d.Subtasks[1].ID}
	d.Tests = append(d.Tests, model.DraftTest{ID: uuid.New(), Ordinal: 3, SubtaskID: d.Subtasks[0].ID, Points: 5, Sample: true,
		Input:  model.DraftInput{Mode: "generator", Generator: &gen, Args: []string{"", "it's", "a b", "$x", "-n=1"}},
		Answer: model.DraftAnswer{Mode: "generated", Solution: &other}})
	d.Tests[0].Ordinal, d.Tests[1].Ordinal = 2, 1
	d.Tests[0], d.Tests[1] = d.Tests[1], d.Tests[0]
	require.NoError(t, d.ComputeScores())
	require.NoError(t, d.Validate(importQuotas, nil))

	var exported bytes.Buffer
	_, err = writeDraftArchive(&exported, *d, uc.Languages, plan.open)
	require.NoError(t, err)
	again, err := uc.parseImport(context.Background(), &exported, uuid.New(), 1)
	require.NoError(t, err)
	defer again.Close()
	// The export renames repeated subtask names, since gfmt keys subtasks by name.
	require.Equal(t, d.Subtasks[0].Name+"-2", again.draft.Subtasks[1].Name)
	again.draft.Subtasks[1].Name = d.Subtasks[1].Name
	require.Equal(t, normalized(plan), normalized(again))
}

// An archive the import would reject for size is not exported.
func TestExportRespectsImportLimits(t *testing.T) {
	plan, err := importDir(t, "../../pkg/testdata/gfmt/a-plus-b", nil)
	require.NoError(t, err)
	defer plan.Close()
	export := func(uc *DraftUseCase) error {
		f, err := os.CreateTemp(t.TempDir(), "export-")
		require.NoError(t, err)
		defer f.Close()
		w, err := writeDraftArchive(f, plan.draft, uc.Languages, plan.open)
		require.NoError(t, err)
		return uc.checkExport(f, w)
	}
	uc := newImportUseCase(t)
	require.NoError(t, export(uc))
	uc.BuildOptions.Limits.FileCount = 3
	require.ErrorIs(t, export(uc), pkg.ErrPayloadTooLarge)
	uc = newImportUseCase(t)
	uc.BuildOptions.Limits.DiskBytes = 1024
	require.ErrorIs(t, export(uc), pkg.ErrPayloadTooLarge)
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
		if t.Input.ObjectID != nil {
			h := hashes[*t.Input.ObjectID]
			t.Input.ObjectID = &h
		}
		if t.Answer.ObjectID != nil {
			h := hashes[*t.Answer.ObjectID]
			t.Answer.ObjectID = &h
		}
	}
	// Re-import lists programs in file name order.
	for _, list := range []*[]model.DraftNamedProgram{&d.Validators, &d.Generators, &d.Solutions} {
		*list = slices.SortedFunc(slices.Values(*list), func(a, b model.DraftNamedProgram) int { return strings.Compare(a.Name, b.Name) })
	}
	d.Assets = append([]model.DraftAsset(nil), d.Assets...)
	for n := range d.Assets {
		d.Assets[n].ObjectID = hashes[d.Assets[n].ObjectID]
	}
	return d
}
