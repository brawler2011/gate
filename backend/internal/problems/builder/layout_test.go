package builder

import (
	"testing"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// An auto-updated library file stores no text; the package must carry the
// system text, or the judge cannot compile the checker.
func TestPlaceDraftPackagesSystemTestlib(t *testing.T) {
	auto := uuid.New()
	d := model.Draft{DraftContent: model.DraftContent{
		Library: []model.DraftLibraryFile{{ID: auto, Name: "testlib.h", AutoUpdate: true}},
		Checker: &model.DraftProgram{Language: "cpp17", Dependencies: []uuid.UUID{auto}},
	}}
	l := placeDraft(d, &sandbox.Config{})
	system, _ := syslib.Lookup("testlib.h")
	require.Equal(t, system.Content, l.texts[l.programs[0].dependencies["testlib.h"]])
}
