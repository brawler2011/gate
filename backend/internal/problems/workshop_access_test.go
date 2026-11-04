package problems

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type workshopPermissions struct{ view, edit bool }

func (p workshopPermissions) HasProblemPermission(_ context.Context, _, _ uuid.UUID, action accessmodel.ProblemAction) (bool, error) {
	if action == accessmodel.ActionViewProblem {
		return p.view, nil
	}
	return p.edit, nil
}

func TestWorkshopEnforcesApplicationAccess(t *testing.T) {
	id := uuid.New()
	for _, ctx := range []context.Context{context.Background(), problemActorContext()} {
		expected := pkg.NoPermission
		if accessmodel.UserFromContext(ctx).IsGuest() {
			expected = pkg.ErrUnauthenticated
		}
		uc := NewWorkshopUseCase(nil, nil, workshopPermissions{})
		_, err := uc.GetManifest(ctx, id)
		require.ErrorIs(t, err, expected)
		_, err = uc.ReadProblemFile(ctx, id, "checkers/source.cpp")
		require.ErrorIs(t, err, expected)
		_, err = uc.GetStatement(ctx, id, "en")
		require.ErrorIs(t, err, expected)
		require.ErrorIs(t, uc.UpdateProblemFile(ctx, problemsmodel.UpdateFileRequest{ProblemID: id}), expected)
		require.ErrorIs(t, uc.SaveManifest(ctx, id, &problemsmodel.ProblemManifest{}), expected)
		require.ErrorIs(t, uc.DeleteProblemFile(ctx, id, "tests/01.in"), expected)
		require.ErrorIs(t, uc.GenerateTests(ctx, problemsmodel.GenerateTestsRequest{ProblemID: id}), expected)
		_, err = uc.TestSolution(ctx, problemsmodel.TestSolutionRequest{ProblemID: id})
		require.ErrorIs(t, err, expected)
	}
}

func TestProblemViewerCannotReadWorkshopSources(t *testing.T) {
	ctx := problemActorContext()
	id := uuid.New()
	workspace := NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "workspace", newProblemImportMockProblemsRepo())
	editor := NewWorkshopUseCase(workspace, nil, allowProblemPermissions{})
	require.NoError(t, editor.InitProblemWorkshop(ctx, id, "Visible"))
	reader := NewWorkshopUseCase(workspace, nil, workshopPermissions{view: true})
	statement, err := reader.GetStatement(ctx, id, "en")
	require.NoError(t, err)
	require.Equal(t, "Visible", statement.Statement.Title)
	_, err = reader.ReadProblemFile(ctx, id, "checkers/checker.cpp")
	require.ErrorIs(t, err, pkg.NoPermission)
	_, err = reader.ListProblemFiles(ctx, id, "tests")
	require.ErrorIs(t, err, pkg.NoPermission)
}

func TestWorkshopPatchesRejectConcurrentReferenceChange(t *testing.T) {
	for _, operation := range []string{"limits", "statement"} {
		t.Run(operation, func(t *testing.T) {
			ctx := problemActorContext()
			id := uuid.New()
			repo := newProblemImportMockProblemsRepo()
			raw := storage.NewLocalStorage(t.TempDir())
			original := NewWorkspaceStorage(raw, "workspace", repo)
			editor := NewWorkshopUseCase(original, nil, allowProblemPermissions{})
			require.NoError(t, editor.InitProblemWorkshop(ctx, id, "Original"))
			object := &packageFaultStorage{Storage: raw}
			changed := false
			object.beforeUpload = func(ctx context.Context, _ string) error {
				if changed {
					return nil
				}
				changed = true
				current, err := editor.GetManifest(ctx, id)
				if err != nil {
					return err
				}
				current.Statement.Title = "Concurrent"
				return editor.SaveManifest(ctx, id, current)
			}
			uc := NewWorkshopUseCase(NewWorkspaceStorage(object, "workspace", repo), nil, allowProblemPermissions{})
			var err error
			if operation == "limits" {
				limit := 4321
				_, err = uc.UpdateLimits(ctx, id, LimitsPatch{TimeLimitMs: &limit})
			} else {
				legend := "Stale"
				_, err = uc.UpdateStatement(ctx, id, "en", StatementPatch{Legend: &legend})
			}
			require.ErrorIs(t, err, pkg.ErrConflict)
			current, err := editor.GetManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "Concurrent", current.Statement.Title)
			require.Equal(t, 1000, current.TimeLimitMs)
			require.Equal(t, "Problem description goes here.", current.Statement.Legend)
		})
	}
}

func TestLimitsPatchPreservesAbsentNullAndZero(t *testing.T) {
	ctx := problemActorContext()
	id := uuid.New()
	uc := NewWorkshopUseCase(NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "workspace", newProblemImportMockProblemsRepo()), nil, allowProblemPermissions{})
	require.NoError(t, uc.InitProblemWorkshop(ctx, id, "Scoring"))
	scoring := "scoring"
	score := 100
	_, err := uc.UpdateLimits(ctx, id, LimitsPatch{ProblemType: &scoring, MaxScoreSet: true, MaxScore: &score})
	require.NoError(t, err)
	limit := 1234
	result, err := uc.UpdateLimits(ctx, id, LimitsPatch{TimeLimitMs: &limit})
	require.NoError(t, err)
	require.NotNil(t, result.MaxScore)
	require.Equal(t, 100, *result.MaxScore)
	result, err = uc.UpdateLimits(ctx, id, LimitsPatch{MaxScoreSet: true})
	require.NoError(t, err)
	require.Nil(t, result.MaxScore)
	zero := 0
	result, err = uc.UpdateLimits(ctx, id, LimitsPatch{MaxScoreSet: true, MaxScore: &zero})
	require.NoError(t, err)
	require.NotNil(t, result.MaxScore)
	require.Zero(t, *result.MaxScore)
	empty := ""
	_, err = uc.UpdateStatement(ctx, id, "en", StatementPatch{Title: &empty})
	require.ErrorIs(t, err, pkg.ErrBadInput)
}
