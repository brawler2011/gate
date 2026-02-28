package problemformat

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gate149/gate/backend/pkg"
)

// ProblemPackage represents a loaded problem package
type ProblemPackage struct {
	Manifest      ProblemManifest
	TestsMetadata TestsMetadata
	TestCases     []LoadedTestCase
	Components    map[string]ComponentFile // key: filename (e.g., "checker.cpp")
	TempDir       string                   // temporary extraction directory
}

// LoadedTestCase contains test input and output
type LoadedTestCase struct {
	Ordinal int
	Input   []byte
	Output  []byte
}

// ComponentFile contains source code of a component
type ComponentFile struct {
	Filename     string
	Type         string // "checker", "validator", "generator", "interactor"
	SourceCode   string
	Language     string
	Dependencies map[string]string // filename -> content
}

// PackageLoader handles downloading and extracting problem packages
type PackageLoader struct {
	s3Client *pkg.S3Client
	bucket   string
	cacheDir string // optional: for caching downloaded packages
}

// NewPackageLoader creates a new package loader
func NewPackageLoader(s3Client *pkg.S3Client, bucket string, cacheDir string) *PackageLoader {
	return &PackageLoader{
		s3Client: s3Client,
		bucket:   bucket,
		cacheDir: cacheDir,
	}
}

// LoadPackage downloads and extracts a problem package
func (pl *PackageLoader) LoadPackage(ctx context.Context, problemID, version string) (*ProblemPackage, error) {
	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp(pl.cacheDir, fmt.Sprintf("problem-%s-%s-*", problemID, version))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Download package from S3
	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID, version)
	file, err := pl.s3Client.DownloadFile(ctx, pl.bucket, s3Key, nil)
	if file == nil || err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to download package: %w", err)
	}
	reader := file.Body
	defer reader.Close()

	// Save to temporary file
	zipPath := filepath.Join(tempDir, "package.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to create zip file: %w", err)
	}

	_, err = io.Copy(zipFile, reader)
	zipFile.Close()
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to write zip file: %w", err)
	}

	// Extract ZIP
	extractDir := filepath.Join(tempDir, "extracted")
	if err := extractZip(zipPath, extractDir); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to extract zip: %w", err)
	}

	// Load package contents
	pkg, err := pl.loadPackageContents(extractDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to load package contents: %w", err)
	}

	pkg.TempDir = tempDir
	return pkg, nil
}

// loadPackageContents reads manifest, tests, and components
func (pl *PackageLoader) loadPackageContents(extractDir string) (*ProblemPackage, error) {
	pkg := &ProblemPackage{
		Components: make(map[string]ComponentFile),
	}

	// Read manifest.json
	manifestPath := filepath.Join(extractDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}
	if err := json.Unmarshal(manifestData, &pkg.Manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	// Read tests/tests.json
	testsMetaPath := filepath.Join(extractDir, "tests", "tests.json")
	testsMetaData, err := os.ReadFile(testsMetaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tests/tests.json: %w", err)
	}
	if err := json.Unmarshal(testsMetaData, &pkg.TestsMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse tests/tests.json: %w", err)
	}

	// Load test cases
	testsDir := filepath.Join(extractDir, "tests")
	for _, test := range pkg.TestsMetadata.Tests {
		inputPath := filepath.Join(testsDir, fmt.Sprintf("%02d.in", test.Ordinal))
		outputPath := filepath.Join(testsDir, fmt.Sprintf("%02d.out", test.Ordinal))

		input, err := os.ReadFile(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read test %02d input: %w", test.Ordinal, err)
		}

		output, err := os.ReadFile(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read test %02d output: %w", test.Ordinal, err)
		}

		pkg.TestCases = append(pkg.TestCases, LoadedTestCase{
			Ordinal: test.Ordinal,
			Input:   input,
			Output:  output,
		})
	}

	// Load component files (checker, validator, generator, interactor)
	for _, meta := range pkg.Manifest.FilesMetadata {
		componentPath := filepath.Join(extractDir, meta.Filename)
		sourceCode, err := os.ReadFile(componentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read component %s: %w", meta.Filename, err)
		}

		// Load dependencies (e.g., testlib.h)
		deps := make(map[string]string)
		for _, dep := range meta.Dependencies {
			depPath := filepath.Join(extractDir, dep.Filename)
			depContent, err := os.ReadFile(depPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read dependency %s: %w", dep.Filename, err)
			}
			deps[dep.Filename] = string(depContent)
		}

		pkg.Components[meta.Filename] = ComponentFile{
			Filename:     meta.Filename,
			Type:         meta.Type,
			SourceCode:   string(sourceCode),
			Language:     meta.Compiler,
			Dependencies: deps,
		}
	}

	return pkg, nil
}

// Cleanup removes temporary files
func (pp *ProblemPackage) Cleanup() error {
	if pp.TempDir != "" {
		return os.RemoveAll(pp.TempDir)
	}
	return nil
}

// GetComponent retrieves a component by type
func (pp *ProblemPackage) GetComponent(componentType string) (*ComponentFile, bool) {
	for _, comp := range pp.Components {
		if comp.Type == componentType {
			return &comp, true
		}
	}
	return nil, false
}

// extractZip extracts a ZIP file to a destination directory
func extractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, file := range reader.File {
		filePath := filepath.Join(destDir, file.Name)

		// Check for ZipSlip vulnerability
		if !filepath.HasPrefix(filePath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Extract file
		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	return nil
}
