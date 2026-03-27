package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

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

func (uc *WorkshopUseCase) IsInitialized(ctx context.Context, problemID uuid.UUID) bool {
	manifest, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
	return err == nil && len(manifest) > 0
}

func (uc *WorkshopUseCase) InitProblemWorkshop(ctx context.Context, problemID uuid.UUID, title string) error {
	if uc.IsInitialized(ctx, problemID) {
		return fmt.Errorf("workshop already initialized for problem %s", problemID)
	}

	defaultDirs := []string{"tests", "solutions", "checkers", "validators", "generators", "interactors", "media"}
	for _, dir := range defaultDirs {
		if err := uc.vcsService.CreateDirectory(ctx, problemID, dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := uc.vcsService.WriteFile(ctx, problemID, "README.md", []byte("# Problem\n\nThis is a problem workspace.\n")); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	testsMeta := defaultTestsMetadata()
	testsMetaBytes, err := json.MarshalIndent(testsMeta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default tests metadata: %w", err)
	}
	if err := uc.vcsService.WriteFile(ctx, problemID, "tests/tests.json", testsMetaBytes); err != nil {
		return fmt.Errorf("failed to create tests metadata: %w", err)
	}

	if err := uc.vcsService.WriteFile(ctx, problemID, "tests/01.in", []byte("1 2\n")); err != nil {
		return fmt.Errorf("failed to create sample input: %w", err)
	}
	if err := uc.vcsService.WriteFile(ctx, problemID, "tests/01.out", []byte("3\n")); err != nil {
		return fmt.Errorf("failed to create sample output: %w", err)
	}

	manifest := defaultManifest(title)
	if err := uc.saveManifest(ctx, problemID, manifest); err != nil {
		return fmt.Errorf("failed to save default manifest: %w", err)
	}

	return nil
}

func (uc *WorkshopUseCase) UpdateProblemFile(ctx context.Context, req models.UpdateFileRequest) error {
	if req.Path == "manifest.json" {
		var manifest problemformat.ProblemManifest
		if err := json.Unmarshal(req.Content, &manifest); err != nil {
			return fmt.Errorf("invalid manifest.json: %w", err)
		}
		if err := problemformat.ValidateManifest(&manifest); err != nil {
			return fmt.Errorf("invalid manifest.json: %w", err)
		}

		if err := uc.saveManifest(ctx, req.ProblemID, &manifest); err != nil {
			return fmt.Errorf("failed to update manifest: %w", err)
		}

		title := strings.TrimSpace(manifest.Statement.Title)
		if title != "" {
			problem, err := uc.problemsRepo.GetProblemById(ctx, req.ProblemID)
			if err != nil {
				return fmt.Errorf("failed to get problem for title sync: %w", err)
			}
			if strings.TrimSpace(problem.Title) != title {
				if err := uc.problemsRepo.UpdateProblem(ctx, req.ProblemID, &models.ProblemUpdate{Title: &title}); err != nil {
					return fmt.Errorf("failed to sync problem title: %w", err)
				}
			}
		}

		return nil
	}

	if err := uc.vcsService.WriteFile(ctx, req.ProblemID, req.Path, req.Content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (uc *WorkshopUseCase) DeleteProblemFile(ctx context.Context, problemID uuid.UUID, path string) error {
	if path == "manifest.json" {
		return fmt.Errorf("manifest.json cannot be deleted")
	}
	return uc.vcsService.DeleteFile(ctx, problemID, path)
}

func (uc *WorkshopUseCase) ReadProblemFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	if path == "manifest.json" {
		manifestBytes, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
		if err != nil {
			return nil, err
		}
		if len(manifestBytes) == 0 {
			return nil, fmt.Errorf("manifest.json not found")
		}
		return manifestBytes, nil
	}
	return uc.vcsService.ReadFile(ctx, problemID, path)
}

func (uc *WorkshopUseCase) ListProblemFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]vcs.FileEntry, error) {
	files, err := uc.vcsService.ListFiles(ctx, problemID, dirPath)
	if err != nil {
		return nil, err
	}

	if dirPath == "" || dirPath == "." {
		if uc.IsInitialized(ctx, problemID) {
			files = append(files, vcs.FileEntry{Path: "manifest.json", IsDirectory: false, Size: 0})
		}
	}

	return files, nil
}

func (uc *WorkshopUseCase) CompileProblemComponent(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error) {
	manifest, err := uc.loadManifest(ctx, req.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

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

	sourceCode, err := uc.vcsService.ReadFile(ctx, req.ProblemID, componentMeta.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	dependencies := make(map[string]string)
	for _, dep := range componentMeta.Dependencies {
		depPath := filepath.Join(filepath.Dir(componentMeta.Filename), dep.Filename)
		depContent, err := uc.vcsService.ReadFile(ctx, req.ProblemID, depPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read dependency %s: %w", dep.Filename, err)
		}
		dependencies[dep.Filename] = string(depContent)
	}

	binary, err := uc.sandboxOrch.CompileComponentFromSource(ctx,
		req.ComponentType,
		string(sourceCode),
		componentMeta.Compiler,
		dependencies,
	)
	if err != nil {
		return &models.CompileResult{Success: false, CompileError: err.Error()}, nil
	}

	if !binary.Success {
		return &models.CompileResult{
			Success:      false,
			CompileLog:   binary.CompileLog,
			CompileError: "Compilation failed",
		}, nil
	}

	sha256Hash := sandbox.ComputeSHA256([]byte(binary.FileID))
	componentMeta.BinarySha256 = &sha256Hash

	if err := uc.saveManifest(ctx, req.ProblemID, manifest); err != nil {
		return nil, fmt.Errorf("failed to update manifest: %w", err)
	}

	return &models.CompileResult{
		Success:    true,
		FileID:     binary.FileID,
		SHA256:     sha256Hash,
		CompileLog: binary.CompileLog,
	}, nil
}

func (uc *WorkshopUseCase) GenerateTests(ctx context.Context, req models.GenerateTestsRequest) error {
	manifest, err := uc.loadManifest(ctx, req.ProblemID)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

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

	if generatorMeta.BinarySha256 == nil || *generatorMeta.BinarySha256 == "" {
		return fmt.Errorf("generator not compiled, please compile first")
	}

	return fmt.Errorf("test generation not yet fully implemented - need fileID caching")
}

func (uc *WorkshopUseCase) ValidateAllTests(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*models.ValidationReport, error) {
	manifest, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	var validatorMeta *problemformat.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == "validator" {
			validatorMeta = &manifest.FilesMetadata[i]
			break
		}
	}

	if validatorMeta == nil {
		return &models.ValidationReport{TotalTests: 0, ValidTests: 0, Results: []models.TestValidationResult{}}, nil
	}

	testsMetaData, err := uc.vcsService.ReadFile(ctx, problemID, "tests/tests.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read tests metadata: %w", err)
	}

	var testsMeta problemformat.TestsMetadata
	if err := json.Unmarshal(testsMetaData, &testsMeta); err != nil {
		return nil, fmt.Errorf("failed to parse tests metadata: %w", err)
	}

	report := &models.ValidationReport{TotalTests: len(testsMeta.Tests), Results: make([]models.TestValidationResult, 0)}

	for _, test := range testsMeta.Tests {
		inputPath := fmt.Sprintf("tests/%02d.in", test.Ordinal)
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

func (uc *WorkshopUseCase) TestSolution(ctx context.Context, req models.TestSolutionRequest) (*models.TestReport, error) {
	manifest, err := uc.loadManifest(ctx, req.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	solutionCode, err := uc.vcsService.ReadFile(ctx, req.ProblemID, req.SolutionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read solution: %w", err)
	}

	ext := filepath.Ext(req.SolutionPath)
	language := detectLanguage(ext)

	testsMetaData, err := uc.vcsService.ReadFile(ctx, req.ProblemID, "tests/tests.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read tests metadata: %w", err)
	}

	var testsMeta problemformat.TestsMetadata
	if err := json.Unmarshal(testsMetaData, &testsMeta); err != nil {
		return nil, fmt.Errorf("failed to parse tests metadata: %w", err)
	}

	report := &models.TestReport{Results: make([]models.TestResult, 0)}

	for _, test := range testsMeta.Tests {
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

		inputPath := fmt.Sprintf("tests/%02d.in", test.Ordinal)
		answerPath := fmt.Sprintf("tests/%02d.out", test.Ordinal)

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

func (uc *WorkshopUseCase) loadManifest(ctx context.Context, problemID uuid.UUID) (*problemformat.ProblemManifest, error) {
	manifestBytes, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest from database: %w", err)
	}
	if len(manifestBytes) == 0 {
		return nil, fmt.Errorf("manifest.json not found")
	}

	var manifest problemformat.ProblemManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

func (uc *WorkshopUseCase) saveManifest(ctx context.Context, problemID uuid.UUID, manifest *problemformat.ProblemManifest) error {
	manifest.LastUpdated = time.Now()

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := uc.problemsRepo.UpdateProblemManifest(ctx, problemID, manifestBytes); err != nil {
		return fmt.Errorf("failed to save manifest to database: %w", err)
	}

	return nil
}

func defaultManifest(title string) *problemformat.ProblemManifest {
	return &problemformat.ProblemManifest{
		LastUpdated:     time.Now(),
		ProblemType:     "pass-fail",
		MaxScore:        nil,
		FilesMetadata:   []problemformat.FileMetadata{},
		TimeLimitMs:     1000,
		MemoryLimitMb:   256,
		StdoutLimitMb:   64,
		CodeSizeLimitKb: 256,
		Statement: problemformat.Statement{
			Title:        title,
			Legend:       "Problem description goes here.",
			InputFormat:  "Input format description.",
			OutputFormat: "Output format description.",
			Notes:        "",
			Interaction:  "",
			Scoring:      "",
		},
	}
}

func defaultTestsMetadata() *problemformat.TestsMetadata {
	return &problemformat.TestsMetadata{
		Groups: []problemformat.TestGroup{
			{
				Ordinal:      0,
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
}

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
