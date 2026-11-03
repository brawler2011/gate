package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	ziputil "github.com/brawler2011/gate/backend/pkg/ziputil"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func (r *StatementReader) loadProblemStatementFiles(ctx context.Context, problemID uuid.UUID, lang string) *problemsmodel.Statement {
	if r.workshop == nil {
		return nil
	}

	if lang != "" && lang != "en" {
		if mdBytes, err := r.workshop.workspaceStorage.ReadFile(ctx, problemID, "statements/"+lang+".md"); err == nil {
			parsed := ParseStatementMarkdown(string(mdBytes))
			if hasStatementContent(parsed) {
				return &parsed
			}
		}
	}

	manifest, err := r.workshop.loadManifest(ctx, problemID)
	if err != nil {
		return nil
	}

	if hasStatementContent(manifest.Statement) {
		statement := manifest.Statement
		return &statement
	}

	return nil
}

func hasStatementContent(statement problemsmodel.Statement) bool {
	return strings.TrimSpace(statement.Title) != "" ||
		strings.TrimSpace(statement.Legend) != "" ||
		strings.TrimSpace(statement.InputFormat) != "" ||
		strings.TrimSpace(statement.OutputFormat) != "" ||
		strings.TrimSpace(statement.Notes) != "" ||
		strings.TrimSpace(statement.Scoring) != ""
}

func (r *StatementReader) loadProblemSampleFiles(ctx context.Context, problemID uuid.UUID) []problemsmodel.Sample {
	if r.workshop == nil {
		return []problemsmodel.Sample{}
	}

	yamlBytes, err := r.workshop.workspaceStorage.ReadFile(ctx, problemID, "problem.yaml")
	if err != nil {
		return []problemsmodel.Sample{}
	}

	var prob gfmt.Problem
	if err := yaml.Unmarshal(yamlBytes, &prob); err != nil {
		return []problemsmodel.Sample{}
	}

	var samples []problemsmodel.Sample
	for subName, subtask := range prob.Subtasks {
		for _, t := range subtask.Tests {
			if !t.Sample && subName != "samples" {
				continue
			}
			if t.Manual == "" {
				continue
			}

			inputBytes, err := r.workshop.workspaceStorage.ReadFile(ctx, problemID, "tests/"+t.Manual)
			if err != nil {
				continue
			}

			// Try .out first, then .ans
			ansFile := strings.TrimSuffix(t.Manual, ".in") + ".out"
			outputBytes, err := r.workshop.workspaceStorage.ReadFile(ctx, problemID, "tests/"+ansFile)
			if err != nil {
				ansFile = strings.TrimSuffix(t.Manual, ".in") + ".ans"
				outputBytes, err = r.workshop.workspaceStorage.ReadFile(ctx, problemID, "tests/"+ansFile)
			}

			if err != nil {
				continue
			}

			samples = append(samples, problemsmodel.Sample{
				Input:  string(inputBytes),
				Output: string(outputBytes),
			})
		}
	}

	return samples
}

func (r *StatementReader) loadPackageStatementAndSamplesWithLang(
	ctx context.Context,
	problemID uuid.UUID,
	packageID uuid.UUID,
	lang string,
) (*problemsmodel.Statement, []problemsmodel.Sample) {
	if r.packages == nil || packageID == uuid.Nil {
		return nil, nil
	}

	pkg, err := r.packages.GetPackageByID(ctx, packageID)
	if err != nil || pkg.PackageHash == "" || pkg.ProblemID != problemID {
		return nil, nil
	}

	reader, err := r.packages.DownloadPackage(ctx, problemID, pkg.PackageHash)
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

	if err := ziputil.ExtractZip(zipReader, tempDir); err != nil {
		return nil, nil
	}

	gfmtPkg, err := gfmt.OpenPackage(tempDir)
	if err != nil || gfmtPkg == nil || gfmtPkg.Problem == nil {
		return nil, nil
	}

	var statement problemsmodel.Statement
	if lang == "" {
		lang = "en"
	}
	if data, err := os.ReadFile(filepath.Join(tempDir, "statements", lang+".md")); err == nil {
		statement = ParseStatementMarkdown(string(data))
	}
	if lang == "en" || strings.TrimSpace(statement.Title) == "" {
		statement.Title = gfmtPkg.Problem.Title
	}

	var samples []problemsmodel.Sample
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

			samples = append(samples, problemsmodel.Sample{
				Input:  string(inputBytes),
				Output: string(outputBytes),
			})
		}
	}

	return &statement, samples
}

func (r *StatementReader) loadWorkspaceStatementAndSamples(ctx context.Context, problemID uuid.UUID, lang string) (*problemsmodel.Statement, []problemsmodel.Sample) {
	if r.workshop == nil {
		return nil, nil
	}
	var statement *problemsmodel.Statement
	var samples []problemsmodel.Sample
	err := r.workshop.workspaceStorage.withPackage(ctx, problemID, func(ctx context.Context, _ *workspacePackage) error {
		statement = r.loadProblemStatementFiles(ctx, problemID, lang)
		samples = r.loadProblemSampleFiles(ctx, problemID)
		return nil
	})
	if err != nil {
		return nil, nil
	}
	return statement, samples
}

// StatementReader composes immutable package content with the current workspace.
// Callers apply problem or contest visibility before reading content.
type StatementReader struct {
	workshop *WorkshopUseCase
	packages StatementPackages
}
type StatementPackages interface {
	GetPackageByID(context.Context, uuid.UUID) (problemsmodel.ProblemPackage, error)
	DownloadPackage(context.Context, uuid.UUID, string) (io.ReadCloser, error)
}

func NewStatementReader(workspace *WorkspaceStorage, packages StatementPackages) *StatementReader {
	return &StatementReader{workshop: newWorkshop(workspace), packages: packages}
}
func (r *StatementReader) ReadStatement(ctx context.Context, problemID, packageID uuid.UUID, lang string) (*problemsmodel.Statement, []problemsmodel.Sample) {
	var statement *problemsmodel.Statement
	var samples []problemsmodel.Sample
	if packageID != uuid.Nil {
		statement, samples = r.loadPackageStatementAndSamplesWithLang(ctx, problemID, packageID, lang)
	}
	if statement == nil || len(samples) == 0 {
		fallback, fallbackSamples := r.loadWorkspaceStatementAndSamples(ctx, problemID, lang)
		if statement == nil {
			statement = fallback
		}
		if len(samples) == 0 {
			samples = fallbackSamples
		}
	}
	return statement, samples
}
