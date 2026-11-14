package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/sandbox"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var importQuotas = model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 1000, Tests: 1000, Components: 100, Subtasks: 100}

func newImportUseCase(t *testing.T) *DraftUseCase {
	return &DraftUseCase{
		TempDir: t.TempDir(), UploadTTL: 1e10, ObjectBytes: 1 << 30, DraftQuotas: importQuotas,
		BuildOptions: model.BuildOptions{
			Limits: model.BuildLimits{DiskBytes: 1 << 30, FileCount: 1000},
			Toolchain: model.Toolchain{Languages: map[string]sandbox.Language{
				"cpp": {}, "python": {}, "go": {}, "java": {},
			}},
		},
	}
}

// Every built-in template must import into a draft that satisfies the draft
// invariants, because new problems are created from them.
func TestBuiltinTemplatesImportIntoValidDrafts(t *testing.T) {
	uc := newImportUseCase(t)
	for _, template := range templates.ListBuiltinTemplates() {
		id := template.ID
		t.Run(id, func(t *testing.T) {
			data, err := templates.GetBuiltinTemplateZip(id)
			require.NoError(t, err)
			problem := uuid.New()
			plan, err := uc.parseImport(context.Background(), bytes.NewReader(data), problem, 1)
			require.NoError(t, err)
			defer plan.Close()
			plan.draft.ProblemID, plan.draft.Revision = problem, 1
			require.NoError(t, plan.draft.Validate(importQuotas, false))
			require.NotEmpty(t, plan.draft.Tests)
			require.NotEmpty(t, plan.draft.Components)
		})
	}
}

// Tests name generators by file stem, so echo.cpp and echo.py together are ambiguous.
func TestImportRejectsGeneratorsWithOneStem(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS("../../pkg/testdata/gfmt/a-plus-b")))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "generators/echo.py"), []byte("print(0, 0)\n"), 0o600))
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	require.NoError(t, w.AddFS(os.DirFS(dir)))
	require.NoError(t, w.Close())

	_, err := newImportUseCase(t).parseImport(context.Background(), &archive, uuid.New(), 1)
	require.ErrorContains(t, err, "several generators named echo")
}
