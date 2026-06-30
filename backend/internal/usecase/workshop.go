package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/sandbox"
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

func (uc *WorkshopUseCase) InitProblemWorkshop(ctx context.Context, problemID uuid.UUID, title string) error {
	if uc.IsInitialized(ctx, problemID) {
		return fmt.Errorf("workshop already initialized for problem %s", problemID)
	}


	testsMeta := defaultTestsMetadata()
	manifest := defaultManifest(title)

	// Save manifest to database
	if err := uc.saveManifest(ctx, problemID, manifest); err != nil {
		return fmt.Errorf("failed to save default manifest: %w", err)
	}

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

	return nil
}

func (uc *WorkshopUseCase) UpdateProblemFile(ctx context.Context, req models.UpdateFileRequest) error {
	if err := uc.workspaceStorage.WriteFile(ctx, req.ProblemID, req.Path, req.Content); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// If the file is a component, register it in the manifest's FilesMetadata
	var componentType string
	if strings.HasPrefix(req.Path, "checkers/") {
		componentType = "checker"
	} else if strings.HasPrefix(req.Path, "validators/") {
		componentType = "validator"
	} else if strings.HasPrefix(req.Path, "interactors/") {
		componentType = "interactor"
	} else if strings.HasPrefix(req.Path, "generators/") {
		componentType = "generator"
	}

	if componentType != "" {
		manifest, err := uc.loadManifest(ctx, req.ProblemID)
		if err == nil {
			foundIdx := -1
			for i := range manifest.FilesMetadata {
				if manifest.FilesMetadata[i].Filename == req.Path {
					foundIdx = i
					break
				}
			}

			compiler := "cpp17"
			ext := filepath.Ext(req.Path)
			if ext == ".py" {
				compiler = "python3"
			} else if ext == ".go" {
				compiler = "go"
			} else if ext == ".java" {
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

			_ = uc.saveManifest(ctx, req.ProblemID, manifest)
		}
	}

	return nil
}

func (uc *WorkshopUseCase) DeleteProblemFile(ctx context.Context, problemID uuid.UUID, path string) error {
	if err := uc.workspaceStorage.DeleteFile(ctx, problemID, path); err != nil {
		return err
	}

	manifest, err := uc.loadManifest(ctx, problemID)
	if err == nil {
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
			_ = uc.saveManifest(ctx, problemID, manifest)
		}
	}

	return nil
}

func (uc *WorkshopUseCase) ReadProblemFile(ctx context.Context, problemID uuid.UUID, path string) ([]byte, error) {
	return uc.workspaceStorage.ReadFile(ctx, problemID, path)
}

func (uc *WorkshopUseCase) ListProblemFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]models.FileEntry, error) {
	return uc.workspaceStorage.ListFiles(ctx, problemID, dirPath)
}

