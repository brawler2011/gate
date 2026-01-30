package vcs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/google/uuid"
)

// LoadManifest reads and parses manifest.json from repo
func (s *GoGitService) LoadManifest(ctx context.Context, problemID uuid.UUID) (*problemformat.ProblemManifest, error) {
	data, err := s.ReadFile(ctx, problemID, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest problemformat.ProblemManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// SaveManifest writes manifest.json to repo (without commit)
func (s *GoGitService) SaveManifest(ctx context.Context, problemID uuid.UUID, manifest *problemformat.ProblemManifest) error {
	// Update last_updated timestamp
	manifest.LastUpdated = time.Now()

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := s.WriteFile(ctx, problemID, "manifest.json", data); err != nil {
		return fmt.Errorf("failed to write manifest.json: %w", err)
	}

	return nil
}

// ValidateRepoStructure checks if repo has valid problem structure
func (s *GoGitService) ValidateRepoStructure(ctx context.Context, problemID uuid.UUID) error {
	repoPath := s.getRepoPath(problemID)

	// Check required directories
	requiredDirs := []string{
		"statement",
		"tests",
	}

	for _, dir := range requiredDirs {
		dirPath := filepath.Join(repoPath, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			return fmt.Errorf("required directory missing: %s", dir)
		}
	}

	// Check if manifest.json exists
	if _, err := os.Stat(filepath.Join(repoPath, "manifest.json")); os.IsNotExist(err) {
		return fmt.Errorf("manifest.json not found")
	}

	// Try to load and validate manifest
	_, err := s.LoadManifest(ctx, problemID)
	if err != nil {
		return fmt.Errorf("invalid manifest.json: %w", err)
	}

	return nil
}

// InitDefaultManifest creates a default manifest.json for a new problem
func (s *GoGitService) InitDefaultManifest(ctx context.Context, problemID uuid.UUID, title string) error {
	manifest := &problemformat.ProblemManifest{
		LastUpdated:     time.Now(),
		ProblemType:     "pass-fail",
		MaxScore:        nil,
		FilesMetadata:   []problemformat.FileMetadata{},
		TimeLimitMs:     1000,
		MemoryLimitMb:   256,
		StdoutLimitMb:   64,
		CodeSizeLimitKb: 256,
		Statements: map[string]problemformat.Statement{
			"en": {
				Title:        title,
				Legend:       "Problem description goes here.",
				InputFormat:  "Input format description.",
				OutputFormat: "Output format description.",
				Notes:        "",
				Interaction:  "",
				Scoring:      "",
			},
		},
	}

	return s.SaveManifest(ctx, problemID, manifest)
}

// InitDefaultTestsMetadata creates a default tests/tests.json
func (s *GoGitService) InitDefaultTestsMetadata(ctx context.Context, problemID uuid.UUID) error {
	testsMetadata := &problemformat.TestsMetadata{
		Groups: []problemformat.TestGroup{
			{
				Ordinal:      1,
				Name:         "Samples",
				Points:       0,
				PointsPolicy: "complete-group",
				DependsOn:    []int{},
				Tests:        [2]int{1, 1},
			},
		},
		Tests: []problemformat.TestCase{
			{
				Ordinal:   1,
				Method:    "manual",
				Generator: nil,
				IsSample:  true,
			},
		},
	}

	data, err := json.MarshalIndent(testsMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tests metadata: %w", err)
	}

	if err := s.WriteFile(ctx, problemID, "tests/tests.json", data); err != nil {
		return fmt.Errorf("failed to write tests/tests.json: %w", err)
	}

	return nil
}
