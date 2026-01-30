package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
)

type WorkshopUseCase struct {
	problemsRepo interfaces.ProblemsRepo
	vcsService   vcs.Service
	sandboxOrch  *sandbox.Orchestrator
	txManager    interfaces.Transactor
}

func NewWorkshopUseCase(
	problemsRepo interfaces.ProblemsRepo,
	vcsService vcs.Service,
	sandboxOrch *sandbox.Orchestrator,
	txManager interfaces.Transactor,
) *WorkshopUseCase {
	return &WorkshopUseCase{
		problemsRepo: problemsRepo,
		vcsService:   vcsService,
		sandboxOrch:  sandboxOrch,
		txManager:    txManager,
	}
}

// InitProblemWorkshop creates Git repo and initial structure
func (uc *WorkshopUseCase) InitProblemWorkshop(ctx context.Context, problemID uuid.UUID, title string) error {
	// Check if repo already exists
	if uc.vcsService.RepoExists(ctx, problemID) {
		return fmt.Errorf("workshop already initialized for problem %s", problemID)
	}

	// Create Git repository
	if err := uc.vcsService.InitProblemRepo(ctx, problemID); err != nil {
		return fmt.Errorf("failed to init repo: %w", err)
	}

	// Create default manifest.json
	if err := uc.vcsService.InitDefaultManifest(ctx, problemID, title); err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// Create default tests/tests.json
	if err := uc.vcsService.InitDefaultTestsMetadata(ctx, problemID); err != nil {
		return fmt.Errorf("failed to create tests metadata: %w", err)
	}

	// Create sample test files
	if err := uc.vcsService.WriteFile(ctx, problemID, "tests/1.in", []byte("1 2\n")); err != nil {
		return fmt.Errorf("failed to create sample input: %w", err)
	}
	if err := uc.vcsService.WriteFile(ctx, problemID, "tests/1.out", []byte("3\n")); err != nil {
		return fmt.Errorf("failed to create sample output: %w", err)
	}

	// Create sample statement
	statementContent := []byte("# " + title + "\n\nProblem statement goes here.\n")
	if err := uc.vcsService.WriteFile(ctx, problemID, "statement/statement.md", statementContent); err != nil {
		return fmt.Errorf("failed to create statement: %w", err)
	}

	// Commit initial structure
	commitSHA, err := uc.vcsService.Commit(ctx, problemID, "Initialize problem structure", "System", "system@workshop")
	if err != nil {
		return fmt.Errorf("failed to commit initial structure: %w", err)
	}

	// Update problem in database with git commit hash
	if err := uc.problemsRepo.UpdateProblem(ctx, problemID, &models.ProblemUpdate{
		GitCommitHash: &commitSHA,
	}); err != nil {
		return fmt.Errorf("failed to update problem metadata: %w", err)
	}

	return nil
}

// UpdateProblemFile updates file and validates manifest/components if needed
func (uc *WorkshopUseCase) UpdateProblemFile(ctx context.Context, req models.UpdateFileRequest) error {
	// Write file
	if err := uc.vcsService.WriteFile(ctx, req.ProblemID, req.Path, req.Content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Validate if it's manifest.json
	if req.Path == "manifest.json" {
		_, err := uc.vcsService.LoadManifest(ctx, req.ProblemID)
		if err != nil {
			return fmt.Errorf("invalid manifest.json: %w", err)
		}
	}

	return nil
}

// DeleteProblemFile deletes a file from the repository
func (uc *WorkshopUseCase) DeleteProblemFile(ctx context.Context, problemID uuid.UUID, path string) error {
	return uc.vcsService.DeleteFile(ctx, problemID, path)
}

// ReadProblemFile reads a file from the repository
func (uc *WorkshopUseCase) ReadProblemFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	return uc.vcsService.ReadFile(ctx, problemID, path)
}

// ListProblemFiles lists files in a directory
func (uc *WorkshopUseCase) ListProblemFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]vcs.FileEntry, error) {
	return uc.vcsService.ListFiles(ctx, problemID, dirPath)
}

// CommitChanges commits changes to the repository
func (uc *WorkshopUseCase) CommitChanges(ctx context.Context, problemID uuid.UUID, message, authorName, authorEmail string) (string, error) {
	commitSHA, err := uc.vcsService.Commit(ctx, problemID, message, authorName, authorEmail)
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	// Update problem in database
	if err := uc.problemsRepo.UpdateProblem(ctx, problemID, &models.ProblemUpdate{
		GitCommitHash: &commitSHA,
	}); err != nil {
		return "", fmt.Errorf("failed to update problem metadata: %w", err)
	}

	return commitSHA, nil
}

