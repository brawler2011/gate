package problems

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// publicProblems makes every problem public with no role for the caller.
type publicProblems struct{ accesstest.Loader }

func (publicProblems) ProblemFacts(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.ProblemFacts, error) {
	facts := map[uuid.UUID]access.ProblemFacts{}
	for _, id := range ids {
		facts[id] = access.ProblemFacts{ID: id, Public: true}
	}
	return facts, nil
}

func TestPublicProblemHidesPackagesAndDraft(t *testing.T) {
	id := uuid.New()
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
	packages := &PackageUseCase{Access: publicProblems{}}
	drafts := &DraftUseCase{Access: publicProblems{}}

	_, err := packages.ListPackages(ctx, id, paging.Request{})
	require.ErrorIs(t, err, pkg.ErrForbidden)
	_, err = packages.DownloadPackage(ctx, id, uuid.New())
	require.ErrorIs(t, err, pkg.ErrForbidden)
	_, err = drafts.GetDraft(ctx, id)
	require.ErrorIs(t, err, pkg.ErrForbidden)
}
