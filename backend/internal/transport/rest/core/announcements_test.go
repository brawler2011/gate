package core

import (
	"context"
	"errors"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type announcementsContestStub struct {
	ContestsUC
	resolve func(context.Context, string, string) (contestsmodel.Contest, error)
}

func (s announcementsContestStub) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	return s.resolve(ctx, orgLogin, contestLogin)
}

type announcementsReaderStub struct {
	AnnouncementsUC
	read func(context.Context, uuid.UUID, int32, int32) (*contestsmodel.ContestAnnouncementsList, error)
}

func (s announcementsReaderStub) ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*contestsmodel.ContestAnnouncementsList, error) {
	return s.read(ctx, contestID, page, pageSize)
}

func TestListContestAnnouncementsReader(t *testing.T) {
	ctx := context.Background()
	contestID := uuid.New()
	params := corev1.ListContestAnnouncementsParams{
		OrgLogin: "org", ContestLogin: "contest", Page: corev1.NewOptInt32(3), PageSize: corev1.NewOptInt32(7),
	}
	readErr := errors.New("read failed")
	for _, tc := range []struct {
		name       string
		resolveErr error
		readErr    error
	}{
		{name: "response"},
		{name: "read error", readErr: readErr},
		{name: "missing contest", resolveErr: pkg.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &contestsmodel.ContestAnnouncementsList{
				Announcements: []contestsmodel.ContestAnnouncement{{ID: uuid.New()}},
				Pagination:    paging.Pagination{Page: 3, PageSize: 50, TotalItems: 175, TotalPages: 4},
			}
			contests := announcementsContestStub{resolve: func(gotCtx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
				require.Equal(t, ctx, gotCtx)
				require.Equal(t, params.OrgLogin, orgLogin)
				require.Equal(t, params.ContestLogin, contestLogin)
				return contestsmodel.Contest{ID: contestID}, tc.resolveErr
			}}
			called := false
			reader := announcementsReaderStub{read: func(gotCtx context.Context, id uuid.UUID, page, pageSize int32) (*contestsmodel.ContestAnnouncementsList, error) {
				called = true
				require.Equal(t, ctx, gotCtx)
				require.Equal(t, contestID, id)
				require.Equal(t, int32(3), page)
				require.Equal(t, int32(7), pageSize)
				if tc.readErr != nil {
					return nil, tc.readErr
				}
				return response, nil
			}}
			h := NewCoreServer(Services{Contests: contests, Announcements: reader}, false)
			got, err := h.ListContestAnnouncements(ctx, params)
			switch {
			case tc.resolveErr != nil:
				require.ErrorIs(t, err, tc.resolveErr)
				require.False(t, called)
				require.Nil(t, got)
			case tc.readErr != nil:
				require.ErrorIs(t, err, tc.readErr)
				require.True(t, called)
				require.Nil(t, got)
			default:
				require.NoError(t, err)
				require.True(t, called)
				require.Equal(t, response.Announcements[0].ID, got.Announcements[0].ID)
				require.Equal(t, response.Pagination.Page, got.Pagination.Page)
				require.Equal(t, response.Pagination.TotalPages, got.Pagination.TotalPages)
			}
		})
	}
}
