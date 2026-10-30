package usecase

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type WorkshopUseCase struct {
	workspaceStorage *WorkspaceStorage
	sandbox          *sandbox.Sandbox
}

func NewWorkshopUseCase(workspaceStorage *WorkspaceStorage, sandbox *sandbox.Sandbox) *WorkshopUseCase {
	return &WorkshopUseCase{workspaceStorage: workspaceStorage, sandbox: sandbox}
}

func (uc *WorkshopUseCase) IsInitialized(ctx context.Context, problemID uuid.UUID) (bool, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (bool, error) {
		_, err := uc.readProblemConfig(ctx, problemID)
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		return err == nil, err
	})
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

func (uc *WorkshopUseCase) initProblemWorkshopFiles(ctx context.Context, problemID uuid.UUID, title string) error {
	if _, err := uc.workspaceStorage.ReadFile(ctx, problemID, "problem.yaml"); err == nil {
		return fmt.Errorf("workshop already initialized for problem %s", problemID)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	testsMeta := defaultTestsMetadata()
	manifest := defaultManifest(title)

	// Save problem.yaml to workspace storage
	gfmtProb := ManifestAndTestsToGfmtProblem(manifest, testsMeta)
	yamlBytes, err := yaml.Marshal(gfmtProb)
	if err != nil {
		return fmt.Errorf("failed to marshal default problem.yaml: %w", err)
	}
	if err := uc.workspaceStorage.WriteFile(ctx, problemID, "problem.yaml", yamlBytes); err != nil {
		return fmt.Errorf("failed to create default problem.yaml: %w", err)
	}

	// Save default statement to statements/en.md
	statement := manifest.Statement
	statement.Title = ""
	stmtBytes := []byte(RenderStatementMarkdown(statement))
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

	return nil
}

func (uc *WorkshopUseCase) updateProblemFileFiles(ctx context.Context, req models.UpdateFileRequest) error {
	req.Path = filepath.ToSlash(filepath.Clean(req.Path))
	if req.Path == "problem.yaml" {
		return fmt.Errorf("cannot modify problem.yaml directly")
	}
	// The title belongs to problem.yaml; statement sections belong to Markdown.
	if req.Path == "statements/en.md" {
		statement := ParseStatementMarkdown(string(req.Content))
		config, err := uc.readProblemConfig(ctx, req.ProblemID)
		if err != nil {
			return err
		}
		if statement.Title != "" {
			config.Title = statement.Title
		}
		statement.Title = ""
		req.Content = []byte(RenderStatementMarkdown(statement))
		if err := uc.writeProblemConfig(ctx, req.ProblemID, config); err != nil {
			return err
		}
	}
	if kind := componentKind(req.Path); kind != "" {
		config, err := uc.readProblemConfig(ctx, req.ProblemID)
		if err != nil {
			return err
		}
		var selected *string
		switch kind {
		case "checker":
			selected = &config.Checker
		case "validator":
			selected = &config.Validator
		case "interactor":
			selected = &config.Interactor
		}
		if selected != nil && *selected == "" {
			*selected = req.Path
			if err := uc.writeProblemConfig(ctx, req.ProblemID, config); err != nil {
				return err
			}
		}
	}
	return uc.workspaceStorage.WriteFile(ctx, req.ProblemID, req.Path, req.Content)
}

func (uc *WorkshopUseCase) deleteProblemFileFiles(ctx context.Context, problemID uuid.UUID, path string) error {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "problem.yaml" {
		return fmt.Errorf("cannot delete problem.yaml directly")
	}
	config, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return err
	}
	for kind, selected := range map[string]*string{"checker": &config.Checker, "validator": &config.Validator, "interactor": &config.Interactor} {
		if componentPath(kind, *selected) == path {
			*selected = ""
		}
	}

	delete(config.Sources, path)
	if err := uc.writeProblemConfig(ctx, problemID, config); err != nil {
		return err
	}
	return uc.workspaceStorage.DeleteFile(ctx, problemID, path)
}

func (uc *WorkshopUseCase) ReadProblemFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	return uc.workspaceStorage.ReadFile(ctx, problemID, path)
}

func (uc *WorkshopUseCase) ListProblemFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]models.FileEntry, error) {
	return uc.workspaceStorage.ListFiles(ctx, problemID, dirPath)
}