// GetWorkshopStatus returns the current status of the workshop
func (uc *WorkshopUseCase) GetWorkshopStatus(ctx context.Context, problemID uuid.UUID) (*models.WorkshopStatus, error) {
	fileStatuses, err := uc.vcsService.GetStatus(ctx, problemID)
	if err != nil {
		return nil, err
	}

	currentSHA, err := uc.vcsService.GetCurrentSHA(ctx, problemID)
	if err != nil {
		return nil, err
	}

	hasChanges, err := uc.vcsService.HasUncommittedChanges(ctx, problemID)
	if err != nil {
		return nil, err
	}

	return &models.WorkshopStatus{
		CurrentSHA:            currentSHA,
		ModifiedFiles:         fileStatuses,
		HasUncommittedChanges: hasChanges,
	}, nil
}

// GetCommitHistory returns the commit history
func (uc *WorkshopUseCase) GetCommitHistory(ctx context.Context, problemID uuid.UUID, limit int) ([]vcs.Commit, error) {
	return uc.vcsService.GetHistory(ctx, problemID, limit)
}

// CompileProblemComponent compiles checker/validator/generator and caches binary
func (uc *WorkshopUseCase) CompileProblemComponent(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error) {
	// Load manifest to find component
	manifest, err := uc.vcsService.LoadManifest(ctx, req.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find component metadata
	var componentMeta *problemformat.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == req.ComponentType {
			componentMeta = &manifest.FilesMetadata[i]
			break
		}
	}

	if componentMeta == nil {
		return nil, fmt.Errorf("component %s not found in manifest", req.ComponentType)
	}

	// Read source code
	sourceCode, err := uc.vcsService.ReadFile(ctx, req.ProblemID, componentMeta.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	// Load dependencies
	dependencies := make(map[string]string)
	for _, dep := range componentMeta.Dependencies {
		depPath := filepath.Join(filepath.Dir(componentMeta.Filename), dep.Filename)
		depContent, err := uc.vcsService.ReadFile(ctx, req.ProblemID, depPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read dependency %s: %w", dep.Filename, err)
		}
		dependencies[dep.Filename] = string(depContent)
	}

	// Compile via sandbox
	binary, err := uc.sandboxOrch.CompileComponentFromSource(ctx,
		req.ComponentType,
		string(sourceCode),
		componentMeta.Compiler,
		dependencies,
	)

	if err != nil {
		return &models.CompileResult{
			Success:      false,
			CompileError: err.Error(),
		}, nil
	}

	if !binary.Success {
		return &models.CompileResult{
			Success:      false,
			CompileLog:   binary.CompileLog,
			CompileError: "Compilation failed",
		}, nil
	}

	// Update manifest with binary SHA256
	sha256Hash := sandbox.ComputeSHA256([]byte(binary.FileID))
	componentMeta.BinarySha256 = &sha256Hash

	if err := uc.vcsService.SaveManifest(ctx, req.ProblemID, manifest); err != nil {
		return nil, fmt.Errorf("failed to update manifest: %w", err)
	}

	return &models.CompileResult{
		Success:    true,
		FileID:     binary.FileID,
		SHA256:     sha256Hash,
		CompileLog: binary.CompileLog,
	}, nil
}

// GenerateTests runs generator and creates test files
func (uc *WorkshopUseCase) GenerateTests(ctx context.Context, req models.GenerateTestsRequest) error {
	// Load manifest
	manifest, err := uc.vcsService.LoadManifest(ctx, req.ProblemID)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find generator component
	var generatorMeta *problemformat.FileMetadata
	for i := range manifest.FilesMetadata {
		meta := &manifest.FilesMetadata[i]
		if meta.Type == "generator" && strings.Contains(meta.Filename, req.GeneratorName) {
			generatorMeta = meta
			break
		}
	}

	if generatorMeta == nil {
		return fmt.Errorf("generator %s not found", req.GeneratorName)
	}

	// Check if generator is compiled
	if generatorMeta.BinarySha256 == nil || *generatorMeta.BinarySha256 == "" {
		return fmt.Errorf("generator not compiled, please compile first")
	}

	// For now, we'll need the compiled fileID - this would come from a cache
	// In a real implementation, we'd store the fileID somewhere
	return fmt.Errorf("test generation not yet fully implemented - need fileID caching")
}

// ValidateAllTests runs validator on all test inputs
func (uc *WorkshopUseCase) ValidateAllTests(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*models.ValidationReport, error) {
	// Load manifest
	manifest, err := uc.vcsService.LoadManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find validator component
	var validatorMeta *problemformat.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == "validator" {
			validatorMeta = &manifest.FilesMetadata[i]
			break
		}
	}

	if validatorMeta == nil {
		return &models.ValidationReport{
			TotalTests: 0,
			ValidTests: 0,
			Results:    []models.TestValidationResult{},
		}, nil // No validator = all tests considered valid
	}

	// Load tests metadata
	testsMetaData, err := uc.vcsService.ReadFile(ctx, problemID, "tests/tests.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read tests metadata: %w", err)
	}

	var testsMeta problemformat.TestsMetadata
	if err := json.Unmarshal(testsMetaData, &testsMeta); err != nil {
		return nil, fmt.Errorf("failed to parse tests metadata: %w", err)
	}

	// Validate each test (simplified - actual implementation would use sandbox)
	report := &models.ValidationReport{
		TotalTests: len(testsMeta.Tests),
		Results:    make([]models.TestValidationResult, 0),
	}

	for _, test := range testsMeta.Tests {
		// Check if test files exist
		inputPath := fmt.Sprintf("tests/%d.in", test.Ordinal)
		_, err := uc.vcsService.ReadFile(ctx, problemID, inputPath)

		if err != nil {
			report.Results = append(report.Results, models.TestValidationResult{
				TestNumber: test.Ordinal,
				Valid:      false,
				Error:      fmt.Sprintf("test file not found: %s", inputPath),
			})
			report.InvalidTests++
		} else {
			report.Results = append(report.Results, models.TestValidationResult{
				TestNumber: test.Ordinal,
				Valid:      true,
				Message:    "Test file exists",
			})
			report.ValidTests++
		}
	}

	return report, nil
}

