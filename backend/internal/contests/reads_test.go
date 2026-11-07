package contests

import (
	"context"
	"errors"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type contestReads struct {
	Reads
	page, size    int32
	user          uuid.UUID
	login, search string
	called        bool
}

func (r *contestReads) ListOrganizationContests(_ context.Context, login, search string, page, size int32) (*contestsmodel.ContestsList, error) {
	r.called, r.login, r.search, r.page, r.size = true, login, search, page, size
	return &contestsmodel.ContestsList{}, nil
}
func (r *contestReads) ListAdminContests(_ context.Context, filter contestsmodel.AdminContestsFilter) (*contestsmodel.ContestsList, error) {
	r.called, r.page, r.size = true, filter.Page, filter.PageSize
	return &contestsmodel.ContestsList{}, nil
}
func (r *contestReads) ListUserContests(_ context.Context, filter contestsmodel.UserContestsFilter) (*contestsmodel.ContestsList, error) {
	r.called, r.page, r.size, r.user = true, filter.Page, filter.PageSize, filter.UserId
	return &contestsmodel.ContestsList{}, nil
}
func (r *contestReads) ListContestMembers(_ context.Context, filter contestsmodel.ParticipantsFilter) (*contestsmodel.ContestMembersList, error) {
	r.called, r.page, r.size = true, filter.Page, filter.PageSize
	return &contestsmodel.ContestMembersList{}, nil
}

type rosterPermissions struct {
	actions []accessmodel.ContestAction
	allow   accessmodel.ContestAction
	err     error
}

func (p *rosterPermissions) HasContestPermission(_ context.Context, _, _ uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	p.actions = append(p.actions, action)
	return action == p.allow, p.err
}

func TestProtectedContestReadsRejectBeforeStorage(t *testing.T) {
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	user := uuid.New()
	for _, ctx := range []context.Context{context.Background(), accessmodel.WithUser(context.Background(), actor)} {
		reader := NewReader(nil, &rosterPermissions{})
		expected := pkg.NoPermission
		if accessmodel.UserFromContext(ctx).IsGuest() {
			expected = pkg.ErrUnauthenticated
		}
		_, err := reader.ListAdminContests(ctx, contestsmodel.AdminContestsFilter{})
		require.ErrorIs(t, err, expected)
		_, err = reader.ListUserContests(ctx, contestsmodel.UserContestsFilter{UserId: user})
		require.ErrorIs(t, err, expected)
		_, err = reader.ListWorkshopContests(ctx, contestsmodel.WorkshopContestsFilter{UserId: user})
		require.ErrorIs(t, err, expected)
		_, err = reader.ListDashboardContests(ctx, user, 5)
		require.ErrorIs(t, err, expected)
		_, err = reader.GetContestTeams(ctx, uuid.New())
		require.ErrorIs(t, err, expected)
		_, err = reader.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{})
		require.ErrorIs(t, err, expected)
	}
}

func (r *contestReads) ListWorkshopContests(_ context.Context, filter contestsmodel.WorkshopContestsFilter) (*contestsmodel.ContestsList, error) {
	r.called, r.page, r.size, r.user = true, filter.Page, filter.PageSize, filter.UserId
	return &contestsmodel.ContestsList{}, nil
}

