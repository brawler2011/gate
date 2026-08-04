package usecase

import (
	"testing"

	"github.com/gate149/gate/backend/internal/domain/models"
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
