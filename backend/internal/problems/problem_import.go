package problems

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats"
	ziputil "github.com/brawler2011/gate/backend/pkg/ziputil"
	"github.com/google/uuid"
)

type ProblemImportUseCase struct {
	transactions     ProblemTransactions
	problemsRepo     ProblemsRepo
	workspaceStorage *WorkspaceStorage
}

func NewProblemImportUseCase(problemsRepo ProblemsRepo, workspaceStorage *WorkspaceStorage, transactions ProblemTransactions) *ProblemImportUseCase {
	return &ProblemImportUseCase{transactions: transactions,
		problemsRepo:     problemsRepo,
		workspaceStorage: workspaceStorage,
	}
}

func (uc *ProblemImportUseCase) importProblemPackageFiles(ctx context.Context, zipReader io.Reader, problemID uuid.UUID) (*problemsmodel.ProblemManifest, error) {
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
	if err := ziputil.ExtractZip(&r.Reader, tempSrc); err != nil {
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
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		return uc.workspaceStorage.WriteFile(ctx, problemID, rel, content)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload workspace files: %w", err)
	}

	workshop := newWorkshop(uc.workspaceStorage)
	manifest, err := workshop.loadManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to map import plan to manifest: %w", err)
	}

	enrichManifestDefaults(ctx, uc.problemsRepo, problemID, manifest)

	// Apply import-only defaults to the package itself, never a second manifest.
	plan.Problem.Title = manifest.Statement.Title
	if err := workshop.writeProblemConfig(ctx, problemID, plan.Problem); err != nil {
		return nil, err
	}

	return workshop.loadManifest(ctx, problemID)
}

func enrichManifestDefaults(ctx context.Context, problemsRepo ProblemsRepo, problemID uuid.UUID, manifest *problemsmodel.ProblemManifest) {
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

}

func detectPackageRoot(extractedDir string) (string, error) {
	if hasPackageMarker(extractedDir) {
		return extractedDir, nil
	}

	entries, err := os.ReadDir(extractedDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted directory %s: %w", extractedDir, err)
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
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
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
			return fmt.Errorf("failed to get relative path for %s: %w", path, relErr)
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
		return "", fmt.Errorf("failed to walk extracted directory %s: %w", extractedDir, walkErr)
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

func (uc *ProblemImportUseCase) ImportProblemPackage(ctx context.Context, zipReader io.Reader, zipSize int64, problemID uuid.UUID) (*problemsmodel.ProblemManifest, error) {
	if err := withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionEditProblem, func(ProblemTxStore) error { return nil }); err != nil {
		return nil, err
	}
	previous, err := uc.workspaceStorage.repo.GetWorkspaceHash(ctx, problemID)
	if err != nil {
		return nil, err
	}
	var result *problemsmodel.ProblemManifest
	version, err := uc.workspaceStorage.preparePackage(ctx, problemID, func(ctx context.Context) error {
		var err error
		result, err = uc.importProblemPackageFiles(ctx, zipReader, problemID)
		return err
	})
	if err != nil {
		return nil, err
	}
	err = withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionEditProblem, func(tx ProblemTxStore) error {
		return tx.CommitWorkspace(ctx, problemID, previous, version.hash, version.metadata)
	})
	if err != nil {
		return nil, err
	}
	result.Revision = version.hash
	return result, nil
}
