package problems

import (
	"context"
	"testing"

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

func TestProblemReadsAndPreflightsAuthorizeWithoutTransactions(t *testing.T) {
	for _, failure := range []error{nil, pkg.ErrNotFound, pkg.ErrInternal} {
		permissions := failedProblemPermissions{failure}
		// No transaction, repository, or object storage dependency is available.
		reader := NewProblemsUseCase(nil, nil, nil, nil, nil, permissions)
		publisher := NewProblemPublishUseCase(nil, nil, nil, nil, "", nil, permissions)
		importer := NewProblemImportUseCase(nil, nil, nil, permissions)
		id := uuid.New()
		calls := map[string]func(context.Context) error{
			"details":     func(ctx context.Context) error { _, err := reader.GetProblemDetails(ctx, id); return err },
			"packages":    func(ctx context.Context) error { _, err := publisher.ListPackages(ctx, id); return err },
			"download":    func(ctx context.Context) error { _, err := publisher.GetPublishedPackageURL(ctx, id, "v1"); return err },
			"template":    func(ctx context.Context) error { _, err := publisher.GetReadyPackage(ctx, id); return err },
			"publication": func(ctx context.Context) error { _, err := publisher.PublishProblem(ctx, id); return err },
			"import":      func(ctx context.Context) error { _, err := importer.ImportProblemPackage(ctx, nil, 0, id); return err },
			"update":      func(ctx context.Context) error { return reader.UpdateProblem(ctx, id, &problemsmodel.ProblemUpdate{}) },
		}
		for name, call := range calls {
			t.Run(name, func(t *testing.T) {
				require.ErrorIs(t, call(context.Background()), pkg.ErrUnauthenticated)
				expected := failure
				if expected == nil {
					expected = pkg.NoPermission
				}
				require.ErrorIs(t, call(problemActorContext()), expected)
			})
		}
	}
}
