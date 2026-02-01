package problemformat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MarshalManifest marshals manifest to JSON bytes
func MarshalManifest(manifest *ProblemManifest) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	return data, nil
}

// MarshalTestsMetadata marshals tests metadata to JSON bytes
func MarshalTestsMetadata(tests *TestsMetadata) ([]byte, error) {
	data, err := json.MarshalIndent(tests, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tests metadata: %w", err)
	}
	return data, nil
}

// SaveManifest сохраняет manifest.json
func SaveManifest(problemDir string, manifest *ProblemManifest) error {
	manifestPath := filepath.Join(problemDir, "manifest.json")

	data, err := MarshalManifest(manifest)
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest.json: %w", err)
	}

	return nil
}

// SaveTestsMetadata сохраняет tests/tests.json
func SaveTestsMetadata(problemDir string, tests *TestsMetadata) error {
	testsMetaPath := filepath.Join(problemDir, "tests", "tests.json")

	data, err := MarshalTestsMetadata(tests)
	if err != nil {
		return err
	}

	if err := os.WriteFile(testsMetaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tests/tests.json: %w", err)
	}

	return nil
}

// SaveTestData сохраняет тест (создает tests/{num}.in и tests/{num}.out)
func SaveTestData(problemDir string, testNum int, input, output []byte) error {
	testsDir := filepath.Join(problemDir, "tests")

	inputPath := filepath.Join(testsDir, fmt.Sprintf("%02d.in", testNum))
	outputPath := filepath.Join(testsDir, fmt.Sprintf("%02d.out", testNum))

	if err := os.WriteFile(inputPath, input, 0644); err != nil {
		return fmt.Errorf("failed to write input file %02d.in: %w", testNum, err)
	}

	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write output file %02d.out: %w", testNum, err)
	}

	return nil
}
