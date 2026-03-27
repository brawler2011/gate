package usecase

import (
	"testing"

	"github.com/gate149/gate/backend/pkg/problemformat"
)

func TestDefaultTestsMetadata_IsValid(t *testing.T) {
	manifest := defaultManifest("Sample Problem")
	metadata := defaultTestsMetadata()

	if err := problemformat.ValidateTestsMetadata(metadata, manifest); err != nil {
		t.Fatalf("default tests metadata must be valid, got error: %v", err)
	}
}
