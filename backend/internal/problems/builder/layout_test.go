package builder

import (
	"testing"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Judge programs compile with the whole library, an auto-updated file with
// the system text the package must carry; solutions compile alone.
func TestPlaceDraftSharesLibraryWithJudgePrograms(t *testing.T) {
	d := model.Draft{DraftContent: model.DraftContent{
		Library:   []model.DraftLibraryFile{{Name: "testlib.h", AutoUpdate: true}, {Name: "graph.h", Content: "// graph\n"}},
		Checker:   &model.DraftProgram{Language: "cpp17"},
		Solutions: []model.DraftNamedProgram{{ID: uuid.New(), Name: "main", DraftProgram: model.DraftProgram{Language: "cpp17"}}},
	}}
	l := placeDraft(d, &sandbox.Config{})
	system, _ := syslib.Lookup("testlib.h")
	checker, solution := l.programs[0], l.programs[1]
	require.Equal(t, "checker", checker.role)
	require.Equal(t, system.Content, l.texts[checker.dependencies["testlib.h"]])
	require.Equal(t, "// graph\n", l.texts[checker.dependencies["graph.h"]])
	require.Empty(t, solution.dependencies)
}
