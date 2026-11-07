package organizations

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type organizationListRepo struct {
	OrganizationsRepo
	calls int
}

func (r *organizationListRepo) ListOrganizations(_ context.Context, filter *organizationsmodel.OrganizationFilter) ([]organizationsmodel.Organization, int32, error) {
	r.calls++
	return []organizationsmodel.Organization{}, 101, nil
}

func TestOrganizationListPaginationDefaults(t *testing.T) {
	caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	ctx := accessmodel.WithUser(context.Background(), caller)
	repo := &organizationListRepo{}
	uc := &OrganizationsUseCase{repo: repo}
	filter := &organizationsmodel.OrganizationFilter{}
	result, err := uc.ListOrganizations(ctx, filter, caller.Id)
	require.NoError(t, err)
	require.Equal(t, paging.Pagination{Page: 1, PageSize: 50, TotalItems: 101, TotalPages: 3}, result.Pagination)
	require.Zero(t, filter.PageSize, "normalizing must not mutate the caller's filter")
	for _, invalid := range []*organizationsmodel.OrganizationFilter{nil, {PageSize: 101}, {Page: -1}} {
		_, err := uc.ListOrganizations(ctx, invalid, caller.Id)
		require.ErrorIs(t, err, pkg.ErrBadInput)
	}
	require.Equal(t, 1, repo.calls)
}
