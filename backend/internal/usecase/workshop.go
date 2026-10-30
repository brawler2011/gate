package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type WorkshopUseCase struct {
	problemsRepo     interfaces.ProblemsRepo
	workspaceStorage *WorkspaceStorage
	sandbox          *sandbox.Sandbox
	txManager        interfaces.Transactor
}

func NewWorkshopUseCase(
	problemsRepo interfaces.ProblemsRepo,
	workspaceStorage *WorkspaceStorage,
	sandbox *sandbox.Sandbox,
	txManager interfaces.Transactor,
) *WorkshopUseCase {
	return &WorkshopUseCase{
		problemsRepo:     problemsRepo,
		workspaceStorage: workspaceStorage,
		sandbox:          sandbox,
		txManager:        txManager,
	}
}

func (uc *WorkshopUseCase) IsInitialized(ctx context.Context, problemID uuid.UUID) bool {
	manifest, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
	return err == nil && len(manifest) > 0
}

func ParseStatementMarkdown(content string) models.Statement {
	var stmt models.Statement
	parts := strings.Split(content, "<!--")
	for _, part := range parts {
		if part == "" {
			continue
		}
		subparts := strings.SplitN(part, "-->", 2)
		if len(subparts) < 2 {
			continue
		}
		tag := strings.TrimSpace(subparts[0])
		tag = strings.ToLower(tag)
		body := strings.TrimSpace(subparts[1])
		switch tag {
		case "title":
			stmt.Title = body
		case "legend":
			stmt.Legend = body
		case "input":
			stmt.InputFormat = body
		case "output":
			stmt.OutputFormat = body
		case "notes":
			stmt.Notes = body
		case "interaction":
			stmt.Interaction = body
		case "scoring":
			stmt.Scoring = body
		}
	}
	return stmt
}

func RenderStatementMarkdown(stmt models.Statement) string {
	var sb strings.Builder
	if stmt.Title != "" {
		sb.WriteString("<!-- title -->\n")
		sb.WriteString(stmt.Title)
		sb.WriteString("\n\n")
	}
	sb.WriteString("<!--legend -->")
	if stmt.Legend != "" {
		sb.WriteString("\n\n")
		sb.WriteString(stmt.Legend)
	}
	if stmt.InputFormat != "" {
		sb.WriteString("\n\n<!-- input -->\n\n")
		sb.WriteString(stmt.InputFormat)
	}
	if stmt.OutputFormat != "" {
		sb.WriteString("\n\n<!-- output -->\n\n")
		sb.WriteString(stmt.OutputFormat)
	}
	if stmt.Interaction != "" {
		sb.WriteString("\n\n<!-- interaction -->\n\n")
		sb.WriteString(stmt.Interaction)
	}
	if stmt.Notes != "" {
		sb.WriteString("\n\n<!-- notes -->\n\n")
		sb.WriteString(stmt.Notes)
	}
	if stmt.Scoring != "" {
		sb.WriteString("\n\n<!-- scoring -->\n\n")
		sb.WriteString(stmt.Scoring)
	}
	sb.WriteString("\n")
	return sb.String()
}

func (uc *WorkshopUseCase) initProblemWorkshopUnlocked(ctx context.Context, problemID uuid.UUID, title string) error {
	existing, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
	if err != nil {
		return fmt.Errorf("check workshop initialization: %w", err)
	}
	if len(existing) > 0 {
		return fmt.Errorf("workshop already initialized for problem %s", problemID)
	}

	if _, err := uc.workspaceStorage.ReadFile(ctx, problemID, "problem.yaml"); err == nil {
		return fmt.Errorf("existing problem.yaml must be recovered or imported, not initialized")
	} else if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("check existing problem.yaml: %w", err)
	}

	manifest := defaultManifest(title)

	// Save default statement to statements/en.md
	stmtBytes := []byte(RenderStatementMarkdown(manifest.Statement))
	if err := uc.workspaceStorage.WriteFile(ctx, problemID, "statements/en.md", stmtBytes); err != nil {
		return fmt.Errorf("failed to create default statement: %w", err)
	}

	// Create a sample test file
	if err := uc.workspaceStorage.WriteFile(ctx, problemID, "tests/01.in", []byte("1 2\n")); err != nil {
		return fmt.Errorf("failed to create sample input: %w", err)
	}
	if err := uc.workspaceStorage.WriteFile(ctx, problemID, "tests/01.out", []byte("3\n")); err != nil {
		return fmt.Errorf("failed to create sample output: %w", err)
	}

	return uc.commitManifest(ctx, problemID, manifest, ManifestAndTestsToGfmtProblem(manifest, defaultTestsMetadata()))
}

