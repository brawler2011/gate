package usecase

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
)

var testFileRegexp = regexp.MustCompile(`^(\d+)\.(in|out|ans)$`)

type ProblemImportUseCase struct {
	problemsRepo interfaces.ProblemsRepo
	vcsService   vcs.Service
}

func NewProblemImportUseCase(problemsRepo interfaces.ProblemsRepo, vcsService vcs.Service) *ProblemImportUseCase {
	return &ProblemImportUseCase{problemsRepo: problemsRepo, vcsService: vcsService}
}

func (uc *ProblemImportUseCase) ImportProblemPackage(ctx context.Context, zipReader io.Reader, zipSize int64, problemID uuid.UUID) (*problemformat.ProblemPackage, error) {
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

	if err := extractZip(&r.Reader, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract zip: %w", err)
	}

	manifest, err := problemformat.LoadManifest(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	if err := problemformat.ValidateManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	testsMetadata, err := problemformat.LoadTestsMetadata(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load tests metadata: %w", err)
	}

	format := detectPackageFormat(tempDir)

	workspaceDir := filepath.Join(tempDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace dir: %w", err)
	}

	if err := problemformat.SaveTestsMetadata(workspaceDir, testsMetadata); err != nil {
		return nil, fmt.Errorf("failed to save tests metadata: %w", err)
	}

	if err := copyTestFiles(tempDir, workspaceDir, format); err != nil {
		return nil, fmt.Errorf("failed to copy test files: %w", err)
	}

	if err := copyExecutableFiles(tempDir, workspaceDir, manifest); err != nil {
		return nil, fmt.Errorf("failed to copy executable files: %w", err)
	}

	if err := uc.vcsService.DeleteProblemWorkspace(ctx, problemID); err != nil {
		return nil, fmt.Errorf("failed to clear existing workspace: %w", err)
	}

	for _, dir := range []string{"tests", "solutions", "checkers", "validators", "generators", "interactors", "media"} {
		if err := uc.vcsService.CreateDirectory(ctx, problemID, dir); err != nil {
			return nil, fmt.Errorf("failed to create workspace directory %s: %w", dir, err)
		}
	}

	err = filepath.WalkDir(workspaceDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(workspaceDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return uc.vcsService.WriteFile(ctx, problemID, rel, content)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload workspace files: %w", err)
	}

	manifest.LastUpdated = time.Now()
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := uc.problemsRepo.UpdateProblemManifest(ctx, problemID, manifestBytes); err != nil {
		return nil, fmt.Errorf("failed to save manifest to database: %w", err)
	}

	return &problemformat.ProblemPackage{Manifest: *manifest, TestsMetadata: *testsMetadata}, nil
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

func detectPackageFormat(extractedDir string) string {
	if _, err := os.Stat(filepath.Join(extractedDir, "data", "secret")); err == nil {
		return "polygon"
	}

	if _, err := os.Stat(filepath.Join(extractedDir, "tests", "tests.json")); err == nil {
		return "native"
	}

	return "unknown"
}

func copyTestFiles(sourceDir, targetDir, format string) error {
	var testFiles []string

	switch format {
	case "polygon":
		testsDir := filepath.Join(sourceDir, "data", "secret")
		entries, err := os.ReadDir(testsDir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if !entry.IsDir() && testFileRegexp.MatchString(entry.Name()) {
				testFiles = append(testFiles, entry.Name())
			}
		}

	case "native":
		testsDir := filepath.Join(sourceDir, "tests")
		entries, err := os.ReadDir(testsDir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if !entry.IsDir() && testFileRegexp.MatchString(entry.Name()) {
				testFiles = append(testFiles, entry.Name())
			}
		}

	default:
		return fmt.Errorf("unsupported package format: %s", format)
	}

	sort.Strings(testFiles)

	for _, fileName := range testFiles {
		var sourcePath string
		if format == "polygon" {
			sourcePath = filepath.Join(sourceDir, "data", "secret", fileName)
		} else {
			sourcePath = filepath.Join(sourceDir, "tests", fileName)
		}

		targetPath := filepath.Join(targetDir, "tests", fileName)

		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}

		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func copyExecutableFiles(sourceDir, targetDir string, manifest *problemformat.ProblemManifest) error {
	for _, fileMeta := range manifest.FilesMetadata {
		var sourcePath string
		targetPath := filepath.Join(targetDir, fileMeta.Filename)

		if _, err := os.Stat(filepath.Join(sourceDir, fileMeta.Filename)); err == nil {
			sourcePath = filepath.Join(sourceDir, fileMeta.Filename)
		} else {
			sourcePath = findFileByName(sourceDir, filepath.Base(fileMeta.Filename))
			if sourcePath == "" {
				return fmt.Errorf("could not find executable file: %s", fileMeta.Filename)
			}
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}

		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return err
		}

		for _, dep := range fileMeta.Dependencies {
			depSourcePath := findFileByName(sourceDir, dep.Filename)
			if depSourcePath == "" {
				return fmt.Errorf("could not find dependency file: %s", dep.Filename)
			}

			depTargetPath := filepath.Join(filepath.Dir(targetPath), dep.Filename)

			depContent, err := os.ReadFile(depSourcePath)
			if err != nil {
				return err
			}

			if err := os.WriteFile(depTargetPath, depContent, 0o644); err != nil {
				return err
			}
		}
	}

	return nil
}

func findFileByName(rootDir, fileName string) string {
	var foundPath string

	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == fileName {
			foundPath = path
			return fs.SkipAll
		}
		return nil
	})

	return foundPath
}
