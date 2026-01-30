package problemformat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadManifest читает manifest.json из директории задачи
func LoadManifest(problemDir string) (*ProblemManifest, error) {
	manifestPath := filepath.Join(problemDir, "manifest.json")
	
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest ProblemManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// LoadTestsMetadata читает tests/tests.json
func LoadTestsMetadata(problemDir string) (*TestsMetadata, error) {
	testsMetaPath := filepath.Join(problemDir, "tests", "tests.json")
	
	data, err := os.ReadFile(testsMetaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tests/tests.json: %w", err)
	}

	var testsMetadata TestsMetadata
	if err := json.Unmarshal(data, &testsMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse tests/tests.json: %w", err)
	}

	return &testsMetadata, nil
}

// LoadTestData читает конкретный тест (input/output)
func LoadTestData(problemDir string, testNum int) (input, output []byte, err error) {
	testsDir := filepath.Join(problemDir, "tests")
	
	inputPath := filepath.Join(testsDir, fmt.Sprintf("%d.in", testNum))
	outputPath := filepath.Join(testsDir, fmt.Sprintf("%d.out", testNum))

	input, err = os.ReadFile(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read input file %d.in: %w", testNum, err)
	}

	output, err = os.ReadFile(outputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read output file %d.out: %w", testNum, err)
	}

	return input, output, nil
}

// LoadMedia читает media/media.json
func LoadMedia(problemDir string) (*Media, error) {
	mediaPath := filepath.Join(problemDir, "media", "media.json")
	
	// Media is optional
	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		return &Media{Images: []Image{}}, nil
	}

	data, err := os.ReadFile(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read media/media.json: %w", err)
	}

	var media Media
	if err := json.Unmarshal(data, &media); err != nil {
		return nil, fmt.Errorf("failed to parse media/media.json: %w", err)
	}

	return &media, nil
}