func (uc *WorkshopUseCase) compileProblemComponentFiles(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error) {
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

func (uc *WorkshopUseCase) validateAllTestsFiles(ctx context.Context, problemID uuid.UUID) (*models.ValidationReport, error) {
	manifest, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	config, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return nil, err
	}
	var validatorMeta *models.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Filename == componentPath("validator", config.Validator) {
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

func (uc *WorkshopUseCase) testSolutionFiles(ctx context.Context, req models.TestSolutionRequest) (*models.TestReport, error) {
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

	config, err := uc.readProblemConfig(ctx, req.ProblemID)
	if err != nil {
		return nil, err
	}
	var checkerMeta *models.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Filename == componentPath("checker", config.Checker) {
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

func componentKind(path string) string {
	switch strings.Split(path, "/")[0] {
	case "checkers":
		return "checker"
	case "validators":
		return "validator"
	case "interactors":
		return "interactor"
	case "generators":
		return "generator"
	}
	return ""
}

func componentPath(kind, name string) string {
	if name != "" && !strings.HasPrefix(name, kind+"s/") {
		return kind + "s/" + name
	}
	return name
}

func (uc *WorkshopUseCase) loadManifestFiles(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	config, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return nil, err
	}
	manifest := &models.ProblemManifest{
		LastUpdated: config.UpdatedAt, ProblemType: config.Type, MaxScore: config.MaxScore,
		TimeLimitMs: config.Limits.TimeMs, MemoryLimitMb: config.Limits.MemoryMb,
		FilesMetadata: []models.FileMetadata{},
	}
	statement, err := uc.workspaceStorage.ReadFile(ctx, problemID, "statements/en.md")
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	manifest.Statement = ParseStatementMarkdown(string(statement))
	manifest.Statement.Title = config.Title
	files, err := uc.workspaceStorage.ListAllFiles(ctx, problemID)
	if err != nil {
		return nil, err
	}
	selected := map[string]string{"checker": componentPath("checker", config.Checker), "validator": componentPath("validator", config.Validator), "interactor": componentPath("interactor", config.Interactor)}
	for _, path := range selected {
		if path != "" && !slices.Contains(files, path) {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	var shared []models.Dependency
	for _, path := range files {
		if strings.HasPrefix(path, "lib/") {
			shared = append(shared, models.Dependency{Filename: filepath.Base(path), Version: "1.0"})
		}
	}
	for _, path := range files {
		kind := componentKind(path)
		if kind == "" || detectLanguage(filepath.Ext(path)) == "" {
			continue
		}
		// Header files are dependencies, not selectable executable components.
		switch filepath.Ext(path) {
		case ".h", ".hpp":
			continue
		}
		compiler := "cpp17"
		switch filepath.Ext(path) {
		case ".py":
			compiler = "python3"
		case ".go":
			compiler = "go"
		case ".java":
			compiler = "java"
		}
		deps := shared
		if source, ok := config.Sources[path]; ok {
			if source.Compiler != "" {
				compiler = source.Compiler
			}
			deps = nil
			for _, dep := range source.Dependencies {
				deps = append(deps, models.Dependency{Filename: dep, Version: "1.0"})
			}
		}
		meta := models.FileMetadata{Type: kind, Filename: path, Compiler: compiler, Dependencies: deps}
		if selected[kind] == path {
			manifest.FilesMetadata = append([]models.FileMetadata{meta}, manifest.FilesMetadata...)
		} else {
			manifest.FilesMetadata = append(manifest.FilesMetadata, meta)
		}
	}
	return manifest, nil
}

func (uc *WorkshopUseCase) saveManifestFiles(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest) error {
	previous, err := uc.readProblemConfig(ctx, problemID)
	if err != nil {
		return err
	}
	stored, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return err
	}
	config := ManifestAndTestsToGfmtProblem(manifest, GfmtProblemToTestsMetadata(previous))
	// Keep test declarations byte-for-byte in the model: metadata saves do not edit tests.
	config.Subtasks = previous.Subtasks
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
	config.Sources = make(map[string]gfmt.Source)
	for _, meta := range manifest.FilesMetadata {
		var deps []string
		for _, dep := range meta.Dependencies {
			deps = append(deps, dep.Filename)
		}
		config.Sources[meta.Filename] = gfmt.Source{Compiler: meta.Compiler, Dependencies: deps}
	}
	if err := uc.writeProblemConfig(ctx, problemID, config); err != nil {
		return err
	}
	statement := manifest.Statement
	statement.Title = ""
	return uc.workspaceStorage.WriteFile(ctx, problemID, "statements/en.md", []byte(RenderStatementMarkdown(statement)))
}

func (uc *WorkshopUseCase) writeProblemConfig(ctx context.Context, problemID uuid.UUID, config *gfmt.Problem) error {
	config.UpdatedAt = time.Now().UTC()
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal problem.yaml: %w", err)
	}
	return uc.workspaceStorage.WriteFile(ctx, problemID, "problem.yaml", data)
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
		MaxScore:      manifest.MaxScore,
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

func (uc *WorkshopUseCase) updateTestsConfigFiles(ctx context.Context, problemID uuid.UUID, testsMeta *models.TestsMetadata) error {
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
	yamlBytes, err := yaml.Marshal(gfmtProb)
	if err != nil {
		return fmt.Errorf("failed to marshal problem.yaml: %w", err)
	}
	if err := uc.workspaceStorage.WriteFile(ctx, problemID, "problem.yaml", yamlBytes); err != nil {
		return fmt.Errorf("failed to write problem.yaml: %w", err)
	}

	return nil
}

func (uc *WorkshopUseCase) GetTestsConfig(ctx context.Context, problemID uuid.UUID) (*models.TestsMetadata, error) {
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
	sort.Strings(names)
	return slices.Compact(names)
}

func (uc *WorkshopUseCase) InitProblemWorkshop(ctx context.Context, problemID uuid.UUID, title string) error {
	return uc.workspaceStorage.withPackage(ctx, problemID, func(ctx context.Context, _ *workspacePackage) error {
		return uc.initProblemWorkshopFiles(ctx, problemID, title)
	})
}

func (uc *WorkshopUseCase) UpdateProblemFile(ctx context.Context, req models.UpdateFileRequest) error {
	return uc.workspaceStorage.withPackage(ctx, req.ProblemID, func(ctx context.Context, _ *workspacePackage) error { return uc.updateProblemFileFiles(ctx, req) })
}

func (uc *WorkshopUseCase) DeleteProblemFile(ctx context.Context, problemID uuid.UUID, path string) error {
	return uc.workspaceStorage.withPackage(ctx, problemID, func(ctx context.Context, _ *workspacePackage) error {
		return uc.deleteProblemFileFiles(ctx, problemID, path)
	})
}

func (uc *WorkshopUseCase) CompileProblemComponent(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, req.ProblemID, func(ctx context.Context) (*models.CompileResult, error) {
		return uc.compileProblemComponentFiles(ctx, req)
	})
}

func (uc *WorkshopUseCase) loadManifest(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (*models.ProblemManifest, error) {
		return uc.loadManifestFiles(ctx, problemID)
	})
}

func (uc *WorkshopUseCase) saveManifest(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest) error {
	return uc.workspaceStorage.withPackage(ctx, problemID, func(ctx context.Context, _ *workspacePackage) error {
		return uc.saveManifestFiles(ctx, problemID, manifest)
	})
}

func (uc *WorkshopUseCase) UpdateTestsConfig(ctx context.Context, problemID uuid.UUID, testsMeta *models.TestsMetadata) error {
	return uc.workspaceStorage.withPackage(ctx, problemID, func(ctx context.Context, _ *workspacePackage) error {
		return uc.updateTestsConfigFiles(ctx, problemID, testsMeta)
	})
}

func (uc *WorkshopUseCase) ValidateAllTests(ctx context.Context, problemID uuid.UUID, userID uuid.UUID) (*models.ValidationReport, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (*models.ValidationReport, error) {
		return uc.validateAllTestsFiles(ctx, problemID)
	})
}

func (uc *WorkshopUseCase) TestSolution(ctx context.Context, req models.TestSolutionRequest) (*models.TestReport, error) {
	return withWorkspaceValue(ctx, uc.workspaceStorage, req.ProblemID, func(ctx context.Context) (*models.TestReport, error) { return uc.testSolutionFiles(ctx, req) })
}
