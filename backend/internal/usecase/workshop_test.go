package usecase

import (
	"testing"
)

func TestDefaultTestsMetadata_IsValid(t *testing.T) {
	metadata := defaultTestsMetadata()

	if len(metadata.Tests) == 0 {
		t.Fatalf("default tests metadata must contain tests")
	}
}