func (uc *WorkshopUseCase) updateProblemFileUnlocked(ctx context.Context, req models.UpdateFileRequest) error {
	req.Path = filepath.ToSlash(filepath.Clean(req.Path))
	if req.Path == "problem.yaml" {
		return fmt.Errorf("cannot modify problem.yaml directly")
	}

	if err := uc.workspaceStorage.WriteFile(ctx, req.ProblemID, req.Path, req.Content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if req.Path == "statements/en.md" {
		manifest, err := uc.loadManifest(ctx, req.ProblemID)
		if err != nil {
			return err
		}
		manifest.Statement = ParseStatementMarkdown(string(req.Content))
		return uc.saveManifest(ctx, req.ProblemID, manifest)
	}

	// If the file is a component, register it in the manifest's FilesMetadata
	var componentType string
	switch {
	case strings.HasPrefix(req.Path, "checkers/"):
		componentType = "checker"
	case strings.HasPrefix(req.Path, "validators/"):
		componentType = "validator"
	case strings.HasPrefix(req.Path, "interactors/"):
		componentType = "interactor"
	case strings.HasPrefix(req.Path, "generators/"):
		componentType = "generator"
	}

	if componentType != "" {
		manifest, err := uc.loadManifest(ctx, req.ProblemID)
		if err != nil {
			return err
		}
		foundIdx := -1
		for i := range manifest.FilesMetadata {
			if manifest.FilesMetadata[i].Filename == req.Path {
				foundIdx = i
				break
			}
		}

		compiler := "cpp17"
		ext := filepath.Ext(req.Path)
		switch ext {
		case ".py":
			compiler = "python3"
		case ".go":
			compiler = "go"
		case ".java":
			compiler = "java"
		}

		if foundIdx != -1 {
			manifest.FilesMetadata[foundIdx].Compiler = compiler
		} else {
			manifest.FilesMetadata = append(manifest.FilesMetadata, models.FileMetadata{
				Type:         componentType,
				Filename:     req.Path,
				Compiler:     compiler,
				Dependencies: []models.Dependency{},
			})
		}

		if err := uc.saveManifest(ctx, req.ProblemID, manifest); err != nil {
			return err
		}
	}

	return nil
}

func (uc *WorkshopUseCase) deleteProblemFileUnlocked(ctx context.Context, problemID uuid.UUID, path string) error {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "problem.yaml" || path == "statements/en.md" {
		return fmt.Errorf("cannot delete derived metadata file %s directly", path)
	}

	if err := uc.workspaceStorage.DeleteFile(ctx, problemID, path); err != nil {
		return err
	}

	manifest, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return err
	}
	newMetadata := make([]models.FileMetadata, 0, len(manifest.FilesMetadata))
	changed := false
	for _, m := range manifest.FilesMetadata {
		if m.Filename == path {
			changed = true
			continue
		}
		newMetadata = append(newMetadata, m)
	}
	if changed {
		manifest.FilesMetadata = newMetadata
		if err := uc.saveManifest(ctx, problemID, manifest); err != nil {
			return err
		}
	}

	return nil
}

func (uc *WorkshopUseCase) ReadProblemFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) ([]byte, error) {
		if path == "problem.yaml" || path == "statements/en.md" {
			data, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
			if err != nil {
				return nil, fmt.Errorf("load file snapshot: %w", err)
			}
			if len(data) > 0 {
				if _, err := uc.loadManifest(ctx, problemID); err != nil {
					return nil, err
				}
			}
		}
		return uc.workspaceStorage.ReadFile(ctx, problemID, path)
	})
}

func (uc *WorkshopUseCase) ListProblemFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]models.FileEntry, error) {
	return uc.workspaceStorage.ListFiles(ctx, problemID, dirPath)
}