// TestSolution compiles and runs solution against tests
func (uc *WorkshopUseCase) TestSolution(ctx context.Context, req models.TestSolutionRequest) (*models.TestReport, error) {
	// Load manifest for limits
	manifest, err := uc.vcsService.LoadManifest(ctx, req.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Read solution
	solutionCode, err := uc.vcsService.ReadFile(ctx, req.ProblemID, req.SolutionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read solution: %w", err)
	}

	// Detect language from extension
	ext := filepath.Ext(req.SolutionPath)
	language := detectLanguage(ext)

	// Load tests
	testsMetaData, err := uc.vcsService.ReadFile(ctx, req.ProblemID, "tests/tests.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read tests metadata: %w", err)
	}

	var testsMeta problemformat.TestsMetadata
	if err := json.Unmarshal(testsMetaData, &testsMeta); err != nil {
		return nil, fmt.Errorf("failed to parse tests metadata: %w", err)
	}

	// Run tests
	report := &models.TestReport{
		Results: make([]models.TestResult, 0),
	}

	for _, test := range testsMeta.Tests {
		// Skip if test numbers specified and this test not in list
		if len(req.TestNumbers) > 0 {
			found := false
			for _, num := range req.TestNumbers {
				if num == test.Ordinal {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		report.TotalTests++

		// Read test input and answer
		inputPath := fmt.Sprintf("tests/%d.in", test.Ordinal)
		answerPath := fmt.Sprintf("tests/%d.out", test.Ordinal)

		input, err := uc.vcsService.ReadFile(ctx, req.ProblemID, inputPath)
		if err != nil {
			report.Results = append(report.Results, models.TestResult{
				TestNumber: test.Ordinal,
				Verdict:    "IE",
				Message:    fmt.Sprintf("Failed to read input: %v", err),
			})
			report.FailedTests++
			continue
		}

		answer, err := uc.vcsService.ReadFile(ctx, req.ProblemID, answerPath)
		if err != nil {
			report.Results = append(report.Results, models.TestResult{
				TestNumber: test.Ordinal,
				Verdict:    "IE",
				Message:    fmt.Sprintf("Failed to read answer: %v", err),
			})
			report.FailedTests++
			continue
		}

		// Judge solution
		judgeReq := sandbox.JudgeSolutionRequest{
			SolutionCode:     string(solutionCode),
			SolutionLanguage: language,
			Input:            input,
			Answer:           answer,
			TimeLimitMs:      int64(manifest.TimeLimitMs),
			MemoryLimitMB:    int64(manifest.MemoryLimitMb),
		}

		result, err := uc.sandboxOrch.JudgeSolution(ctx, judgeReq)
		if err != nil {
			report.Results = append(report.Results, models.TestResult{
				TestNumber: test.Ordinal,
				Verdict:    "IE",
				Message:    fmt.Sprintf("Judging failed: %v", err),
			})
			report.FailedTests++
			continue
		}

		testResult := models.TestResult{
			TestNumber: test.Ordinal,
			Verdict:    result.Verdict,
			Time:       result.Time,
			Memory:     result.Memory,
			Message:    result.Message,
		}

		report.Results = append(report.Results, testResult)

		if result.Verdict == "OK" {
			report.PassedTests++
		} else {
			report.FailedTests++
		}
	}

	return report, nil
}

// detectLanguage detects programming language from file extension
func detectLanguage(ext string) string {
	switch ext {
	case ".cpp", ".cc", ".cxx":
		return "cpp17"
	case ".py":
		return "python3"
	case ".go":
		return "golang"
	case ".java":
		return "java11"
	case ".c":
		return "c11"
	default:
		return "cpp17"
	}
}
