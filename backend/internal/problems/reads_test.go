package problems

import (
	"context"
	"errors"
	"testing"

	paging "github.com/brawler2011/gate/backend/internal/page"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type problemReads struct {
	Reads
	filter *problemsmodel.ProblemsFilter
	list   *problemsmodel.ProblemsList
}

func (r *problemReads) ListProblems(_ context.Context, filter *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error) {
	r.filter = filter
	if r.list == nil {
		return &problemsmodel.ProblemsList{}, nil
	}
	return r.list, nil
}

type templateOrganizations struct {
	id  uuid.UUID
	err error
}

func (r templateOrganizations) GetUserOrganizations(context.Context, uuid.UUID) ([]organizationsmodel.Organization, error) {
	if r.err != nil {
		return nil, r.err
	}
	return []organizationsmodel.Organization{{ID: r.id}}, nil
}

type templateManifests struct{ id uuid.UUID }

func (r *templateManifests) GetDraft(_ context.Context, id uuid.UUID) (problemsmodel.Draft, error) {
	r.id = id
	return problemsmodel.Draft{Type: "interactive", Files: map[string]string{"statements/en.md": "Template description"}}, nil
}

func TestProblemReadsBindViewerAndPagination(t *testing.T) {
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	ctx := accessmodel.WithUser(context.Background(), actor)
	reads := &problemReads{}
	reader := NewReader(reads, nil, nil, nil)
	other := uuid.New()
	filter := &problemsmodel.ProblemsFilter{ViewerID: &other}
	original := *filter
	_, err := reader.ListProblems(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, actor.Id, *reads.filter.ViewerID)
	require.EqualValues(t, 1, reads.filter.Page)
	require.EqualValues(t, 50, reads.filter.PageSize)
	require.Equal(t, other, *filter.ViewerID)
	require.Equal(t, original, *filter)
	for _, filter := range []*problemsmodel.ProblemsFilter{nil, {Page: -1}, {PageSize: -1}, {PageSize: 101}} {
		reads.filter = nil
		_, err := reader.ListProblems(ctx, filter)
		require.ErrorIs(t, err, pkg.ErrBadInput)
		require.Nil(t, reads.filter)
	}
	for _, size := range []int32{1, 100} {
		filter := &problemsmodel.ProblemsFilter{Page: 2, PageSize: size, ViewerID: &other}
		original := *filter
		_, err := reader.ListProblems(ctx, filter)
		require.NoError(t, err)
		require.EqualValues(t, 2, reads.filter.Page)
		require.Equal(t, size, reads.filter.PageSize)
		require.Equal(t, actor.Id, *reads.filter.ViewerID)
		require.Equal(t, original, *filter)
	}
	for _, desc := range []bool{true, false} {
		descVal := desc
		filter := &problemsmodel.ProblemsFilter{Descending: &descVal, ViewerID: &other}
		original := *filter
		_, err := reader.ListProblems(ctx, filter)
		require.NoError(t, err)
		require.NotNil(t, reads.filter.Descending)
		require.Equal(t, desc, *reads.filter.Descending)
		require.Equal(t, original, *filter)
	}
	_, err = reader.ListProblems(context.Background(), &problemsmodel.ProblemsFilter{})
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, *reads.filter.ViewerID)
}

func TestProblemPrivateReadsRejectBeforeStorage(t *testing.T) {
	reader := NewReader(nil, denyProblemPermissions{}, nil, nil)
	user := uuid.New()
	for _, ctx := range []context.Context{context.Background(), problemActorContext()} {
		expected := pkg.NoPermission
		if accessmodel.UserFromContext(ctx).IsGuest() {
			expected = pkg.ErrUnauthenticated
		}
		_, err := reader.ListProblemMembers(ctx, uuid.New(), paging.Request{})
		require.ErrorIs(t, err, expected)
		_, err = reader.GetProblemTeams(ctx, uuid.New())
		require.ErrorIs(t, err, expected)
		_, err = reader.ListDashboardProblems(ctx, user, 5)
		require.ErrorIs(t, err, expected)
	}
}

func TestTemplateCatalogUsesVisibleProblemsAndApplicationMapping(t *testing.T) {
	id, org := uuid.New(), uuid.New()
	reads := &problemReads{list: &problemsmodel.ProblemsList{Problems: []problemsmodel.Problem{{ID: id, Title: "Organization template"}}}}
	manifests := &templateManifests{}
	reader := NewReader(reads, nil, templateOrganizations{id: org}, manifests)
	ctx := problemActorContext()
	templates, err := reader.ListTemplates(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Greater(t, len(templates), 1)
	require.True(t, templates[0].IsBuiltin)
	require.Equal(t, Template{ID: id.String(), Title: "Organization template", Description: "Template description", ProblemType: "interactive"}, templates[len(templates)-1])
	require.Equal(t, id, manifests.id)
	require.Equal(t, org, *reads.filter.OrganizationID)
	require.Equal(t, accessmodel.UserFromContext(ctx).Id, *reads.filter.ViewerID)
	require.True(t, *reads.filter.IsTemplate)
	_, err = reader.ListTemplates(context.Background(), org)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	expected := errors.New("organizations unavailable")
	_, err = NewReader(nil, nil, templateOrganizations{err: expected}, nil).ListTemplates(ctx, uuid.Nil)
	require.ErrorIs(t, err, expected)
}

type denyProblemPermissions struct{}

func (denyProblemPermissions) HasProblemPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ProblemAction) (bool, error) {
	return false, nil
}
