package usecase

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/formats"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
)

type ProblemImportUseCase struct {
	problemsRepo     interfaces.ProblemsRepo
	workspaceStorage *WorkspaceStorage
}

func NewProblemImportUseCase(problemsRepo interfaces.ProblemsRepo, workspaceStorage *WorkspaceStorage) *ProblemImportUseCase {
	return &ProblemImportUseCase{
		problemsRepo:     problemsRepo,
		workspaceStorage: workspaceStorage,
	}
}

func (uc *ProblemImportUseCase) ImportProblemPackage(ctx context.Context, zipReader io.Reader, zipSize int64, problemID uuid.UUID) (*models.ProblemManifest, error) {
	tempDir, err := os.MkdirTemp("", "problem-import-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "package.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp zip file: %w", err)
	}

	if _, err := io.Copy(zipFile, zipReader); err != nil {
		zipFile.Close()
		return nil, fmt.Errorf("failed to copy zip content: %w", err)
	}
	zipFile.Close()

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	tempSrc := filepath.Join(tempDir, "src")
	if err := extractZip(&r.Reader, tempSrc); err != nil {
		return nil, fmt.Errorf("failed to extract zip: %w", err)
	}

	packageRoot, err := detectPackageRoot(tempSrc)
	if err != nil {
		return nil, fmt.Errorf("failed to detect package root: %w", err)
	}

	// Detect format
	format, err := formats.DetectFormat(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to detect package format: %w", err)
	}

	parser, err := formats.GetParser(format)
	if err != nil {
		return nil, fmt.Errorf("failed to get parser for format %s: %w", format, err)
	}

	tempDst := filepath.Join(tempDir, "dst")
	plan, err := parser.Parse(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package: %w", err)
	}

	if err := formats.Import(packageRoot, tempDst, parser); err != nil {
		return nil, fmt.Errorf("failed to import package: %w", err)
	}

	// Clean existing workspace
	if err := uc.workspaceStorage.DeleteProblemWorkspace(ctx, problemID); err != nil {
		return nil, fmt.Errorf("failed to clear existing workspace: %w", err)
	}

	// Upload standardized files to storage workspace
	err = filepath.WalkDir(tempDst, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(tempDst, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return uc.workspaceStorage.WriteFile(ctx, problemID, rel, content)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload workspace files: %w", err)
	}

	// Map and enrich the manifest
	manifest, err := mapImportPlanToManifest(plan, tempDst)
	if err != nil {
		return nil, fmt.Errorf("failed to map import plan to manifest: %w", err)
	}

	enrichManifestDefaults(ctx, uc.problemsRepo, problemID, manifest, format)

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := uc.problemsRepo.UpdateProblemManifest(ctx, problemID, manifestBytes); err != nil {
		return nil, fmt.Errorf("failed to save manifest to database: %w", err)
	}

	if err := uc.problemsRepo.UpdateProblemLimits(ctx, problemID, manifest.TimeLimitMs, manifest.MemoryLimitMb); err != nil {
		return nil, fmt.Errorf("failed to save limits to database: %w", err)
	}

	return manifest, nil
}

func mapImportPlanToManifest(plan *gfmt.ImportPlan, tempDst string) (*models.ProblemManifest, error) {
	manifest := &models.ProblemManifest{
		LastUpdated:   time.Now(),
		ProblemType:   plan.Problem.Type,
		TimeLimitMs:   plan.Problem.Limits.TimeMs,
		MemoryLimitMb: plan.Problem.Limits.MemoryMb,
	}

	manifest.Statement.Title = plan.Problem.Title
	manifest.Statement.Legend = fmt.Sprintf("Imported from unified package.")

	// Try to load statement from statements/en.md
	enStatementPath := filepath.Join(tempDst, "statements", "en.md")
	if data, err := os.ReadFile(enStatementPath); err == nil {
		parsedStmt := ParseStatementMarkdown(string(data))
		manifest.Statement = parsedStmt
		manifest.Statement.Title = plan.Problem.Title // Always preserve title from problem.yaml
	}

	// Populate FilesMetadata
	for _, mapping := range plan.Mappings {
		target := mapping.TargetPath
		var fileType string
		if strings.HasPrefix(target, "checkers/") {
			fileType = "checker"
		} else if strings.HasPrefix(target, "validators/") {
			fileType = "validator"
		} else if strings.HasPrefix(target, "interactors/") {
			fileType = "interactor"
		} else if strings.HasPrefix(target, "generators/") {
			fileType = "generator"
		}

		if fileType != "" {
			// Read file to compute hash
			absPath := filepath.Join(tempDst, target)
			data, err := os.ReadFile(absPath)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(data)
			hashStr := fmt.Sprintf("%x", hash)

			compiler := "cpp17"
			if strings.HasSuffix(target, ".py") {
				compiler = "python3"
			} else if strings.HasSuffix(target, ".go") {
				compiler = "go"
			}

			// Add dependencies if they exist
			var deps []models.Dependency
			for _, m := range plan.Mappings {
				if strings.HasPrefix(m.TargetPath, "lib/") {
					deps = append(deps, models.Dependency{
						Filename: filepath.Base(m.TargetPath),
						Version:  "1.0",
					})
				}
			}

			manifest.FilesMetadata = append(manifest.FilesMetadata, models.FileMetadata{
				Type:         fileType,
				Filename:     target,
				Compiler:     compiler,
				BinarySha256: &hashStr,
				Dependencies: deps,
			})
		}
	}

	return manifest, nil
}

func enrichManifestDefaults(ctx context.Context, problemsRepo interfaces.ProblemsRepo, problemID uuid.UUID, manifest *models.ProblemManifest, format string) {
	problemTitle := ""
	if problem, err := problemsRepo.GetProblemById(ctx, problemID); err == nil {
		problemTitle = strings.TrimSpace(problem.Title)
	}

	if strings.TrimSpace(manifest.Statement.Title) == "" {
		if problemTitle != "" {
			manifest.Statement.Title = problemTitle
		} else {
			manifest.Statement.Title = "Imported problem"
		}
	}

	if strings.TrimSpace(manifest.Statement.Legend) == "" {
		manifest.Statement.Legend = fmt.Sprintf("Imported from %s package.", format)
	}
}

func extractZip(r *zip.Reader, destDir string) error {
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "../") || strings.Contains(f.Name, "..\\") {
			return fmt.Errorf("invalid file path in zip: %s", f.Name)
		}

		path := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(path)
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}

		out.Close()
		rc.Close()
	}

	return nil
}

