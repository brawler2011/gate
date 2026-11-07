package organizations

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type teamsListRepo struct {
	TeamsRepo
	filter    organizationsmodel.TeamsFilter
	caller    uuid.UUID
	canManage bool
	err       error
}

func (r *teamsListRepo) ListTeams(_ context.Context, filter *organizationsmodel.TeamsFilter, caller uuid.UUID, canManage bool) ([]organizationsmodel.Team, int32, error) {
	r.filter, r.caller, r.canManage = *filter, caller, canManage
	return []organizationsmodel.Team{}, 7, r.err
}

type teamsListPermissions struct {
	view, manage bool
	actions      []accessmodel.OrgAction
	errAction    accessmodel.OrgAction
	caller, org  uuid.UUID
}

func (p *teamsListPermissions) HasOrganizationPermission(_ context.Context, org, caller uuid.UUID, action accessmodel.OrgAction) (bool, error) {
	p.actions = append(p.actions, action)
	p.caller, p.org = caller, org
	if action == p.errAction {
		return false, context.Canceled
	}
	if action == accessmodel.ActionViewOrganization {
		return p.view, nil
	}
	return p.manage, nil
}

func TestListTeamsScopeAndPagination(t *testing.T) {
	org := uuid.New()
	caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	for _, tc := range []struct {
		name       string
		org        *uuid.UUID
		manage     bool
		page, size int32
	}{
		{name: "own teams"},
		{name: "organization member", org: &org, page: 2, size: 1},
		{name: "organization manager", org: &org, manage: true, page: 3, size: 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &teamsListRepo{}
			permissions := &teamsListPermissions{view: true, manage: tc.manage}
			uc := NewTeamsUseCase(repo, permissions, nil)
			filter := &organizationsmodel.TeamsFilter{OrganizationID: tc.org, Search: "КоМ_%", Page: tc.page, PageSize: tc.size}
			original := *filter
			result, err := uc.ListTeams(accessmodel.WithUser(context.Background(), caller), filter)
			require.NoError(t, err)
			require.Equal(t, original, *filter, "normalization must not mutate the caller's filter")
			require.Equal(t, caller.Id, repo.caller)
			require.Equal(t, tc.org, repo.filter.OrganizationID)
			require.Equal(t, "КоМ_%", repo.filter.Search)
			require.Equal(t, tc.manage, repo.canManage)
			if tc.page == 0 {
				require.Equal(t, int32(1), repo.filter.Page)
				require.Equal(t, int32(50), repo.filter.PageSize)
			} else {
				require.Equal(t, tc.page, repo.filter.Page)
				require.Equal(t, tc.size, repo.filter.PageSize)
			}
			require.Equal(t, repo.filter.Page, result.Pagination.Page)
			require.Equal(t, int32(7), result.Pagination.TotalItems, "total_items counts all matching records even for an empty page")
			if tc.org == nil {
				require.Empty(t, permissions.actions)
			} else {
				require.Equal(t, []accessmodel.OrgAction{accessmodel.ActionViewOrganization, accessmodel.ActionManageOrganization}, permissions.actions)
				require.Equal(t, caller.Id, permissions.caller)
				require.Equal(t, org, permissions.org)
			}
		})
	}
}

func TestListTeamsRejectsInvalidRequests(t *testing.T) {
	caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	expired := caller
	past := time.Now().Add(-time.Hour)
	expired.ExpiresAt = &past
	for _, tc := range []struct {
		name   string
		caller accessmodel.User
		filter *organizationsmodel.TeamsFilter
		err    error
	}{
		{name: "guest", caller: accessmodel.Guest, filter: &organizationsmodel.TeamsFilter{}, err: pkg.ErrUnauthenticated},
		{name: "expired", caller: expired, filter: &organizationsmodel.TeamsFilter{}, err: pkg.ErrUnauthenticated},
		{name: "nil filter", caller: caller, err: pkg.ErrBadInput},
		{name: "negative page", caller: caller, filter: &organizationsmodel.TeamsFilter{Page: -1}, err: pkg.ErrBadInput},
		{name: "negative size", caller: caller, filter: &organizationsmodel.TeamsFilter{PageSize: -1}, err: pkg.ErrBadInput},
		{name: "oversized", caller: caller, filter: &organizationsmodel.TeamsFilter{PageSize: 101}, err: pkg.ErrBadInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Missing dependencies ensure invalid calls cannot reach permissions or storage.
			uc := NewTeamsUseCase(nil, nil, nil)
			_, err := uc.ListTeams(accessmodel.WithUser(context.Background(), tc.caller), tc.filter)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func TestListTeamsPropagatesFailures(t *testing.T) {
	org := uuid.New()
	caller := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	for _, tc := range []struct {
		name        string
		permissions teamsListPermissions
		repoErr     error
		wantErr     error
	}{
		{name: "view denied", wantErr: pkg.NoPermission},
		{name: "view check fails", permissions: teamsListPermissions{errAction: accessmodel.ActionViewOrganization}, wantErr: context.Canceled},
		{name: "manage check fails", permissions: teamsListPermissions{view: true, errAction: accessmodel.ActionManageOrganization}, wantErr: context.Canceled},
		{name: "storage fails", permissions: teamsListPermissions{view: true}, repoErr: context.Canceled, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var repo TeamsRepo
			if tc.repoErr != nil {
				repo = &teamsListRepo{err: tc.repoErr}
			}
			uc := NewTeamsUseCase(repo, &tc.permissions, nil)
			_, err := uc.ListTeams(accessmodel.WithUser(context.Background(), caller), &organizationsmodel.TeamsFilter{OrganizationID: &org})
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}
