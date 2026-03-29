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
	"github.com/gate149/gate/backend/pkg/parsers"
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

	packageRoot, err := detectPackageRoot(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to detect package root: %w", err)
	}

	manifest, testsMetadata, format, err := loadPackageMetadata(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load package metadata: %w", err)
	}

	enrichManifestDefaults(ctx, uc.problemsRepo, problemID, manifest, format)
	if format != "native" {
		normalizeTestsOrdinals(testsMetadata)
	}

	if err := problemformat.ValidateManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	manifest.LastUpdated = time.Now()

	workspaceDir := filepath.Join(tempDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace dir: %w", err)
	}

	if err := problemformat.SaveTestsMetadata(workspaceDir, testsMetadata); err != nil {
		return nil, fmt.Errorf("failed to save tests metadata: %w", err)
	}

	if err := copyTestFiles(packageRoot, workspaceDir, format, testsMetadata); err != nil {
		return nil, fmt.Errorf("failed to copy test files: %w", err)
	}

	if err := copyExecutableFiles(packageRoot, workspaceDir, manifest); err != nil {
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
		"manifest.json",
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

func loadPackageMetadata(packageDir string) (*problemformat.ProblemManifest, *problemformat.TestsMetadata, string, error) {
	nativeManifest, nativeManifestErr := problemformat.LoadManifest(packageDir)
	nativeTests, nativeTestsErr := problemformat.LoadTestsMetadata(packageDir)
	if nativeManifestErr == nil && nativeTestsErr == nil {
		return nativeManifest, nativeTests, "native", nil
	}

	manifest, testsMetadata, format, parseErr := parsers.ParsePackage(packageDir)
	if parseErr != nil {
		return nil, nil, "", fmt.Errorf(
			"native load failed (manifest: %v, tests: %v), parser fallback failed: %w",
			nativeManifestErr,
			nativeTestsErr,
			parseErr,
		)
	}

	return manifest, testsMetadata, format, nil
}

func enrichManifestDefaults(ctx context.Context, problemsRepo interfaces.ProblemsRepo, problemID uuid.UUID, manifest *problemformat.ProblemManifest, format string) {
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

func normalizeTestsOrdinals(testsMetadata *problemformat.TestsMetadata) {
	for i := range testsMetadata.Tests {
		testsMetadata.Tests[i].Ordinal = i + 1
	}
}

func copyTestFiles(sourceDir, targetDir, format string, testsMetadata *problemformat.TestsMetadata) error {
	if testsMetadata == nil {
		return fmt.Errorf("tests metadata is nil")
	}

	if err := os.MkdirAll(filepath.Join(targetDir, "tests"), 0o755); err != nil {
		return err
	}

	switch format {
	case "polygon":
		return copyPolygonTestFiles(sourceDir, targetDir, len(testsMetadata.Tests))
	case "icpc":
		return copyICPCTestFiles(sourceDir, targetDir, len(testsMetadata.Tests))
	case "native":
		return copyNativeTestFiles(sourceDir, targetDir, testsMetadata)
	default:
		return fmt.Errorf("unsupported package format: %s", format)
	}
}

func copyNativeTestFiles(sourceDir, targetDir string, testsMetadata *problemformat.TestsMetadata) error {
	testsDir := filepath.Join(sourceDir, "tests")

	for _, test := range testsMetadata.Tests {
		inputPath, err := findNativeInputPath(testsDir, test.Ordinal)
		if err != nil {
			return err
		}

		outputPath, err := findNativeOutputPath(testsDir, test.Ordinal)
		if err != nil {
			return err
		}

		targetInputPath := filepath.Join(targetDir, "tests", fmt.Sprintf("%02d.in", test.Ordinal))
		targetOutputPath := filepath.Join(targetDir, "tests", fmt.Sprintf("%02d.out", test.Ordinal))

		if err := copyFileContents(inputPath, targetInputPath); err != nil {
			return err
		}
		if err := copyFileContents(outputPath, targetOutputPath); err != nil {
			return err
		}
	}

	return nil
}

func copyPolygonTestFiles(sourceDir, targetDir string, expectedCount int) error {
	pairs, err := collectIndexedTestPairs(filepath.Join(sourceDir, "data", "secret"))
	if err != nil {
		return err
	}

	if len(pairs) < expectedCount {
		return fmt.Errorf("polygon package has %d test pairs, expected at least %d", len(pairs), expectedCount)
	}

	for i := 0; i < expectedCount; i++ {
		targetOrdinal := i + 1
		targetInputPath := filepath.Join(targetDir, "tests", fmt.Sprintf("%02d.in", targetOrdinal))
		targetOutputPath := filepath.Join(targetDir, "tests", fmt.Sprintf("%02d.out", targetOrdinal))

		if err := copyFileContents(pairs[i].inputPath, targetInputPath); err != nil {
			return err
		}
		if err := copyFileContents(pairs[i].outputPath, targetOutputPath); err != nil {
			return err
		}
	}

	return nil
}

func copyICPCTestFiles(sourceDir, targetDir string, expectedCount int) error {
	samplePairs, err := collectNamedTestPairs(filepath.Join(sourceDir, "data", "sample"))
	if err != nil {
		return err
	}

	secretPairs, err := collectNamedTestPairs(filepath.Join(sourceDir, "data", "secret"))
	if err != nil {
		return err
	}

	pairs := append(samplePairs, secretPairs...)
	if len(pairs) < expectedCount {
		return fmt.Errorf("icpc package has %d test pairs, expected at least %d", len(pairs), expectedCount)
	}

	for i := 0; i < expectedCount; i++ {
		targetOrdinal := i + 1
		targetInputPath := filepath.Join(targetDir, "tests", fmt.Sprintf("%02d.in", targetOrdinal))
		targetOutputPath := filepath.Join(targetDir, "tests", fmt.Sprintf("%02d.out", targetOrdinal))

		if err := copyFileContents(pairs[i].inputPath, targetInputPath); err != nil {
			return err
		}
		if err := copyFileContents(pairs[i].outputPath, targetOutputPath); err != nil {
			return err
		}
	}

	return nil
}

func findNativeInputPath(testsDir string, ordinal int) (string, error) {
	candidates := []string{
		fmt.Sprintf("%02d.in", ordinal),
		fmt.Sprintf("%d.in", ordinal),
	}

	for _, candidate := range candidates {
		path := filepath.Join(testsDir, candidate)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("input file for test %d not found", ordinal)
}

func findNativeOutputPath(testsDir string, ordinal int) (string, error) {
	candidates := []string{
		fmt.Sprintf("%02d.out", ordinal),
		fmt.Sprintf("%02d.ans", ordinal),
		fmt.Sprintf("%d.out", ordinal),
		fmt.Sprintf("%d.ans", ordinal),
	}

	for _, candidate := range candidates {
		path := filepath.Join(testsDir, candidate)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("output file for test %d not found", ordinal)
}

type testPair struct {
	inputPath  string
	outputPath string
}

func collectIndexedTestPairs(dir string) ([]testPair, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type pairInfo struct {
		inputPath  string
		outputPath string
	}

	pairMap := make(map[string]*pairInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		match := testFileRegexp.FindStringSubmatch(name)
		if len(match) != 3 {
			continue
		}

		number := match[1]
		ext := match[2]

		pair := pairMap[number]
		if pair == nil {
			pair = &pairInfo{}
			pairMap[number] = pair
		}

		fullPath := filepath.Join(dir, name)
		if ext == "in" {
			pair.inputPath = fullPath
		} else if ext == "out" {
			pair.outputPath = fullPath
		} else if ext == "ans" && pair.outputPath == "" {
			pair.outputPath = fullPath
		}
	}

	keys := make([]string, 0, len(pairMap))
	for number := range pairMap {
		if pairMap[number].inputPath != "" && pairMap[number].outputPath != "" {
			keys = append(keys, number)
		}
	}
	sort.Strings(keys)

	pairs := make([]testPair, 0, len(keys))
	for _, number := range keys {
		pair := pairMap[number]
		pairs = append(pairs, testPair{inputPath: pair.inputPath, outputPath: pair.outputPath})
	}

	return pairs, nil
}

func collectNamedTestPairs(dir string) ([]testPair, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []testPair{}, nil
		}
		return nil, err
	}

	type pairInfo struct {
		inputPath  string
		outputPath string
	}

	pairMap := make(map[string]*pairInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		if stem == "" {
			continue
		}

		pair := pairMap[stem]
		if pair == nil {
			pair = &pairInfo{}
			pairMap[stem] = pair
		}

		fullPath := filepath.Join(dir, name)
		switch ext {
		case ".in":
			pair.inputPath = fullPath
		case ".out":
			pair.outputPath = fullPath
		case ".ans":
			if pair.outputPath == "" {
				pair.outputPath = fullPath
			}
		}
	}

	stems := make([]string, 0, len(pairMap))
	for stem := range pairMap {
		if pairMap[stem].inputPath != "" && pairMap[stem].outputPath != "" {
			stems = append(stems, stem)
		}
	}
	sort.Strings(stems)

	pairs := make([]testPair, 0, len(stems))
	for _, stem := range stems {
		pair := pairMap[stem]
		pairs = append(pairs, testPair{inputPath: pair.inputPath, outputPath: pair.outputPath})
	}

	return pairs, nil
}

func copyFileContents(sourcePath, targetPath string) error {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return err
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