func (uc *WorkshopUseCase) compileProblemComponentUnlocked(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error) {
	var folderPrefix string
	switch req.ComponentType {
	case "checker":
		folderPrefix = "checkers/"
	case "validator":
		folderPrefix = "validators/"
	case "interactor":
		folderPrefix = "interactors/"
	case "generator":
		folderPrefix = "generators/"
	default:
		return nil, fmt.Errorf("unknown component type: %s", req.ComponentType)
	}

	allFiles, err := uc.workspaceStorage.ListAllFiles(ctx, req.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace files: %w", err)
	}

	var sourcePath string
	for _, file := range allFiles {
		if strings.HasPrefix(file, folderPrefix) {
			ext := filepath.Ext(file)
			if ext == ".cpp" || ext == ".py" || ext == ".go" || ext == ".java" {
				sourcePath = file
				break
			}
		}
	}

	if sourcePath == "" {
		return nil, fmt.Errorf("no source file found in %s folder", folderPrefix)
	}

	sourceCode, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
	}

	dependencies := make(map[string][]byte)
	for _, file := range allFiles {
		if strings.HasPrefix(file, "lib/") {
			depContent, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, file)
			if err != nil {
				return nil, fmt.Errorf("failed to read dependency %s: %w", file, err)
			}
			dependencies[filepath.Base(file)] = depContent
		}
	}

	langKey := detectLanguage(filepath.Ext(sourcePath))
	binary, compileErr := uc.sandbox.Compile(ctx, sourceCode, langKey, dependencies)
	if compileErr != nil {
		if errors.Is(compileErr, context.DeadlineExceeded) || errors.Is(compileErr, context.Canceled) {
			return nil, compileErr
		}
		_ = compileErr
		return &models.CompileResult{Success: false, CompileError: compileErr.Error()}, nil
	}

	sha256Hash := computeSHA256([]byte(binary.PrimaryFileID))

	manifest, err := uc.loadManifest(ctx, req.ProblemID)
	if err != nil {
		return nil, err
	}
	foundIdx := -1
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == req.ComponentType {
			foundIdx = i
			break
		}
	}

	compiler := "cpp17"
	ext := filepath.Ext(sourcePath)
	switch ext {
	case ".py":
		compiler = "python3"
	case ".go":
		compiler = "go"
	case ".java":
		compiler = "java"
	}

	var deps []models.Dependency
	for depName := range dependencies {
		deps = append(deps, models.Dependency{
			Filename: depName,
			Version:  "1.0",
		})
	}

	if foundIdx != -1 {
		manifest.FilesMetadata[foundIdx].Filename = sourcePath
		manifest.FilesMetadata[foundIdx].Compiler = compiler
		manifest.FilesMetadata[foundIdx].BinarySha256 = &sha256Hash
		manifest.FilesMetadata[foundIdx].Dependencies = deps
	} else {
		manifest.FilesMetadata = append(manifest.FilesMetadata, models.FileMetadata{
			Type:         req.ComponentType,
			Filename:     sourcePath,
			Compiler:     compiler,
			BinarySha256: &sha256Hash,
			Dependencies: deps,
		})
	}
	if err := uc.saveManifest(ctx, req.ProblemID, manifest); err != nil {
		return nil, err
	}

	return &models.CompileResult{
		Success:    true,
		FileID:     binary.PrimaryFileID,
		SHA256:     sha256Hash,
		CompileLog: "Component compiled successfully",
	}, nil
}

func (uc *WorkshopUseCase) GenerateTests(ctx context.Context, req models.GenerateTestsRequest) error {
	return fmt.Errorf("test generation not yet fully implemented")
}

