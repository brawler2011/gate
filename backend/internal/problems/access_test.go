package problems

import (
	"context"
	"testing"

	paging "github.com/brawler2011/gate/backend/internal/page"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type failedProblemPermissions struct{ err error }

func (p failedProblemPermissions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return false, p.err
}

func TestProblemReadsAndDraftOperationsAuthorizeBeforeAnyDependency(t *testing.T) {
	for _, failure := range []error{nil, pkg.ErrNotFound, pkg.ErrInternal} {
		permissions := failedProblemPermissions{failure}
		// No repository, transaction or object storage dependency is available:
		// every operation must be rejected by authorization alone.
		reader := &ProblemsUseCase{permissions: permissions}
		packages := &PackageUseCase{Permissions: permissions}
		drafts := &DraftUseCase{Permissions: permissions}
		id := uuid.New()
		calls := map[string]func(context.Context) error{
			"details": func(ctx context.Context) error { _, err := reader.GetProblemDetails(ctx, id); return err },
			"packages": func(ctx context.Context) error {
				_, err := packages.ListPackages(ctx, id, paging.Request{})
				return err
			},
			"download": func(ctx context.Context) error { _, err := packages.DownloadPackage(ctx, id, uuid.New()); return err },
			"draft":    func(ctx context.Context) error { _, err := drafts.GetDraft(ctx, id); return err },
			"save": func(ctx context.Context) error {
				_, err := drafts.SaveDraft(ctx, id, 1, "key", problemsmodel.Draft{})
				return err
			},
			"build":  func(ctx context.Context) error { _, err := drafts.RequestBuild(ctx, id, 1, "key"); return err },
			"import": func(ctx context.Context) error { _, err := drafts.Import(ctx, id, 1, "key", nil); return err },
		}
		for name, call := range calls {
			t.Run(name, func(t *testing.T) {
				readOnly := name == "details" || name == "packages" || name == "download" || name == "draft"
				guestError := pkg.ErrUnauthenticated
				if readOnly {
					guestError = failure
					if guestError == nil {
						guestError = pkg.NoPermission
					}
				}
				require.ErrorIs(t, call(context.Background()), guestError)
				expected := failure
				if expected == nil {
					expected = pkg.NoPermission
				}
				require.ErrorIs(t, call(problemActorContext()), expected)
			})
		}
	}
}

func problemActorContext() context.Context {
	return accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
}
