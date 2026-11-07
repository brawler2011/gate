package problems

import (
	"context"
	"testing"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProblemDetailsUseOneCanonicalPackage(t *testing.T) {
	ctx := problemActorContext()
	id := uuid.New()
	repo := newProblemImportMockProblemsRepo()
	repo.problems[id] = problemsmodel.Problem{ID: id, Title: "Catalog", TimeLimitMs: 9, MemoryLimitMb: 8}
	raw := storage.NewLocalStorage(t.TempDir())
	original := NewWorkspaceStorage(raw, "workspace", repo)
	editor := NewWorkshopUseCase(original, nil, allowProblemPermissions{})
	require.NoError(t, editor.InitProblemWorkshop(ctx, id, "Original"))
	objects := &packageFaultStorage{Storage: raw}
	changed := false
	objects.beforeDownload = func(ctx context.Context) error {
		if changed {
			return nil
		}
		changed = true
		return editor.WithWorkspace(ctx, id, func(ctx context.Context) error {
			manifest, err := editor.GetManifest(ctx, id)
			if err != nil {
				return err
			}
			manifest.Statement.Title = "Changed"
			manifest.Statement.Legend = "Changed legend"
			manifest.TimeLimitMs = 4321
			if err := editor.SaveManifest(ctx, id, manifest); err != nil {
				return err
			}
			return editor.UpdateProblemFile(ctx, problemsmodel.UpdateFileRequest{ProblemID: id, Path: "tests/01.in", Content: []byte("new sample")})
		})
	}
	uc := NewProblemsUseCase(repo, NewWorkspaceStorage(objects, "workspace", repo), newProblemTestTransactions(repo, nil), nil, nil, allowProblemPermissions{})
	result, err := uc.GetProblemDetails(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "Original", result.Problem.Title)
	require.Equal(t, 1000, result.Problem.TimeLimitMs)
	require.Equal(t, 256, result.Problem.MemoryLimitMb)
	require.Equal(t, "Problem description goes here.", result.Statement.Legend)
	require.Len(t, result.Samples, 1)
	require.Equal(t, "1 2\n", result.Samples[0].Input)
	actual, err := editor.GetManifest(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "Changed", actual.Statement.Title)
}

func TestLocalizedStatementUsesCapturedRevision(t *testing.T) {
	for _, lang := range []string{"en", "ru"} {
		t.Run(lang, func(t *testing.T) {
			ctx := problemActorContext()
			id := uuid.New()
			repo := newProblemImportMockProblemsRepo()
			raw := storage.NewLocalStorage(t.TempDir())
			original := NewWorkspaceStorage(raw, "workspace", repo)
			editor := NewWorkshopUseCase(original, nil, allowProblemPermissions{})
			require.NoError(t, editor.InitProblemWorkshop(ctx, id, "Original"))
			require.NoError(t, editor.UpdateProblemFile(ctx, problemsmodel.UpdateFileRequest{ProblemID: id, Path: "statements/ru.md", Content: []byte("<!-- legend -->\nOriginal translation\n")}))
			objects := &packageFaultStorage{Storage: raw}
			changed := false
			objects.beforeDownload = func(ctx context.Context) error {
				if changed {
					return nil
				}
				changed = true
				return editor.WithWorkspace(ctx, id, func(ctx context.Context) error {
					manifest, err := editor.GetManifest(ctx, id)
					if err != nil {
						return err
					}
					manifest.Statement.Title = "Changed"
					if err := editor.SaveManifest(ctx, id, manifest); err != nil {
						return err
					}
					return editor.UpdateProblemFile(ctx, problemsmodel.UpdateFileRequest{ProblemID: id, Path: "statements/ru.md", Content: []byte("<!-- legend -->\nNew translation\n")})
				})
			}
			reader := NewWorkshopUseCase(NewWorkspaceStorage(objects, "workspace", repo), nil, allowProblemPermissions{})
			result, err := reader.GetStatement(ctx, id, lang)
			require.NoError(t, err)
			require.Equal(t, "Original", result.Statement.Title)
			if lang == "en" {
				require.Equal(t, "Problem description goes here.", result.Statement.Legend)
			} else {
				require.Equal(t, "Original translation", result.Statement.Legend)
			}
			current, err := editor.GetManifest(ctx, id)
			require.NoError(t, err)
			require.Equal(t, "Changed", current.Statement.Title)
		})
	}
}
