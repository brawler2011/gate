package packagegen

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gate149/gate/backend/pkg/problemformat"
)

// Package represents an in-memory problem package
type Package struct {
	Manifest      *problemformat.ProblemManifest
	TestsMetadata *problemformat.TestsMetadata
	TestFiles     map[int]TestFile  // ordinal -> TestFile
	Executables   map[string][]byte // filename -> content
	Media         map[string][]byte // filename -> content
}

// TestFile represents a single test case
type TestFile struct {
	Input  []byte
	Output []byte
}

// BuildPackage creates an in-memory package from a problem directory
func BuildPackage(problemDir string) (*Package, error) {
	// Load manifest
	manifest, err := problemformat.LoadManifest(problemDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Load tests metadata
	testsMetadata, err := problemformat.LoadTestsMetadata(problemDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load tests metadata: %w", err)
	}

	// Load test files
	testFiles := make(map[int]TestFile)
	for _, test := range testsMetadata.Tests {
		input, output, err := problemformat.LoadTestData(problemDir, test.Ordinal)
		if err != nil {
			return nil, fmt.Errorf("failed to load test %02d: %w", test.Ordinal, err)
		}
		testFiles[test.Ordinal] = TestFile{
			Input:  input,
			Output: output,
		}
	}

	// Load executable files
	executables := make(map[string][]byte)
	for _, fileMeta := range manifest.FilesMetadata {
		filePath := filepath.Join(problemDir, fileMeta.Filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			// Log warning but continue
			continue
		}
		executables[fileMeta.Filename] = content
	}

	// Load media files
	media := make(map[string][]byte)
	mediaDir := filepath.Join(problemDir, "media")
	if err := loadMediaFiles(mediaDir, "", media); err == nil {
		// Media loaded successfully
	}

	return &Package{
		Manifest:      manifest,
		TestsMetadata: testsMetadata,
		TestFiles:     testFiles,
		Executables:   executables,
		Media:         media,
	}, nil
}

func loadMediaFiles(dir, prefix string, media map[string][]byte) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		relPath := filepath.Join(prefix, entry.Name())

		if entry.IsDir() {
			loadMediaFiles(path, relPath, media)
		} else {
			content, err := os.ReadFile(path)
			if err == nil {
				media[relPath] = content
			}
		}
	}

	return nil
}

// ValidatePackage validates the package structure
func ValidatePackage(pkg *Package) error {
	// Validate manifest
	if err := problemformat.ValidateManifest(pkg.Manifest); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	// Validate tests metadata
	if err := problemformat.ValidateTestsMetadata(pkg.TestsMetadata, pkg.Manifest); err != nil {
		return fmt.Errorf("invalid tests metadata: %w", err)
	}

	// Validate that all tests have files
	for _, test := range pkg.TestsMetadata.Tests {
		if _, ok := pkg.TestFiles[test.Ordinal]; !ok {
			return fmt.Errorf("missing test files for test %02d", test.Ordinal)
		}
	}

	// Validate that all executables exist
	for _, fileMeta := range pkg.Manifest.FilesMetadata {
		if _, ok := pkg.Executables[fileMeta.Filename]; !ok {
			return fmt.Errorf("missing executable file: %s", fileMeta.Filename)
		}
	}

	return nil
}

// WritePackageToZip creates a ZIP archive from the package
func WritePackageToZip(pkg *Package, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Write manifest.json
	manifestFile, err := zipWriter.Create("manifest.json")
	if err != nil {
		return err
	}
	manifestJSON, err := problemformat.MarshalManifest(pkg.Manifest)
	if err != nil {
		return err
	}
	if _, err := manifestFile.Write(manifestJSON); err != nil {
		return err
	}

	// Write tests/tests.json
	testsFile, err := zipWriter.Create("tests/tests.json")
	if err != nil {
		return err
	}
	testsJSON, err := problemformat.MarshalTestsMetadata(pkg.TestsMetadata)
	if err != nil {
		return err
	}
	if _, err := testsFile.Write(testsJSON); err != nil {
		return err
	}

	// Write test files
	for ordinal, testFile := range pkg.TestFiles {
		// Write input file
		inputFile, err := zipWriter.Create(fmt.Sprintf("tests/%02d.in", ordinal))
		if err != nil {
			return err
		}
		if _, err := inputFile.Write(testFile.Input); err != nil {
			return err
		}

		// Write output file
		outputFile, err := zipWriter.Create(fmt.Sprintf("tests/%02d.out", ordinal))
		if err != nil {
			return err
		}
		if _, err := outputFile.Write(testFile.Output); err != nil {
			return err
		}
	}

	// Write executable files
	for filename, content := range pkg.Executables {
		execFile, err := zipWriter.Create(filename)
		if err != nil {
			return err
		}
		if _, err := execFile.Write(content); err != nil {
			return err
		}
	}

	// Write media files
	for filename, content := range pkg.Media {
		mediaFile, err := zipWriter.Create(filepath.Join("media", filename))
		if err != nil {
			return err
		}
		if _, err := mediaFile.Write(content); err != nil {
			return err
		}
	}

	return nil
}