func TestContestReadsNormalizePaginationAndValidateBeforeStorage(t *testing.T) {
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleAdmin}
	ctx := accessmodel.WithUser(context.Background(), actor)
	contest := uuid.New()
	for _, method := range []string{"organization", "admin", "user", "workshop", "members"} {
		t.Run(method, func(t *testing.T) {
			for _, tc := range []struct {
				name                           string
				page, size, wantPage, wantSize int32
				err                            error
			}{
				{name: "defaults", wantPage: 1, wantSize: 50},
				{name: "default page", size: 100, wantPage: 1, wantSize: 100},
				{name: "default size", page: 3, wantPage: 3, wantSize: 50},
				{name: "minimum", page: 1, size: 1, wantPage: 1, wantSize: 1},
				{name: "maximum size", page: 2, size: 100, wantPage: 2, wantSize: 100},
				{name: "negative page", page: -1, size: 50, err: pkg.ErrBadInput},
				{name: "negative size", page: 1, size: -1, err: pkg.ErrBadInput},
				{name: "oversized", page: 1, size: 101, err: pkg.ErrBadInput},
			} {
				t.Run(tc.name, func(t *testing.T) {
					reads := &contestReads{}
					permissions := &rosterPermissions{allow: accessmodel.ActionGetMonitor}
					reader := NewReader(reads, permissions)
					var err error
					switch method {
					case "organization":
						_, err = reader.ListOrganizationContests(ctx, "organization", "search", tc.page, tc.size)
					case "admin":
						_, err = reader.ListAdminContests(ctx, contestsmodel.AdminContestsFilter{Page: tc.page, PageSize: tc.size})
					case "user":
						_, err = reader.ListUserContests(ctx, contestsmodel.UserContestsFilter{UserId: actor.Id, Page: tc.page, PageSize: tc.size})
					case "workshop":
						_, err = reader.ListWorkshopContests(ctx, contestsmodel.WorkshopContestsFilter{UserId: actor.Id, Page: tc.page, PageSize: tc.size})
					case "members":
						_, err = reader.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{ContestId: contest, Page: tc.page, PageSize: tc.size})
					}
					require.ErrorIs(t, err, tc.err)
					require.Equal(t, tc.err == nil, reads.called)
					if tc.err != nil {
						require.Empty(t, permissions.actions)
						return
					}
					require.Equal(t, tc.wantPage, reads.page)
					require.Equal(t, tc.wantSize, reads.size)
					if method == "organization" {
						require.Equal(t, "organization", reads.login)
						require.Equal(t, "search", reads.search)
					}
					if method == "user" || method == "workshop" {
						require.Equal(t, actor.Id, reads.user)
					}
				})
			}
		})
	}
}

func TestContestRosterPreservesMonitorOrParticipantPermission(t *testing.T) {
	ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
	for _, action := range []accessmodel.ContestAction{accessmodel.ActionGetMonitor, accessmodel.ActionListOwnSubmissions} {
		reads := &contestReads{}
		permissions := &rosterPermissions{allow: action}
		reader := NewReader(reads, permissions)
		_, err := reader.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{})
		require.NoError(t, err)
		require.True(t, reads.called)
		require.EqualValues(t, 1, reads.page)
		require.EqualValues(t, 50, reads.size)
		require.Contains(t, permissions.actions, action)
	}
	expected := errors.New("authorization unavailable")
	_, err := NewReader(nil, &rosterPermissions{err: expected}).ListContestMembers(ctx, contestsmodel.ParticipantsFilter{})
	require.ErrorIs(t, err, expected)
}

func TestContestReadsPreserveValidationOrder(t *testing.T) {
	other := uuid.New()
	for _, ctx := range []context.Context{context.Background(), accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})} {
		reader := NewReader(nil, nil)
		want := pkg.NoPermission
		if accessmodel.UserFromContext(ctx).IsGuest() {
			want = pkg.ErrUnauthenticated
		}
		_, err := reader.ListAdminContests(ctx, contestsmodel.AdminContestsFilter{Page: -1})
		require.ErrorIs(t, err, want)
		_, err = reader.ListUserContests(ctx, contestsmodel.UserContestsFilter{UserId: other, Page: -1})
		require.ErrorIs(t, err, want)
		_, err = reader.ListWorkshopContests(ctx, contestsmodel.WorkshopContestsFilter{UserId: other, Page: -1})
		require.ErrorIs(t, err, want)
		// Roster pagination is checked before authorization, unlike the lists above.
		_, err = reader.ListContestMembers(ctx, contestsmodel.ParticipantsFilter{Page: -1})
		require.ErrorIs(t, err, pkg.ErrBadInput)
	}
	_, err := NewReader(nil, nil).ListOrganizationContests(context.Background(), "organization", "", -1, 50)
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
}