func (uc *WorkshopUseCase) ValidateAllTests(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*models.ValidationReport, error) {
	manifest, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	var validatorMeta *models.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == "validator" {
			validatorMeta = &manifest.FilesMetadata[i]
			break
		}
	}

	if validatorMeta == nil {
		return nil, fmt.Errorf("problem must have a validator")
	}

	prob, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to read tests metadata: %w", err)
	}

	source, err := uc.workspaceStorage.ReadFile(ctx, problemID, validatorMeta.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read validator %s: %w", validatorMeta.Filename, err)
	}
	deps, err := uc.readComponentDependencies(ctx, problemID, validatorMeta)
	if err != nil {
		return nil, err
	}
	language := detectLanguage(filepath.Ext(validatorMeta.Filename))
	executable, err := uc.sandbox.Compile(ctx, source, language, deps)
	if err != nil {
		return nil, fmt.Errorf("failed to compile validator %s: %w", validatorMeta.Filename, err)
	}

	report := &models.ValidationReport{Results: make([]models.TestValidationResult, 0)}
	for _, group := range prob.OrderedSubtaskNames() {
		for _, test := range prob.Subtasks[group].Tests {
			report.TotalTests++
			inputName := test.Manual
			if inputName == "" {
				inputName = fmt.Sprintf("%02d.in", report.TotalTests)
			}
			inputPath := filepath.ToSlash(filepath.Join("tests", inputName))
			input, err := uc.readTestInput(ctx, problemID, test, report.TotalTests)
			result := models.TestValidationResult{TestNumber: report.TotalTests}
			switch {
			case errors.Is(err, storage.ErrNotFound):
				result.Error = fmt.Sprintf("test file not found: %s", inputPath)
			case err != nil:
				return nil, fmt.Errorf("failed to read test input %s: %w", inputPath, err)
			default:
				run, err := uc.sandbox.Test(ctx, executable, language, input, 5000, 512)
				if err != nil {
					return nil, fmt.Errorf("failed to run validator on test %d (%s): %w", result.TestNumber, inputPath, err)
				}
				if run.Status == sandbox.StatusFail {
					return nil, fmt.Errorf("validator sandbox failure on test %d (%s): %s", result.TestNumber, inputPath, strings.TrimSpace(string(run.Stderr)))
				}
				result.Valid = run.Status == sandbox.StatusOK && run.ExitStatus == 0
				if result.Valid {
					result.Message = "Validator accepted test input"
				} else {
					result.Error = fmt.Sprintf("validator status=%s, exit code=%d: %s", run.Status, run.ExitStatus, strings.TrimSpace(string(run.Stderr)))
				}
			}

			report.Results = append(report.Results, result)
			if result.Valid {
				report.ValidTests++
			} else {
				report.InvalidTests++
			}
		}
	}

	return report, nil
}

func (uc *WorkshopUseCase) TestSolution(ctx context.Context, req models.TestSolutionRequest) (*models.TestReport, error) {
	manifest, err := uc.loadManifest(ctx, req.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	solutionCode, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, req.SolutionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read solution: %w", err)
	}

	ext := filepath.Ext(req.SolutionPath)
	language := detectLanguage(ext)

	testsMeta, err := uc.readTestsMetadata(ctx, req.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("failed to read tests metadata: %w", err)
	}

	// A checker is required for solution evaluation.
	var checkerMeta *models.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == "checker" {
			checkerMeta = &manifest.FilesMetadata[i]
			break
		}
	}

	if checkerMeta == nil {
		return nil, fmt.Errorf("problem must have a checker")
	}
	chkSource, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, checkerMeta.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read checker %s: %w", checkerMeta.Filename, err)
	}
	chkDeps, err := uc.readComponentDependencies(ctx, req.ProblemID, checkerMeta)
	if err != nil {
		return nil, err
	}
	chkLang := detectLanguage(filepath.Ext(checkerMeta.Filename))
	checkerExec, err := uc.sandbox.Compile(ctx, chkSource, chkLang, chkDeps)
	if err != nil {
		return nil, fmt.Errorf("failed to compile checker %s: %w", checkerMeta.Filename, err)
	}

	// Compile solution
	solExec, err := uc.sandbox.Compile(ctx, solutionCode, language, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to compile solution: %w", err)
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

		inputName := test.Filename
		if inputName == "" {
			inputName = fmt.Sprintf("%02d.in", test.Ordinal)
		}

		input, err := uc.readTestInput(ctx, req.ProblemID, gfmt.Test{Manual: test.Filename, Generate: stringValue(test.Generator)}, test.Ordinal)
		if err != nil {
			report.Results = append(report.Results, models.TestResult{
				TestNumber: test.Ordinal,
				Verdict:    "IE",
				Message:    fmt.Sprintf("Failed to read input: %v", err),
			})
			report.FailedTests++
			continue
		}

		var answer []byte
		err = storage.ErrNotFound
		if test.Filename != "" {
			answer, err = uc.readTestAnswer(ctx, req.ProblemID, inputName)
		}
		if errors.Is(err, storage.ErrNotFound) {
			answer, err = uc.generateTestAnswer(ctx, req.ProblemID, input)
		}
		if err != nil {
			report.Results = append(report.Results, models.TestResult{
				TestNumber: test.Ordinal,
				Verdict:    "IE",
				Message:    fmt.Sprintf("Failed to read answer: %v", err),
			})
			report.FailedTests++
			continue
		}

		// Run solution test in sandbox
		runRes, err := uc.sandbox.Test(ctx, solExec, language, input, manifest.TimeLimitMs, manifest.MemoryLimitMb)
		if err != nil {
			report.Results = append(report.Results, models.TestResult{
				TestNumber: test.Ordinal,
				Verdict:    "IE",
				Message:    fmt.Sprintf("Sandbox execution dispatch failed: %v", err),
			})
			report.FailedTests++
			continue
		}

		verdict := string(runRes.Status)
		message := string(runRes.Stderr)

		if runRes.Status == sandbox.StatusOK {
			checkRes, err := uc.sandbox.Check(ctx, checkerExec, input, runRes.Stdout, answer)
			if err != nil {
				verdict = "IE"
				message = fmt.Sprintf("Checker execution failed: %v", err)
			} else {
				verdict = string(checkRes.Status)
				message = checkRes.Message
			}
		}

		testResult := models.TestResult{
			TestNumber: test.Ordinal,
			Verdict:    verdict,
			Time:       runRes.Time.Nanoseconds() / 1_000_000,
			Memory:     runRes.Memory,
			Message:    message,
		}

		report.Results = append(report.Results, testResult)

		if verdict == "OK" {
			report.PassedTests++
		} else {
			report.FailedTests++
		}
	}

	return report, nil
}

