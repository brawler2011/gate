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

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest.json: %w", err)
	}

	return nil
}

// SaveTestsMetadata сохраняет tests/tests.json
func SaveTestsMetadata(problemDir string, tests *TestsMetadata) error {
	testsDir := filepath.Join(problemDir, "tests")

	// Create tests directory if it doesn't exist
	if err := os.MkdirAll(testsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tests directory: %w", err)
	}

	testsMetaPath := filepath.Join(testsDir, "tests.json")

	data, err := json.MarshalIndent(tests, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tests metadata: %w", err)
	}

	if err := os.WriteFile(testsMetaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tests/tests.json: %w", err)
	}

	return nil
}

// SaveTestData сохраняет тест (создает tests/{num}.in и tests/{num}.out)
func SaveTestData(problemDir string, testNum int, input, output []byte) error {
	testsDir := filepath.Join(problemDir, "tests")

	// Create tests directory if it doesn't exist
	if err := os.MkdirAll(testsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tests directory: %w", err)
	}

	inputPath := filepath.Join(testsDir, fmt.Sprintf("%d.in", testNum))
	outputPath := filepath.Join(testsDir, fmt.Sprintf("%d.out", testNum))

	if err := os.WriteFile(inputPath, input, 0644); err != nil {
		return fmt.Errorf("failed to write input file %d.in: %w", testNum, err)
	}

	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write output file %d.out: %w", testNum, err)
	}

	return nil
}

// SaveMedia сохраняет media/media.json
func SaveMedia(problemDir string, media *Media) error {
	mediaDir := filepath.Join(problemDir, "media")

	// Create media directory if it doesn't exist
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return fmt.Errorf("failed to create media directory: %w", err)
	}

	mediaPath := filepath.Join(mediaDir, "media.json")

	data, err := json.MarshalIndent(media, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal media: %w", err)
	}

	if err := os.WriteFile(mediaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write media/media.json: %w", err)
	}

	return nil
}
