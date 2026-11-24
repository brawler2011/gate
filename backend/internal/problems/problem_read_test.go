package problems

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
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

type latestPackage struct {
	PackagesRepo
	id uuid.UUID
}

func (r latestPackage) GetReadyPackage(context.Context, uuid.UUID) (model.ProblemPackage, error) {
	return model.ProblemPackage{ID: r.id}, nil
}

func TestPublicProblemShowsOnlyItsStatement(t *testing.T) {
	id := uuid.New()
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
	statement := storedStatement{p: model.PackageStatement{ProblemID: id, Title: "Sum", Statements: []model.BuiltStatement{{DraftStatement: model.DraftStatement{Language: "en"}, HTML: map[string]string{"legend": "Add"}}}}}
	packages := &PackageUseCase{Repo: latestPackage{id: uuid.New()}, Content: &PackageContent{Repo: statement}, Access: publicProblems{}}
	drafts := &DraftUseCase{Access: publicProblems{}}

	_, content, ok, err := packages.Statement(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Add", content.Statement.Legend)

	_, err = packages.ListPackages(ctx, id, paging.Request{})
	require.ErrorIs(t, err, pkg.ErrForbidden)
	_, err = packages.DownloadPackage(ctx, id, uuid.New())
	require.ErrorIs(t, err, pkg.ErrForbidden)
	_, err = drafts.GetDraft(ctx, id)
	require.ErrorIs(t, err, pkg.ErrForbidden)
}