// persistedManifest commits both metadata and the complete YAML configuration in
// one database update. Legacy records are adopted on their first successful save.
type persistedManifest struct {
	*models.ProblemManifest
	WorkspaceYAML string `json:"workspace_yaml,omitempty"`
}

func (uc *WorkshopUseCase) loadManifestUnlocked(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	data, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest from database: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("manifest.json not found")
	}
	stored := persistedManifest{ProblemManifest: &models.ProblemManifest{}}
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}
	if stored.ProblemManifest == nil {
		return nil, fmt.Errorf("manifest.json is null")
	}
	if stored.WorkspaceYAML != "" {
		if err := uc.reconcileManifest(ctx, problemID, &stored); err != nil {
			return nil, err
		}
	}
	return stored.ProblemManifest, nil
}

func (uc *WorkshopUseCase) saveManifestUnlocked(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest) error {
	stored, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return err
	}
	previous, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return err
	}
	config := ManifestAndTestsToGfmtProblem(manifest, GfmtProblemToTestsMetadata(previous))
	config.Solutions = previous.Solutions
	if slices.Equal(componentFilenames(stored, "checker"), componentFilenames(manifest, "checker")) {
		config.Checker = previous.Checker
	}
	if slices.Equal(componentFilenames(stored, "validator"), componentFilenames(manifest, "validator")) {
		config.Validator = previous.Validator
	}
	if slices.Equal(componentFilenames(stored, "interactor"), componentFilenames(manifest, "interactor")) {
		config.Interactor = previous.Interactor
	}
	return uc.commitManifest(ctx, problemID, manifest, config)
}

func (uc *WorkshopUseCase) commitManifest(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest, config *gfmt.Problem) error {
	manifest.LastUpdated = time.Now()
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal problem.yaml: %w", err)
	}
	stored := persistedManifest{ProblemManifest: manifest, WorkspaceYAML: string(yamlBytes)}
	data, err := json.Marshal(&stored)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := uc.problemsRepo.UpdateProblemManifest(ctx, problemID, data); err != nil {
		return fmt.Errorf("commit manifest: %w", err)
	}
	return uc.reconcileManifest(ctx, problemID, &stored)
}

