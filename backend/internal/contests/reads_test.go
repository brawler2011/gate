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

func TestContestReadsNormalizePaginationAndValidateBeforeStorage(t *testing.T) {
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleAdmin}
	ctx := accessmodel.WithUser(context.Background(), actor)
	reads := &contestReads{}
	reader := NewReader(reads, nil)
	_, err := reader.ListOrganizationContests(ctx, "organization", "search", 0, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, reads.page)
	require.EqualValues(t, 50, reads.size)
	require.Equal(t, "organization", reads.login)
	require.Equal(t, "search", reads.search)
	_, err = reader.ListAdminContests(ctx, contestsmodel.AdminContestsFilter{})
	require.NoError(t, err)
	_, err = reader.ListUserContests(ctx, contestsmodel.UserContestsFilter{UserId: actor.Id})
	require.NoError(t, err)
	require.Equal(t, actor.Id, reads.user)
	for _, pair := range [][2]int32{{-1, 50}, {1, -1}, {1, 101}} {
		reads.called = false
		_, err := reader.ListOrganizationContests(ctx, "organization", "", pair[0], pair[1])
		require.ErrorIs(t, err, pkg.ErrBadInput)
		require.False(t, reads.called)
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
