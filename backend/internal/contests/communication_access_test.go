package contests

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type communicationPermissions struct{ view, manage bool }

func (p communicationPermissions) HasContestPermission(_ context.Context, _, _ uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	if action == accessmodel.ActionManageContest {
		return p.manage, nil
	}
	return p.view, nil
}

type communicationReads struct {
	user       uuid.UUID
	moderator  bool
	called     bool
	page, size int32
}

func (r *communicationReads) ListAnnouncements(_ context.Context, _ uuid.UUID, page, size int32) (*contestsmodel.ContestAnnouncementsList, error) {
	r.called = true
	r.page, r.size = page, size
	return &contestsmodel.ContestAnnouncementsList{}, nil
}
func (r *communicationReads) ListClarificationsForUser(_ context.Context, _ uuid.UUID, user uuid.UUID, page, size int32) (*contestsmodel.ContestClarificationsList, error) {
	r.called = true
	r.user = user
	r.page, r.size = page, size
	return &contestsmodel.ContestClarificationsList{}, nil
}
func (r *communicationReads) ListClarificationsForModerator(_ context.Context, _ uuid.UUID, filter *contestsmodel.ContestClarificationsFilter) (*contestsmodel.ContestClarificationsList, error) {
	r.called = true
	r.moderator = true
	r.page, r.size = filter.Page, filter.PageSize
	return &contestsmodel.ContestClarificationsList{}, nil
}

func TestCommunicationReadsEnforceVisibility(t *testing.T) {
	for _, tc := range []struct {
		name                string
		guest, view, manage bool
		err                 error
	}{
		{name: "participant", view: true},
		{name: "moderator", view: true, manage: true},
		{name: "outsider", err: pkg.NoPermission},
		{name: "guest", guest: true, view: true, err: pkg.ErrUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := &communicationReads{}
			reader := newCommunicationReader(reads, communicationPermissions{tc.view, tc.manage})
			user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			ctx := context.Background()
			if !tc.guest {
				ctx = accessmodel.WithUser(ctx, user)
			}
			_, err := reader.ListClarifications(ctx, uuid.New(), &contestsmodel.ContestClarificationsFilter{Page: 1, PageSize: 50})
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.err == nil, reads.called)
			if tc.err == nil {
				require.Equal(t, tc.manage, reads.moderator)
				if !tc.manage {
					require.Equal(t, user.Id, reads.user)
				}
			}
			reads.called = false
			_, err = reader.ListAnnouncements(ctx, uuid.New(), 1, 50)
			if tc.view {
				require.NoError(t, err)
				require.True(t, reads.called)
			} else {
				require.ErrorIs(t, err, pkg.NoPermission)
				require.False(t, reads.called)
			}
		})
	}
}

// communicationReader exposes the announcement and clarification reads together.
type communicationReader struct {
	*AnnouncementsUseCase
	*ClarificationsUseCase
}

func newCommunicationReader(reads *communicationReads, permissions Permissions) communicationReader {
	return communicationReader{NewAnnouncementsUseCase(nil, reads, permissions), NewClarificationsUseCase(nil, reads, permissions)}
}

type deniedCommunicationStore struct {
	contestEventsTestStore
}

func (s *deniedCommunicationStore) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return false, nil
}

func TestCommunicationCommandsRequireTransactionBoundPermission(t *testing.T) {
	store := &deniedCommunicationStore{}
	transactions := testTransactions[ContestEventsTxStore]{store}
	actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	ctx := accessmodel.WithUser(context.Background(), actor)
	announcements := NewAnnouncementsUseCase(transactions, nil, nil)
	_, err := announcements.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{ContestID: uuid.New(), AuthorID: actor.Id, Title: "Title", Body: "Body"})
	require.ErrorIs(t, err, pkg.NoPermission)
	require.ErrorIs(t, announcements.DeleteAnnouncement(ctx, uuid.New(), uuid.New()), pkg.NoPermission)
	clarifications := NewClarificationsUseCase(transactions, nil, nil)
	_, err = clarifications.CreateClarification(ctx, &contestsmodel.CreateContestClarificationInput{ContestID: uuid.New(), UserID: actor.Id, Question: "Question"})
	require.ErrorIs(t, err, pkg.NoPermission)
	_, err = clarifications.AnswerClarification(ctx, &contestsmodel.AnswerContestClarificationInput{ContestID: uuid.New(), AnsweredBy: actor.Id, Answer: "Answer"})
	require.ErrorIs(t, err, pkg.NoPermission)
}

func TestClarificationsSettingCheckedInsideTransaction(t *testing.T) {
	store := contestEventsTestStore{Contest: contestsmodel.Contest{Settings: map[string]interface{}{"allow_clarifications": false}}}
	user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	uc := NewClarificationsUseCase(testTransactions[ContestEventsTxStore]{store}, nil, nil)
	_, err := uc.CreateClarification(accessmodel.WithUser(context.Background(), user), &contestsmodel.CreateContestClarificationInput{ContestID: uuid.New(), UserID: user.Id, Question: "Question"})
	require.ErrorIs(t, err, pkg.NoPermission)
}

func TestCommunicationPagination(t *testing.T) {
	user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	ctx := accessmodel.WithUser(context.Background(), user)
	for _, method := range []string{"announcements", "user clarifications", "moderator clarifications"} {
		t.Run(method, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				page, size int32
				err        error
			}{
				{name: "minimum", page: 1, size: 1},
				{name: "maximum size", page: 2, size: 100},
				{name: "zeros", err: pkg.ErrBadInput},
				{name: "zero page", size: 50, err: pkg.ErrBadInput},
				{name: "zero size", page: 1, err: pkg.ErrBadInput},
				{name: "negative page", page: -1, size: 50, err: pkg.ErrBadInput},
				{name: "negative size", page: 1, size: -1, err: pkg.ErrBadInput},
				{name: "oversized", page: 1, size: 101, err: pkg.ErrBadInput},
			} {
				t.Run(tc.name, func(t *testing.T) {
					reads := &communicationReads{}
					reader := newCommunicationReader(reads, communicationPermissions{view: true, manage: method == "moderator clarifications"})
					if tc.err != nil {
						// Invalid pagination must fail before permission or storage access.
						reader = newCommunicationReader(nil, nil)
					}
					var err error
					if method == "announcements" {
						_, err = reader.ListAnnouncements(ctx, uuid.New(), tc.page, tc.size)
					} else {
						_, err = reader.ListClarifications(ctx, uuid.New(), &contestsmodel.ContestClarificationsFilter{Page: tc.page, PageSize: tc.size})
					}
					require.ErrorIs(t, err, tc.err)
					require.Equal(t, tc.err == nil, reads.called)
					if tc.err == nil {
						require.Equal(t, tc.page, reads.page)
						require.Equal(t, tc.size, reads.size)
					}
				})
			}
		})
	}
	_, err := newCommunicationReader(nil, nil).ListClarifications(context.Background(), uuid.New(), &contestsmodel.ContestClarificationsFilter{})
	require.ErrorIs(t, err, pkg.ErrUnauthenticated)
}
