package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/formats"
	"github.com/stretchr/testify/require"

	"github.com/brawler2011/gate/backend/internal/domain/models"
)

func TestDefaultTestsMetadata_IsValid(t *testing.T) {
	metadata := defaultTestsMetadata()

	if len(metadata.Tests) == 0 {
		t.Fatalf("default tests metadata must contain tests")
	}
}

func TestManifestAndTestsToGfmtProblem_PreservesIsSample(t *testing.T) {
	manifest := defaultManifest("Sample Problem")
	testsMeta := &models.TestsMetadata{
		Groups: []models.TestGroup{
			{
				Ordinal:      1,
				Name:         "subtask1",
				Points:       100,
				PointsPolicy: "complete-group",
				DependsOn:    []int{},
				Tests:        [2]int{1, 2},
			},
		},
		Tests: []models.TestCase{
			{
				Ordinal:  1,
				Method:   "manual",
				IsSample: true,
			},
			{
				Ordinal:  2,
				Method:   "manual",
				IsSample: false,
			},
		},
	}

	gfmtProb := ManifestAndTestsToGfmtProblem(manifest, testsMeta)
	subtask1, ok := gfmtProb.Subtasks["subtask1"]
	if !ok {
		t.Fatalf("subtask1 not found in gfmt problem")
	}
	if len(subtask1.Tests) != 2 {
		t.Fatalf("expected 2 tests in subtask1, got %d", len(subtask1.Tests))
	}
	if !subtask1.Tests[0].Sample {
		t.Errorf("expected test 1 to have Sample=true")
	}
	if subtask1.Tests[1].Sample {
		t.Errorf("expected test 2 to have Sample=false")
	}

	// Round-trip back to TestsMetadata
	roundTripMeta := GfmtProblemToTestsMetadata(gfmtProb)
	if len(roundTripMeta.Tests) != 2 {
		t.Fatalf("expected 2 tests in roundtrip, got %d", len(roundTripMeta.Tests))
	}
	if !roundTripMeta.Tests[0].IsSample {
		t.Errorf("expected test 1 IsSample=true after roundtrip")
	}
	if roundTripMeta.Tests[1].IsSample {
		t.Errorf("expected test 2 IsSample=false after roundtrip")
	}
}

func TestWorkshopTestsConfig_RoundTrip(t *testing.T) {
	for _, format := range []string{"gfmt", "polygon", "pcms2", "icpc"} {
		for _, name := range []string{"a-plus-b", "subtasks-groups", "max-subarray", "interactive-guess"} {
			t.Run(format+"/"+name, func(t *testing.T) {
				ctx := context.Background()
				src := t.TempDir()
				require.NoError(t, os.CopyFS(src, os.DirFS("../../pkg/testdata/"+format+"/"+name)))
				parser, err := formats.GetParser(format)
				require.NoError(t, err)
				plan, err := parser.Parse(src)
				require.NoError(t, err)
				dst := t.TempDir()
				require.NoError(t, formats.Import(src, dst, parser))
				manifest, err := mapImportPlanToManifest(plan, dst)
				require.NoError(t, err)
				uc, id := newWorkshopValidatorTest(t, false, nil, nil)
				require.NoError(t, uc.workspaceStorage.DeleteProblemWorkspace(ctx, id))
				require.NoError(t, filepath.WalkDir(dst, func(path string, entry fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if entry.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(dst, path)
					if err != nil {
						return fmt.Errorf("relative import path: %w", err)
					}
					data, err := os.ReadFile(path)
					if err != nil {
						return fmt.Errorf("read imported file: %w", err)
					}
					return uc.workspaceStorage.WriteFile(ctx, id, filepath.ToSlash(rel), data)
				}))
				data, err := json.Marshal(manifest)
				require.NoError(t, err)
				require.NoError(t, uc.problemsRepo.UpdateProblemManifest(ctx, id, data))
				before, err := uc.GetTestsConfig(ctx, id)
				require.NoError(t, err)
				original, err := uc.readProblemConfig(ctx, id)
				require.NoError(t, err)
				for range 2 {
					require.NoError(t, uc.UpdateTestsConfig(ctx, id, before))
					require.NoError(t, uc.saveManifest(ctx, id, manifest))
					after, err := uc.GetTestsConfig(ctx, id)
					require.NoError(t, err)
					require.Equal(t, before, after)
					saved, err := uc.readProblemConfig(ctx, id)
					require.NoError(t, err)
					require.Equal(t, original.Checker, saved.Checker)
					require.Equal(t, original.Validator, saved.Validator)
					require.Equal(t, original.Interactor, saved.Interactor)
					require.Equal(t, original.Solutions, saved.Solutions)
				}
			})
		}
	}
}

