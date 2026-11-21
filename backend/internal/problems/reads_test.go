package problems

import (
	"context"
	"errors"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type problemReads struct {
	ProblemsRepo
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

type templatePackages struct{ id uuid.UUID }

func (r *templatePackages) GetReadyPackage(_ context.Context, id uuid.UUID) (problemsmodel.ProblemPackage, error) {
	r.id = id
	return problemsmodel.ProblemPackage{Title: "Organization template", Type: "interactive", Statements: []problemsmodel.DraftStatement{{Language: "en", Legend: "Template description"}}}, nil
}

func TestTemplateCatalogUsesVisibleProblemsAndApplicationMapping(t *testing.T) {
	id, org := uuid.New(), uuid.New()
	reads := &problemReads{list: &problemsmodel.ProblemsList{Problems: []problemsmodel.Problem{{ID: id, Title: "Organization template"}}}}
	manifests := &templatePackages{}
	reader := NewProblemsUseCase(reads, nil, templateOrganizations{id: org}, nil, manifests)
	ctx := problemActorContext()
	templates, err := reader.ListTemplates(ctx, uuid.Nil)
	require.NoError(t, err)
	require.Greater(t, len(templates), 1)
	require.True(t, templates[0].IsBuiltin)
	require.Equal(t, Template{ID: id.String(), Title: "Organization template", Description: "Template description", ProblemType: "interactive"}, templates[len(templates)-1])
	require.Equal(t, id, manifests.id)
	require.Equal(t, org, *reads.filter.OrganizationID)
	require.True(t, *reads.filter.IsTemplate)
	_, err = reader.ListTemplates(context.Background(), org)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
	expected := errors.New("organizations unavailable")
	_, err = NewProblemsUseCase(nil, nil, templateOrganizations{err: expected}, nil, nil).ListTemplates(ctx, uuid.Nil)
	require.ErrorIs(t, err, expected)
}

func problemActorContext() context.Context {
	return accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
}
