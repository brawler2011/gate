package builder

import (
	"testing"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/stretchr/testify/require"
)

// Auto-updated library references must materialize system text for the judge.
func TestPlaceDraftPackagesSystemLibraryText(t *testing.T) {
	d := model.Draft{DraftContent: model.DraftContent{
		Library: []model.DraftLibraryFile{{Name: "testlib.h", AutoUpdate: true}, {Name: "graph.h", Content: "// graph\n"}},
	}}
	l := placeDraft(d, &sandbox.Config{})
	system, _ := syslib.Lookup("testlib.h")
	require.Equal(t, system.Content, l.texts["lib/testlib.h"])
	require.Equal(t, "// graph\n", l.texts["lib/graph.h"])
}
