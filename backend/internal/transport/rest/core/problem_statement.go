package core

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func (h *CoreServer) loadProblemStatementFiles(ctx context.Context, problemID uuid.UUID, lang string) *models.Statement {
	if h.workshopUC == nil {
		return nil
	}

	if lang != "" && lang != "en" {
		if mdBytes, err := h.workshopUC.ReadProblemFile(ctx, problemID, "statements/"+lang+".md"); err == nil {
			parsed := usecase.ParseStatementMarkdown(string(mdBytes))
			if hasStatementContent(parsed) {
				return &parsed
			}
		}
	}

	manifest, err := h.workshopUC.GetManifest(ctx, problemID)
	if err != nil {
		return nil
	}

	if hasStatementContent(manifest.Statement) {
		statement := manifest.Statement
		return &statement
	}

	return nil
}

func hasStatementContent(statement models.Statement) bool {
	return strings.TrimSpace(statement.Title) != "" ||
		strings.TrimSpace(statement.Legend) != "" ||
		strings.TrimSpace(statement.InputFormat) != "" ||
		strings.TrimSpace(statement.OutputFormat) != "" ||
		strings.TrimSpace(statement.Notes) != "" ||
		strings.TrimSpace(statement.Scoring) != ""
}

func (h *CoreServer) loadProblemSampleFiles(ctx context.Context, problemID uuid.UUID) []corev1.ProblemSampleModel {
	if h.workshopUC == nil {
		return []corev1.ProblemSampleModel{}
	}

	yamlBytes, err := h.workshopUC.ReadProblemFile(ctx, problemID, "problem.yaml")
	if err != nil {
		return []corev1.ProblemSampleModel{}
	}

	var prob gfmt.Problem
	if err := yaml.Unmarshal(yamlBytes, &prob); err != nil {
		return []corev1.ProblemSampleModel{}
	}

	var samples []corev1.ProblemSampleModel
	for subName, subtask := range prob.Subtasks {
		for _, t := range subtask.Tests {
			if !t.Sample && subName != "samples" {
				continue
			}
			if t.Manual == "" {
				continue
			}

			inputBytes, err := h.workshopUC.ReadProblemFile(ctx, problemID, "tests/"+t.Manual)
			if err != nil {
				continue
			}

			// Try .out first, then .ans
			ansFile := strings.TrimSuffix(t.Manual, ".in") + ".out"
			outputBytes, err := h.workshopUC.ReadProblemFile(ctx, problemID, "tests/"+ansFile)
			if err != nil {
				ansFile = strings.TrimSuffix(t.Manual, ".in") + ".ans"
				outputBytes, err = h.workshopUC.ReadProblemFile(ctx, problemID, "tests/"+ansFile)
			}

			if err != nil {
				continue
			}

			samples = append(samples, corev1.ProblemSampleModel{
				Input:  string(inputBytes),
				Output: string(outputBytes),
			})
		}
	}

	return samples
}

func (h *CoreServer) loadPackageStatementAndSamples(
	ctx context.Context,
	problemID uuid.UUID,
	packageID uuid.UUID,
) (*models.Statement, []corev1.ProblemSampleModel) {
	return h.loadPackageStatementAndSamplesWithLang(ctx, problemID, packageID, "")
}

func (h *CoreServer) loadPackageStatementAndSamplesWithLang(
	ctx context.Context,
	problemID uuid.UUID,
	packageID uuid.UUID,
	lang string,
) (*models.Statement, []corev1.ProblemSampleModel) {
	if h.publishUC == nil || packageID == uuid.Nil {
		return nil, nil
	}

	pkg, err := h.publishUC.GetPackageByID(ctx, packageID)
	if err != nil || pkg.PackageHash == "" {
		return nil, nil
	}

	reader, err := h.publishUC.DownloadPackage(ctx, problemID, pkg.PackageHash)
	if err != nil {
		return nil, nil
	}
	defer reader.Close()

	tempDir, err := os.MkdirTemp("", "contest-pkg-*")
	if err != nil {
		return nil, nil
	}
	defer os.RemoveAll(tempDir)

	zipBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, nil
	}

	for _, f := range zipReader.File {
		path := filepath.Join(tempDir, filepath.Clean(f.Name))
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(path, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(path)
		if err != nil {
			rc.Close()
			continue
		}
		_, _ = io.Copy(out, io.LimitReader(rc, 100<<20))
		rc.Close()
		out.Close()
	}

	gfmtPkg, err := gfmt.OpenPackage(tempDir)
	if err != nil || gfmtPkg == nil || gfmtPkg.Problem == nil {
		return nil, nil
	}

	var statement models.Statement
	if lang == "" {
		lang = "en"
	}
	if data, err := os.ReadFile(filepath.Join(tempDir, "statements", lang+".md")); err == nil {
		statement = usecase.ParseStatementMarkdown(string(data))
	}
	if lang == "en" || strings.TrimSpace(statement.Title) == "" {
		statement.Title = gfmtPkg.Problem.Title
	}

	var samples []corev1.ProblemSampleModel
	for subName, subtask := range gfmtPkg.Problem.Subtasks {
		for _, t := range subtask.Tests {
			if !t.Sample && subName != "samples" {
				continue
			}
			if t.Manual == "" {
				continue
			}

			inputBytes, err := os.ReadFile(filepath.Join(tempDir, "tests", t.Manual))
			if err != nil {
				continue
			}

			ansFile := strings.TrimSuffix(t.Manual, ".in") + ".out"
			outputBytes, err := os.ReadFile(filepath.Join(tempDir, "tests", ansFile))
			if err != nil {
				ansFile = strings.TrimSuffix(t.Manual, ".in") + ".ans"
				outputBytes, err = os.ReadFile(filepath.Join(tempDir, "tests", ansFile))
			}

			if err != nil {
				continue
			}

			samples = append(samples, corev1.ProblemSampleModel{
				Input:  string(inputBytes),
				Output: string(outputBytes),
			})
		}
	}

	return &statement, samples
}

func (h *CoreServer) loadWorkspaceStatementAndSamples(ctx context.Context, problemID uuid.UUID, lang string) (*models.Statement, []corev1.ProblemSampleModel) {
	if h.workshopUC == nil {
		return nil, nil
	}
	var statement *models.Statement
	var samples []corev1.ProblemSampleModel
	err := h.workshopUC.WithWorkspace(ctx, problemID, func(ctx context.Context) error {
		statement = h.loadProblemStatementFiles(ctx, problemID, lang)
		samples = h.loadProblemSampleFiles(ctx, problemID)
		return nil
	})
	if err != nil {
		return nil, nil
	}
	return statement, samples
}