// A failed projection leaves the committed snapshot intact. Every subsequent
// manifest read/save/publication retries reconciliation and surfaces any failure.
func (uc *WorkshopUseCase) reconcileManifest(ctx context.Context, problemID uuid.UUID, stored *persistedManifest) error {
	var config gfmt.Problem
	if err := yaml.Unmarshal([]byte(stored.WorkspaceYAML), &config); err != nil {
		return fmt.Errorf("invalid committed YAML snapshot: %w", err)
	}
	for _, file := range []struct {
		path    string
		content []byte
	}{
		{"problem.yaml", []byte(stored.WorkspaceYAML)},
		{"statements/en.md", []byte(RenderStatementMarkdown(stored.Statement))},
	} {
		current, err := uc.workspaceStorage.ReadFile(ctx, problemID, file.path)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("read projection %s: %w", file.path, err)
		}
		if !bytes.Equal(current, file.content) || err != nil {
			if err := uc.workspaceStorage.WriteFile(ctx, problemID, file.path, file.content); err != nil {
				return fmt.Errorf("repair projection %s: %w", file.path, err)
			}
		}
	}
	if err := uc.problemsRepo.UpdateProblemLimits(ctx, problemID, stored.TimeLimitMs, stored.MemoryLimitMb); err != nil {
		return fmt.Errorf("repair problem limits: %w", err)
	}
	return uc.syncProblemTitle(ctx, problemID, stored.Statement.Title)
}

func (uc *WorkshopUseCase) readComponentDependencies(ctx context.Context, problemID uuid.UUID, meta *models.FileMetadata) (map[string][]byte, error) {
	deps := make(map[string][]byte)
	for _, dep := range meta.Dependencies {
		depPath := filepath.ToSlash(filepath.Join(filepath.Dir(meta.Filename), dep.Filename))
		depContent, err := uc.workspaceStorage.ReadFile(ctx, problemID, depPath)
		// CompileProblemComponent records shared lib/ dependencies by basename.
		if errors.Is(err, storage.ErrNotFound) && filepath.Base(dep.Filename) == dep.Filename {
			allFiles, listErr := uc.workspaceStorage.ListAllFiles(ctx, problemID)
			if listErr != nil {
				return nil, fmt.Errorf("failed to list %s dependencies: %w", meta.Type, listErr)
			}
			depPath = filepath.ToSlash(filepath.Join("lib", dep.Filename))
			for _, file := range allFiles {
				if strings.HasPrefix(file, "lib/") && filepath.Base(file) == dep.Filename {
					depPath = file
				}
			}
			depContent, err = uc.workspaceStorage.ReadFile(ctx, problemID, depPath)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read %s dependency %s: %w", meta.Type, depPath, err)
		}
		deps[dep.Filename] = depContent
	}
	return deps, nil
}

func (uc *WorkshopUseCase) readTestsMetadata(ctx context.Context, problemID uuid.UUID) (*models.TestsMetadata, error) {
	prob, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return nil, err
	}
	return GfmtProblemToTestsMetadata(prob), nil
}

func (uc *WorkshopUseCase) readProblemConfig(ctx context.Context, problemID uuid.UUID) (*gfmt.Problem, error) {
	yamlBytes, err := uc.workspaceStorage.ReadFile(ctx, problemID, "problem.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.yaml: %w", err)
	}

	var prob gfmt.Problem
	if err := yaml.Unmarshal(yamlBytes, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse problem.yaml: %w", err)
	}

	return &prob, nil
}