func TestWorkshopTestsConfig_PreservesNamedFilesOnSave(t *testing.T) {
	data := `limits: {time_ms: 1000, memory_mb: 256}
solutions: {main.cpp: main}
subtasks:
 z:
  dependencies: [a]
  tests:
   - generate: generator 42
   - manual: nested/last.input
 samples:
  tests:
   - manual: example.in
 a:
  dependencies: [samples]
  tests:
   - manual: edge-case.in
`
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{"problem.yaml": data, "tests/edge-case.in": "edge"}, nil)
	ctx := context.Background()
	before, err := uc.GetTestsConfig(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "edge-case.in", before.Tests[1].Filename)
	for range 2 {
		require.NoError(t, uc.UpdateTestsConfig(ctx, id, before))
		manifest, err := uc.loadManifest(ctx, id)
		require.NoError(t, err)
		require.NoError(t, uc.saveManifest(ctx, id, manifest))
		after, err := uc.GetTestsConfig(ctx, id)
		require.NoError(t, err)
		require.Equal(t, before, after)
		prob, err := uc.readProblemConfig(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "main", prob.Solutions["main.cpp"])
	}
	content, err := uc.workspaceStorage.ReadFile(ctx, id, "tests/edge-case.in")
	require.NoError(t, err)
	require.Equal(t, "edge", string(content))
}

func TestWorkshopSaveManifest_IntentionalComponentSelection(t *testing.T) {
	uc, id := newWorkshopValidatorTest(t, true, map[string]string{"problem.yaml": "validator: validators/selected.cpp\nsubtasks: {}\n"}, nil)
	ctx := context.Background()
	manifest, err := uc.loadManifest(ctx, id)
	require.NoError(t, err)
	require.NoError(t, uc.saveManifest(ctx, id, manifest))
	saved, err := uc.readProblemConfig(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "validators/selected.cpp", saved.Validator)
	manifest.FilesMetadata[0].Filename = "validators/replacement.cpp"
	require.NoError(t, uc.saveManifest(ctx, id, manifest))
	saved, err = uc.readProblemConfig(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "validators/replacement.cpp", saved.Validator)
}

func TestWorkshopSaveManifest_UnrelatedComponentMetadata(t *testing.T) {
	uc, id := newWorkshopValidatorTest(t, false, map[string]string{"problem.yaml": "checker: checkers/z-selected.cpp\nvalidator: validators/z-selected.cpp\ninteractor: interactors/z-selected.cpp\nsubtasks: {}\n"}, nil)
	ctx := context.Background()
	manifest, err := uc.loadManifest(ctx, id)
	require.NoError(t, err)
	for _, kind := range []string{"checker", "validator", "interactor"} {
		manifest.FilesMetadata = append(manifest.FilesMetadata, models.FileMetadata{Type: kind, Filename: kind + "s/a-unused.cpp"}, models.FileMetadata{Type: kind, Filename: kind + "s/z-selected.cpp"})
	}
	manifest.FilesMetadata = append(manifest.FilesMetadata, models.FileMetadata{Type: "generator", Filename: "generators/gen.cpp"})
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, uc.problemsRepo.UpdateProblemManifest(ctx, id, data))
	hash := "compiled"
	manifest.FilesMetadata[len(manifest.FilesMetadata)-1].BinarySha256 = &hash
	// Compiling any component changes metadata, not its selected filename.
	manifest.FilesMetadata[0].BinarySha256 = &hash
	require.NoError(t, uc.saveManifest(ctx, id, manifest))
	saved, err := uc.readProblemConfig(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "checkers/z-selected.cpp", saved.Checker)
	require.Equal(t, "validators/z-selected.cpp", saved.Validator)
	require.Equal(t, "interactors/z-selected.cpp", saved.Interactor)
}
