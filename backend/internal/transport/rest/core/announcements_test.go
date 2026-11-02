package core

import (
	"context"
	"errors"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type announcementsContestStub struct {
	interfaces.ContestsUC
	resolve func(context.Context, string, string) (models.Contest, error)
}

func (s announcementsContestStub) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (models.Contest, error) {
	return s.resolve(ctx, orgLogin, contestLogin)
}

type announcementsReaderStub struct {
	read func(context.Context, uuid.UUID, int32, int32) (*corev1.ListContestAnnouncementsResponseModel, error)
}

func (s announcementsReaderStub) ListAnnouncementResponses(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*corev1.ListContestAnnouncementsResponseModel, error) {
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
			response := &corev1.ListContestAnnouncementsResponseModel{
				Announcements: []corev1.ContestAnnouncementModel{{ID: uuid.New()}},
				Pagination:    corev1.PaginationModel{Page: 3, Total: 4},
			}
			contests := announcementsContestStub{resolve: func(gotCtx context.Context, orgLogin, contestLogin string) (models.Contest, error) {
				require.Equal(t, ctx, gotCtx)
				require.Equal(t, params.OrgLogin, orgLogin)
				require.Equal(t, params.ContestLogin, contestLogin)
				return models.Contest{ID: contestID}, tc.resolveErr
			}}
			called := false
			reader := announcementsReaderStub{read: func(gotCtx context.Context, id uuid.UUID, page, pageSize int32) (*corev1.ListContestAnnouncementsResponseModel, error) {
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
			h := NewCoreServer(nil, contests, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, reader, nil, nil, nil)
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
				require.Same(t, response, got)
			}
		})
	}
}