func (uc *WorkshopUseCase) CompileProblemComponent(ctx context.Context, req models.CompileComponentRequest) (*models.CompileResult, error) {
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
	binary, err := uc.sandbox.Compile(ctx, sourceCode, langKey, dependencies)
	if err != nil {
		return &models.CompileResult{Success: false, CompileError: err.Error()}, nil
	}

	sha256Hash := computeSHA256([]byte(binary.PrimaryFileID))

	manifest, err := uc.loadManifest(ctx, req.ProblemID)
	if err == nil {
		foundIdx := -1
		for i := range manifest.FilesMetadata {
			if manifest.FilesMetadata[i].Type == req.ComponentType {
				foundIdx = i
				break
			}
		}

		compiler := "cpp17"
		ext := filepath.Ext(sourcePath)
		if ext == ".py" {
			compiler = "python3"
		} else if ext == ".go" {
			compiler = "go"
		} else if ext == ".java" {
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
		_ = uc.saveManifest(ctx, req.ProblemID, manifest)
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
		return &models.ValidationReport{TotalTests: 0, ValidTests: 0, Results: []models.TestValidationResult{}}, nil
	}

	testsMeta, err := uc.readTestsMetadata(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to read tests metadata: %w", err)
	}

	report := &models.ValidationReport{TotalTests: len(testsMeta.Tests), Results: make([]models.TestValidationResult, 0)}

	for _, test := range testsMeta.Tests {
		inputPath := fmt.Sprintf("tests/%02d.in", test.Ordinal)
		_, err := uc.workspaceStorage.ReadFile(ctx, problemID, inputPath)

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

	// Compile checker if needed
	var checkerExec *sandbox.Executable
	var checkerMeta *models.FileMetadata
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == "checker" {
			checkerMeta = &manifest.FilesMetadata[i]
			break
		}
	}

	if checkerMeta != nil {
		chkSource, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, checkerMeta.Filename)
		if err == nil {
			chkDeps := make(map[string][]byte)
			for _, dep := range checkerMeta.Dependencies {
				depPath := filepath.Join(filepath.Dir(checkerMeta.Filename), dep.Filename)
				depPath = filepath.ToSlash(depPath)
				depContent, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, depPath)
				if err == nil {
					chkDeps[dep.Filename] = depContent
				}
			}
			chkLang := detectLanguage(filepath.Ext(checkerMeta.Filename))
			compiled, err := uc.sandbox.Compile(ctx, chkSource, chkLang, chkDeps)
			if err == nil {
				checkerExec = &compiled
			}
		}
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

		inputPath := fmt.Sprintf("tests/%02d.in", test.Ordinal)
		answerPath := fmt.Sprintf("tests/%02d.out", test.Ordinal)

		input, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, inputPath)
		if err != nil {
			report.Results = append(report.Results, models.TestResult{
				TestNumber: test.Ordinal,
				Verdict:    "IE",
				Message:    fmt.Sprintf("Failed to read input: %v", err),
			})
			report.FailedTests++
			continue
		}

		answer, err := uc.workspaceStorage.ReadFile(ctx, req.ProblemID, answerPath)
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
			if checkerExec != nil {
				checkRes, err := uc.sandbox.Check(ctx, *checkerExec, input, runRes.Stdout, answer)
				if err != nil {
					verdict = "IE"
					message = fmt.Sprintf("Checker execution failed: %v", err)
				} else {
					verdict = string(checkRes.Status)
					message = checkRes.Message
				}
			} else {
				// Simple text comparison if checker is not compiled
				if string(bytes.TrimSpace(runRes.Stdout)) == string(bytes.TrimSpace(answer)) {
					verdict = "OK"
					message = "Answer is correct"
				} else {
					verdict = "WA"
					message = "Output does not match expected answer"
				}
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

func (uc *WorkshopUseCase) loadManifest(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	manifestBytes, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest from database: %w", err)
	}
	if len(manifestBytes) == 0 {
		return nil, fmt.Errorf("manifest.json not found")
	}

	var manifest models.ProblemManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

func (uc *WorkshopUseCase) saveManifest(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest) error {
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

func (uc *WorkshopUseCase) readTestsMetadata(ctx context.Context, problemID uuid.UUID) (*models.TestsMetadata, error) {
	yamlBytes, err := uc.workspaceStorage.ReadFile(ctx, problemID, "problem.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.yaml: %w", err)
	}

	var prob gfmt.Problem
	if err := yaml.Unmarshal(yamlBytes, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse problem.yaml: %w", err)
	}

	return GfmtProblemToTestsMetadata(&prob), nil
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
	for subName, sub := range prob.Subtasks {
		startTest := testOrdinal
		for _, t := range sub.Tests {
			tc := models.TestCase{
				Ordinal:  testOrdinal,
				IsSample: subName == "samples",
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
		case "generator":
			if prob.Generator == "" {
				prob.Generator = meta.Filename
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

				var t gfmt.Test
				if testCase != nil && testCase.Method == "generated" && testCase.Generator != nil {
					t.Generate = *testCase.Generator
				} else {
					t.Manual = fmt.Sprintf("%02d.in", testNum)
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
	if err := uc.saveManifest(ctx, problemID, manifest); err != nil {
		return err
	}

	// Sync to problem.yaml
	testsMeta, err := uc.readTestsMetadata(ctx, problemID)
	if err != nil {
		testsMeta = defaultTestsMetadata()
	}
	gfmtProb := ManifestAndTestsToGfmtProblem(manifest, testsMeta)
	yamlBytes, err := yaml.Marshal(gfmtProb)
	if err == nil {
		_ = uc.workspaceStorage.WriteFile(ctx, problemID, "problem.yaml", yamlBytes)
	}

	// Sync to statements/en.md
	stmtBytes := []byte(RenderStatementMarkdown(manifest.Statement))
	_ = uc.workspaceStorage.WriteFile(ctx, problemID, "statements/en.md", stmtBytes)

	// Sync title if changed
	title := strings.TrimSpace(manifest.Statement.Title)
	if title != "" {
		problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
		if err == nil && strings.TrimSpace(problem.Title) != title {
			_ = uc.problemsRepo.UpdateProblem(ctx, problemID, &models.ProblemUpdate{Title: &title})
		}
	}

	return nil
}

func (uc *WorkshopUseCase) UpdateTestsConfig(ctx context.Context, problemID uuid.UUID, testsMeta *models.TestsMetadata) error {
	manifest, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return fmt.Errorf("failed to load manifest for tests config update: %w", err)
	}

	gfmtProb := ManifestAndTestsToGfmtProblem(manifest, testsMeta)
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