func defaultManifest(title string) *models.ProblemManifest {
	return &models.ProblemManifest{
		LastUpdated:   time.Now(),
		ProblemType:   "pass-fail",
		MaxScore:      nil,
		FilesMetadata: []models.FileMetadata{},
		TimeLimitMs:   1000,
		MemoryLimitMb: 256,
		Statement: models.Statement{
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

func defaultTestsMetadata() *models.TestsMetadata {
	return &models.TestsMetadata{
		Groups: []models.TestGroup{
			{
				Ordinal:      1,
				Name:         "samples",
				Points:       0,
				PointsPolicy: "complete",
				DependsOn:    []int{},
				Tests:        [2]int{1, 1},
			},
		},
		Tests: []models.TestCase{
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
		return "cpp"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".java":
		return "java"
	default:
		return "cpp"
	}
}

func computeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func GfmtProblemToTestsMetadata(prob *gfmt.Problem) *models.TestsMetadata {
	testsMeta := &models.TestsMetadata{
		Groups: []models.TestGroup{},
		Tests:  []models.TestCase{},
	}

	testOrdinal := 1
	groupOrdinal := 1
	for _, subName := range prob.OrderedSubtaskNames() {
		sub := prob.Subtasks[subName]
		startTest := testOrdinal
		for _, t := range sub.Tests {
			tc := models.TestCase{
				Ordinal:  testOrdinal,
				Filename: t.Manual,
				IsSample: subName == "samples" || t.Sample,
			}
			if t.Generate != "" {
				tc.Method = "generated"
				tc.Generator = &t.Generate
			} else {
				tc.Method = "manual"
			}
			testsMeta.Tests = append(testsMeta.Tests, tc)
			testOrdinal++
		}
		endTest := testOrdinal - 1

		group := models.TestGroup{
			Ordinal:      groupOrdinal,
			Name:         subName,
			Points:       sub.Points,
			PointsPolicy: sub.Policy,
			DependsOn:    []int{},
			Tests:        [2]int{startTest, endTest},
		}
		testsMeta.Groups = append(testsMeta.Groups, group)
		groupOrdinal++
	}

	// Resolve DependsOn names to ordinals
	for i, grp := range testsMeta.Groups {
		subName := grp.Name
		sub := prob.Subtasks[subName]
		for _, depName := range sub.Dependencies {
			for _, otherGrp := range testsMeta.Groups {
				if otherGrp.Name == depName {
					testsMeta.Groups[i].DependsOn = append(testsMeta.Groups[i].DependsOn, otherGrp.Ordinal)
					break
				}
			}
		}
	}

	return testsMeta
}

func ManifestAndTestsToGfmtProblem(manifest *models.ProblemManifest, testsMeta *models.TestsMetadata) *gfmt.Problem {
	prob := &gfmt.Problem{
		FormatVersion: "1.0",
		Title:         manifest.Statement.Title,
		Type:          manifest.ProblemType,
		Limits: gfmt.Limits{
			TimeMs:   manifest.TimeLimitMs,
			MemoryMb: manifest.MemoryLimitMb,
		},
		Subtasks:  make(map[string]gfmt.Subtask),
		Solutions: make(map[string]string),
	}

	for _, meta := range manifest.FilesMetadata {
		switch meta.Type {
		case "checker":
			if prob.Checker == "" {
				prob.Checker = meta.Filename
			}
		case "interactor":
			if prob.Interactor == "" {
				prob.Interactor = meta.Filename
			}
		case "validator":
			if prob.Validator == "" {
				prob.Validator = meta.Filename
			}
		}
	}

	if testsMeta != nil {
		for _, grp := range testsMeta.Groups {
			var tests []gfmt.Test
			for testNum := grp.Tests[0]; testNum <= grp.Tests[1]; testNum++ {
				var testCase *models.TestCase
				for _, tc := range testsMeta.Tests {
					if tc.Ordinal == testNum {
						testCase = &tc
						break
					}
				}

				if testCase == nil {
					continue
				}

				t := gfmt.Test{Manual: testCase.Filename}
				if testCase.Method == "generated" && testCase.Generator != nil {
					t.Generate = *testCase.Generator
				} else if t.Manual == "" {
					t.Manual = fmt.Sprintf("%02d.in", testNum)
				}
				if testCase.IsSample {
					t.Sample = true
				}
				tests = append(tests, t)
			}

			var deps []string
			for _, depOrd := range grp.DependsOn {
				for _, g := range testsMeta.Groups {
					if g.Ordinal == depOrd {
						deps = append(deps, g.Name)
						break
					}
				}
			}

			prob.Subtasks[grp.Name] = gfmt.Subtask{
				Points:       grp.Points,
				Policy:       grp.PointsPolicy,
				Dependencies: deps,
				Tests:        tests,
			}
		}
	}

	return prob
}

func (uc *WorkshopUseCase) GetManifest(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	return uc.loadManifest(ctx, problemID)
}

func (uc *WorkshopUseCase) SaveManifest(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest) error {
	return uc.saveManifest(ctx, problemID, manifest)
}

func (uc *WorkshopUseCase) syncProblemTitle(ctx context.Context, problemID uuid.UUID, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return fmt.Errorf("read problem title: %w", err)
	}
	if strings.TrimSpace(problem.Title) != title {
		if err := uc.problemsRepo.UpdateProblem(ctx, problemID, &models.ProblemUpdate{Title: &title}); err != nil {
			return fmt.Errorf("repair problem title: %w", err)
		}
	}
	return nil
}

func (uc *WorkshopUseCase) updateTestsConfigUnlocked(ctx context.Context, problemID uuid.UUID, testsMeta *models.TestsMetadata) error {
	manifest, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return fmt.Errorf("failed to load manifest for tests config update: %w", err)
	}

	gfmtProb := ManifestAndTestsToGfmtProblem(manifest, testsMeta)
	previous, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return fmt.Errorf("failed to read tests config: %w", err)
	}
	previous.Subtasks = gfmtProb.Subtasks
	gfmtProb = previous
	return uc.commitManifest(ctx, problemID, manifest, gfmtProb)
}

func (uc *WorkshopUseCase) getTestsConfigUnlocked(ctx context.Context, problemID uuid.UUID) (*models.TestsMetadata, error) {
	if _, err := uc.loadManifest(ctx, problemID); err != nil {
		return nil, err
	}
	return uc.readTestsMetadata(ctx, problemID)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (uc *WorkshopUseCase) readTestAnswer(ctx context.Context, problemID uuid.UUID, name string) ([]byte, error) {
	for _, candidate := range gfmt.TestOutputNames(name) {
		data, err := uc.workspaceStorage.ReadFile(ctx, problemID, "tests/"+candidate)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("test output file not found for %s: %w", name, storage.ErrNotFound)
}

func (uc *WorkshopUseCase) readTestInput(ctx context.Context, problemID uuid.UUID, test gfmt.Test, ordinal int) ([]byte, error) {
	if test.Manual != "" {
		for _, name := range gfmt.TestInputNames(test.Manual) {
			data, err := uc.workspaceStorage.ReadFile(ctx, problemID, "tests/"+name)
			if err == nil {
				return data, nil
			}
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, err
			}
		}
		return nil, fmt.Errorf("test input file not found for %s: %w", test.Manual, storage.ErrNotFound)
	}
	if test.Generate == "" {
		return uc.workspaceStorage.ReadFile(ctx, problemID, fmt.Sprintf("tests/%02d.in", ordinal))
	}
	parts := strings.Fields(test.Generate)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty generate command")
	}
	files, err := uc.workspaceStorage.ListAllFiles(ctx, problemID)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var source string
	deps := make(map[string][]byte)
	for _, file := range files {
		if filepath.Dir(file) == "generators" && strings.HasPrefix(filepath.Base(file), parts[0]) && source == "" {
			source = file
		}
		if filepath.Dir(file) == "lib" {
			data, err := uc.workspaceStorage.ReadFile(ctx, problemID, file)
			if err != nil {
				return nil, err
			}
			deps[filepath.Base(file)] = data
		}
	}
	if source == "" {
		return nil, fmt.Errorf("generator source not found for %s", parts[0])
	}
	data, err := uc.workspaceStorage.ReadFile(ctx, problemID, source)
	if err != nil {
		return nil, err
	}
	executable, err := uc.sandbox.Compile(ctx, data, detectLanguage(filepath.Ext(source)), deps)
	if err != nil {
		return nil, err
	}
	return uc.sandbox.Generate(ctx, executable, parts[1:])
}

func (uc *WorkshopUseCase) generateTestAnswer(ctx context.Context, problemID uuid.UUID, input []byte) ([]byte, error) {
	prob, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return nil, err
	}
	var candidates []string
	for name, tag := range prob.Solutions {
		if tag == "OK" || tag == "Accepted" || tag == "main" {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no test answer file or correct solution found to generate answer")
	}
	name := candidates[0]
	for _, candidate := range candidates {
		if prob.Solutions[candidate] == "main" {
			name = candidate
			break
		}
	}
	data, err := uc.workspaceStorage.ReadFile(ctx, problemID, "solutions/"+name)
	if err != nil {
		return nil, err
	}
	language := detectLanguage(filepath.Ext(name))
	executable, err := uc.sandbox.Compile(ctx, data, language, nil)
	if err != nil {
		return nil, err
	}
	result, err := uc.sandbox.Test(ctx, executable, language, input, prob.Limits.TimeMs, prob.Limits.MemoryMb)
	if err != nil {
		return nil, err
	}
	if result.Status != sandbox.StatusOK {
		return nil, fmt.Errorf("correct solution failed to run: %s", result.Status)
	}
	return result.Stdout, nil
}

func componentFilenames(manifest *models.ProblemManifest, kind string) []string {
	var names []string
	for _, file := range manifest.FilesMetadata {
		if file.Type == kind {
			names = append(names, file.Filename)
		}
	}
	return names
}