func detectPackageRoot(extractedDir string) (string, error) {
	if hasPackageMarker(extractedDir) {
		return extractedDir, nil
	}

	entries, err := os.ReadDir(extractedDir)
	if err != nil {
		return "", err
	}

	filteredDirs := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "__MACOSX" {
			continue
		}
		filteredDirs = append(filteredDirs, filepath.Join(extractedDir, name))
	}

	if len(filteredDirs) == 1 && hasPackageMarker(filteredDirs[0]) {
		return filteredDirs[0], nil
	}

	best := ""
	bestDepth := int(^uint(0) >> 1)

	walkErr := filepath.WalkDir(extractedDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || !d.IsDir() {
			return nil
		}

		if filepath.Base(path) == "__MACOSX" {
			return filepath.SkipDir
		}

		if !hasPackageMarker(path) {
			return nil
		}

		rel, relErr := filepath.Rel(extractedDir, path)
		if relErr != nil {
			return nil
		}
		if rel == "." {
			best = path
			bestDepth = 0
			return filepath.SkipAll
		}

		depth := strings.Count(filepath.ToSlash(rel), "/") + 1
		if depth < bestDepth || (depth == bestDepth && path < best) {
			best = path
			bestDepth = depth
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}

	if best != "" {
		return best, nil
	}

	return extractedDir, nil
}

func hasPackageMarker(dir string) bool {
	markers := []string{
		"problem.xml",
		"problem.yaml",
	}

	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}

	return false
}
