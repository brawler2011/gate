package problems

import (
	"bytes"
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/sandbox"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Every built-in template must import into a draft that satisfies the draft
// invariants, because new problems are created from them.
func TestBuiltinTemplatesImportIntoValidDrafts(t *testing.T) {
	quotas := model.DraftQuotas{FileBytes: 1 << 20, TextBytes: 4 << 20, Files: 1000, Tests: 1000, Components: 100, Subtasks: 100}
	uc := &DraftUseCase{
		TempDir: t.TempDir(), UploadTTL: 1e10, ObjectBytes: 1 << 30, DraftQuotas: quotas,
		BuildOptions: model.BuildOptions{
			Limits: model.BuildLimits{DiskBytes: 1 << 30, FileCount: 1000},
			Toolchain: model.Toolchain{Languages: map[string]sandbox.Language{
				"cpp": {}, "python": {}, "go": {}, "java": {},
			}},
		},
	}
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
			require.NoError(t, plan.draft.Validate(quotas, false))
			require.NotEmpty(t, plan.draft.Tests)
			require.NotEmpty(t, plan.draft.Components)
		})
	}
}
