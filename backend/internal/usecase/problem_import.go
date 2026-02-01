package usecase

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gate149/gate/backend/pkg/parsers"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/google/uuid"
)

type ProblemImportUseCase struct {
	workshopReposDir string
}

func NewProblemImportUseCase(workshopReposDir string) *ProblemImportUseCase {
	return &ProblemImportUseCase{
		workshopReposDir: workshopReposDir,
	}
}

// ImportProblemPackage imports a problem package (ZIP) and converts it to unified format
func (uc *ProblemImportUseCase) ImportProblemPackage(
	ctx context.Context,
	zipReader io.ReaderAt,
	zipSize int64,
	problemID uuid.UUID,
) (string, error) {
	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "problem-import-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract ZIP
	err = extractZip(zipReader, zipSize, tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to extract ZIP: %w", err)
	}

	// Detect format and parse
	manifest, testsMetadata, format, err := parsers.ParsePackage(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to parse package: %w", err)
	}

	// Create problem directory in workshop
	problemDir := filepath.Join(uc.workshopReposDir, problemID.String())
	if err := os.MkdirAll(problemDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create problem directory: %w", err)
	}

	// Save manifest
	if err := problemformat.SaveManifest(problemDir, manifest); err != nil {
		return "", fmt.Errorf("failed to save manifest: %w", err)
	}

	// Save tests metadata
	if err := problemformat.SaveTestsMetadata(problemDir, testsMetadata); err != nil {
		return "", fmt.Errorf("failed to save tests metadata: %w", err)
	}

	// Copy test files
	err = copyTestFiles(tempDir, problemDir, format)
	if err != nil {
		return "", fmt.Errorf("failed to copy test files: %w", err)
	}

	// Copy executable files (checkers, validators, etc.)
	err = copyExecutableFiles(tempDir, problemDir, manifest.FilesMetadata)
	if err != nil {
		return "", fmt.Errorf("failed to copy executable files: %w", err)
	}

	return format, nil
}

func extractZip(reader io.ReaderAt, size int64, destDir string) error {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return fmt.Errorf("failed to open ZIP: %w", err)
	}

	for _, file := range zipReader.File {
		err := extractZipFile(file, destDir)
		if err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}

func extractZipFile(file *zip.File, destDir string) error {
	filePath := filepath.Join(destDir, file.Name)

	// Check for ZipSlip vulnerability
	if !filepath.HasPrefix(filePath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path: %s", file.Name)
	}

	if file.FileInfo().IsDir() {
		return os.MkdirAll(filePath, file.Mode())
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// Extract file
	destFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

func copyTestFiles(srcDir, destDir string, format string) error {
	testsDir := filepath.Join(destDir, "tests")
	if err := os.MkdirAll(testsDir, 0755); err != nil {
		return err
	}

	var srcTestsDir string
	if format == "polygon" {
		srcTestsDir = filepath.Join(srcDir, "tests")
	} else if format == "icpc" {
		// ICPC has data/sample and data/secret
		// Copy from both directories
		sampleDir := filepath.Join(srcDir, "data", "sample")
		secretDir := filepath.Join(srcDir, "data", "secret")

		testNum := 1
		// Copy sample tests
		if entries, err := os.ReadDir(sampleDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if filepath.Ext(name) == ".in" {
					baseName := name[:len(name)-3]
					// Copy .in file
					copyFile(filepath.Join(sampleDir, name), filepath.Join(testsDir, fmt.Sprintf("%02d.in", testNum)))
					// Copy .ans file as .out
					copyFile(filepath.Join(sampleDir, baseName+".ans"), filepath.Join(testsDir, fmt.Sprintf("%02d.out", testNum)))
					testNum++
				}
			}
		}

		// Copy secret tests
		if entries, err := os.ReadDir(secretDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if filepath.Ext(name) == ".in" {
					baseName := name[:len(name)-3]
					// Copy .in file
					copyFile(filepath.Join(secretDir, name), filepath.Join(testsDir, fmt.Sprintf("%02d.in", testNum)))
					// Copy .ans file as .out
					copyFile(filepath.Join(secretDir, baseName+".ans"), filepath.Join(testsDir, fmt.Sprintf("%02d.out", testNum)))
					testNum++
				}
			}
		}

		return nil
	}

	// Copy test files from source directory
	if _, err := os.Stat(srcTestsDir); os.IsNotExist(err) {
		return nil // No tests directory
	}

	entries, err := os.ReadDir(srcTestsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(srcTestsDir, entry.Name())
		destPath := filepath.Join(testsDir, entry.Name())
		if err := copyFile(srcPath, destPath); err != nil {
			return err
		}
	}

	return nil
}

func copyExecutableFiles(srcDir, destDir string, filesMetadata []problemformat.FileMetadata) error {
	for _, fileMeta := range filesMetadata {
		srcPath := filepath.Join(srcDir, fileMeta.Filename)
		destPath := filepath.Join(destDir, filepath.Base(fileMeta.Filename))

		if err := copyFile(srcPath, destPath); err != nil {
			// Log warning but don't fail
			continue
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
