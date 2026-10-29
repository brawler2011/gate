package usecase

import (
	"context"
	"os"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/formats/polygon"
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
	for _, format := range []string{"gfmt", "polygon"} {
		for _, name := range []string{"a-plus-b", "subtasks-groups", "max-subarray", "interactive-guess"} {
			t.Run(format+"/"+name, func(t *testing.T) {
				var prob *gfmt.Problem
				dir := "../../pkg/testdata/" + format + "/" + name
				if format == "gfmt" {
					pkg, err := gfmt.OpenPackage(dir)
					require.NoError(t, err)
					prob = pkg.Problem
				} else {
					tempDir := t.TempDir()
					require.NoError(t, os.CopyFS(tempDir, os.DirFS(dir)))
					plan, err := polygon.NewParser().Parse(tempDir)
					require.NoError(t, err)
					prob = plan.Problem
				}
				before := GfmtProblemToTestsMetadata(prob)
				after := GfmtProblemToTestsMetadata(ManifestAndTestsToGfmtProblem(defaultManifest("roundtrip"), before))
				require.Equal(t, before, after)
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
